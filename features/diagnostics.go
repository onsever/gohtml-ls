package features

import (
	"path/filepath"
	"strings"
	"text/template/parse"

	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
	"github.com/onsever/gohtml-ls/workspace"
)

// ComputeDiagnostics returns diagnostics for a parsed template.
func ComputeDiagnostics(pt *tmpl.ParsedTemplate, index *tmpl.TemplateIndex, bindings []goanalysis.TemplateBinding) []lsp.Diagnostic {
	diags := make([]lsp.Diagnostic, 0, len(pt.Errors))
	diags = append(diags, pt.Errors...)

	// Check for undefined templates
	if pt.Trees != nil {
		for name := range index.References {
			for _, ref := range index.References[name] {
				if ref.URI != pt.URI {
					continue
				}
				if _, ok := index.Definitions[name]; !ok {
					diags = append(diags, lsp.Diagnostic{
						Range:    ref.Range,
						Severity: lsp.DiagnosticSeverityWarning,
						Source:   "gohtml-lsp",
						Message:  "undefined template: " + name,
					})
				}
			}
		}
	}

	// Validate field references against bound data type
	if pt.Trees != nil && len(bindings) > 0 {
		matched := matchBindingsForDiag(pt.URI, bindings)
		if len(matched) == 0 {
			// Fallback: use bindings not specifically bound to another template.
			baseName := filepath.Base(workspace.URIToPath(pt.URI))
			for _, b := range bindings {
				if b.DataType != nil && len(b.DataType.Fields) > 0 {
					if b.TemplateName == "" || b.TemplateName == baseName ||
						strings.TrimSuffix(baseName, filepath.Ext(baseName)) == b.TemplateName {
						matched = append(matched, b)
					}
				}
			}
		}

		for treeName, tree := range pt.Trees {
			if tree.Root == nil {
				continue
			}
			dataType := findBindingForTree(treeName, matched)
			// Fall back to cross-template data flow for child templates
			if dataType == nil && treeName != "" {
				dataType = resolveTemplateCallDotType(treeName, index, bindings)
			}
			if dataType != nil {
				walkFieldDiagnostics(tree.Root, pt, dataType, matched, &diags)
			}
		}
	}

	return diags
}

// findBindingForTree returns the DataType for a specific tree name.
// It first tries an exact match on TemplateName, then falls back only
// for the root tree (empty name) which represents a file without {{define}}.
func findBindingForTree(treeName string, bindings []goanalysis.TemplateBinding) *goanalysis.TypeInfo {
	for _, b := range bindings {
		if b.DataType != nil && len(b.DataType.Fields) > 0 && b.TemplateName == treeName {
			return b.DataType
		}
	}
	// Only fall back for the root tree (no {{define}}) — this is the
	// file-level template content, so any file-matched binding applies.
	// Named trees (from {{define "x"}}) without an exact binding match
	// are partials receiving dot via {{template "x" .}} — skip them
	// to avoid false positives.
	if treeName == "" {
		for _, b := range bindings {
			if b.DataType != nil && len(b.DataType.Fields) > 0 {
				return b.DataType
			}
		}
	}
	return nil
}

// walkFieldDiagnostics walks the parse tree looking for FieldNode references
// and validates that the first field in the chain exists on the bound type.
func walkFieldDiagnostics(node parse.Node, pt *tmpl.ParsedTemplate, dataType *goanalysis.TypeInfo, bindings []goanalysis.TemplateBinding, diags *[]lsp.Diagnostic) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				walkFieldDiagnostics(child, pt, dataType, bindings, diags)
			}
		}
	case *parse.ActionNode:
		if n.Pipe != nil {
			walkFieldDiagnostics(n.Pipe, pt, dataType, bindings, diags)
		}
	case *parse.PipeNode:
		for _, cmd := range n.Cmds {
			walkFieldDiagnostics(cmd, pt, dataType, bindings, diags)
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			walkFieldDiagnostics(arg, pt, dataType, bindings, diags)
		}
	case *parse.VariableNode:
		// Validate $v.Field chains against resolved variable type
		if len(n.Ident) > 1 {
			varType := resolveVariableType(n.Ident[0], pt, int(n.Position()), bindings)
			if varType != nil {
				current := varType
				for i := 1; i < len(n.Ident); i++ {
					if current == nil {
						break
					}
					fname := n.Ident[i]
					if fi, ok := current.Fields[fname]; ok {
						current = fi.ChildType
						continue
					}
					if _, ok := current.Methods[fname]; ok {
						break
					}
					pos := pt.OffsetToPosition(int(n.Position()))
					endPos := pos
					endPos.Character += uint32(len(n.String()))
					*diags = append(*diags, lsp.Diagnostic{
						Range:    lsp.Range{Start: pos, End: endPos},
						Severity: lsp.DiagnosticSeverityWarning,
						Source:   "gohtml-lsp",
						Message:  "undefined field or method: " + fname + " on " + current.Name,
					})
					break
				}
			}
		}
	case *parse.FieldNode:
		// Validate the full field chain: .Field1.Field2.Field3
		if len(n.Ident) > 0 {
			current := dataType
			for i, fname := range n.Ident {
				if current == nil {
					break
				}
				if fi, ok := current.Fields[fname]; ok {
					current = fi.ChildType
					continue
				}
				if mi, ok := current.Methods[fname]; ok {
					current = mi.ReturnType
					continue
				}
				// Undefined field/method
				pos := pt.OffsetToPosition(int(n.Position()))
				endPos := pos
				endPos.Character += uint32(len(n.String()))
				msg := "undefined field or method: " + fname + " on " + current.Name
				if i > 0 {
					msg = "undefined field or method: " + fname + " on " + current.Name + " (in chain ." + strings.Join(n.Ident[:i+1], ".") + ")"
				}
				*diags = append(*diags, lsp.Diagnostic{
					Range:    lsp.Range{Start: pos, End: endPos},
					Severity: lsp.DiagnosticSeverityWarning,
					Source:   "gohtml-lsp",
					Message:  msg,
				})
				break
			}
		}
	case *parse.IfNode:
		walkBranchDiagnostics(&n.BranchNode, pt, dataType, bindings, diags, false)
	case *parse.RangeNode:
		walkBranchDiagnostics(&n.BranchNode, pt, dataType, bindings, diags, true)
	case *parse.WithNode:
		walkBranchDiagnostics(&n.BranchNode, pt, dataType, bindings, diags, true)
	case *parse.TemplateNode:
		if n.Pipe != nil {
			walkFieldDiagnostics(n.Pipe, pt, dataType, bindings, diags)
		}
	}
}

func walkBranchDiagnostics(n *parse.BranchNode, pt *tmpl.ParsedTemplate, dataType *goanalysis.TypeInfo, bindings []goanalysis.TemplateBinding, diags *[]lsp.Diagnostic, rescope bool) {
	// Validate the pipe expression against the current (parent) data type
	if n.Pipe != nil {
		walkFieldDiagnostics(n.Pipe, pt, dataType, bindings, diags)
	}

	// Determine the dot type for the body
	bodyType := dataType
	if rescope && n.Pipe != nil {
		if resolved := resolvePipeDotType(n.Pipe, dataType); resolved != nil {
			bodyType = resolved
		}
	}

	if n.List != nil {
		walkFieldDiagnostics(n.List, pt, bodyType, bindings, diags)
	}
	if n.ElseList != nil {
		walkFieldDiagnostics(n.ElseList, pt, dataType, bindings, diags)
	}
}

// resolvePipeDotType resolves the dot type that a range/with pipe expression produces.
// For range: if pipe is `.Items` and Items is `[]Item`, dot becomes Item's type.
// For with: if pipe is `.User`, dot becomes User's type.
func resolvePipeDotType(pipe *parse.PipeNode, dataType *goanalysis.TypeInfo) *goanalysis.TypeInfo {
	if len(pipe.Cmds) == 0 {
		return nil
	}
	cmd := pipe.Cmds[0]
	if len(cmd.Args) == 0 {
		return nil
	}
	fieldNode, ok := cmd.Args[0].(*parse.FieldNode)
	if !ok || len(fieldNode.Ident) == 0 {
		return nil
	}

	// Walk the field chain to find the target field
	current := dataType
	var lastField *goanalysis.FieldInfo
	for _, fname := range fieldNode.Ident {
		if current == nil {
			return nil
		}
		fi, ok := current.Fields[fname]
		if !ok {
			return nil
		}
		lastField = &fi
		current = fi.ChildType
	}

	if lastField == nil {
		return nil
	}

	// For range over a slice, the dot becomes the element type
	typeName := lastField.TypeName
	if strings.HasPrefix(typeName, "[]") {
		elemName := goanalysis.ElementTypeName(typeName)
		if lastField.ChildType != nil && lastField.ChildType.Name == elemName {
			return lastField.ChildType
		}
		// The ChildType might be the slice element itself if resolved
		// Return a basic type info for the element
		return goanalysis.NewTypeInfo(elemName, "")
	}

	// For with, dot becomes the field's type directly
	if lastField.ChildType != nil {
		return lastField.ChildType
	}
	return nil
}

// matchBindingsForDiag finds bindings matching a template URI.
func matchBindingsForDiag(uri string, bindings []goanalysis.TemplateBinding) []goanalysis.TemplateBinding {
	var matched []goanalysis.TemplateBinding
	path := workspace.URIToPath(uri)
	baseName := filepath.Base(path)

	for _, b := range bindings {
		if b.DataType == nil {
			continue
		}
		if b.TemplateName == baseName {
			matched = append(matched, b)
			continue
		}
		parsedMatch := false
		for _, pf := range b.ParsedFiles {
			pfBase := filepath.Base(pf)
			if pfBase == baseName || pf == baseName {
				matched = append(matched, b)
				parsedMatch = true
				break
			}
		}
		if parsedMatch {
			continue
		}
		// Also match if the template name (without extension) matches
		if strings.TrimSuffix(baseName, filepath.Ext(baseName)) == b.TemplateName {
			matched = append(matched, b)
		}
	}
	return matched
}
