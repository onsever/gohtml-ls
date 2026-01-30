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

// Definition returns the definition location for the item at the given position.
func Definition(pt *tmpl.ParsedTemplate, pos lsp.Position, index *tmpl.TemplateIndex, bindings []goanalysis.TemplateBinding) *lsp.Location {
	offset := pt.PositionToOffset(pos)
	node, _ := pt.NodeAtOffset(offset)
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *parse.TemplateNode:
		// Jump to {{define "name"}}
		if defs, ok := index.Definitions[n.Name]; ok && len(defs) > 0 {
			return &defs[0]
		}

	case *parse.IdentifierNode:
		// Jump to custom FuncMap function definition
		funcName := n.Ident
		for _, b := range bindings {
			if sig, ok := b.FuncMaps[funcName]; ok && sig.GoFile != "" {
				uri := workspace.PathToURI(sig.GoFile)
				return &lsp.Location{
					URI: uri,
					Range: lsp.Range{
						Start: lsp.Position{Line: uint32(sig.Line), Character: uint32(sig.Col)},
						End:   lsp.Position{Line: uint32(sig.Line), Character: uint32(sig.Col + len(funcName))},
					},
				}
			}
		}

	case *parse.VariableNode:
		// Jump to Go struct field/method on variable like $v.Name
		if len(n.Ident) <= 1 {
			return nil // just $v with no field chain
		}
		matchedBindings := matchBindings(pt.URI, bindings)
		if len(matchedBindings) == 0 {
			matchedBindings = bindings
		}
		varType := resolveVariableType(n.Ident[0], pt, offset, matchedBindings)
		if varType == nil {
			return nil
		}
		current := varType
		for i := 1; i < len(n.Ident); i++ {
			if current == nil {
				break
			}
			fname := n.Ident[i]
			if fi, ok := current.Fields[fname]; ok {
				if i == len(n.Ident)-1 && fi.GoFile != "" {
					uri := workspace.PathToURI(fi.GoFile)
					return &lsp.Location{
						URI: uri,
						Range: lsp.Range{
							Start: lsp.Position{Line: uint32(fi.Line), Character: uint32(fi.Col)},
							End:   lsp.Position{Line: uint32(fi.Line), Character: uint32(fi.Col + len(fi.Name))},
						},
					}
				}
				current = fi.ChildType
			} else if mi, ok := current.Methods[fname]; ok {
				if mi.GoFile != "" {
					uri := workspace.PathToURI(mi.GoFile)
					return &lsp.Location{
						URI: uri,
						Range: lsp.Range{
							Start: lsp.Position{Line: uint32(mi.Line), Character: uint32(mi.Col)},
							End:   lsp.Position{Line: uint32(mi.Line), Character: uint32(mi.Col + len(mi.Name))},
						},
					}
				}
				break
			} else {
				break
			}
		}

	case *parse.FieldNode:
		// Jump to Go struct field or method declaration
		if len(n.Ident) == 0 {
			return nil
		}
		matchedBindings := matchBindings(pt.URI, bindings)
		if len(matchedBindings) == 0 {
			// Fallback: only use bindings not specifically bound to another template.
			baseName := filepath.Base(workspace.URIToPath(pt.URI))
			for _, b := range bindings {
				if b.TemplateName == "" || b.TemplateName == baseName ||
					strings.TrimSuffix(baseName, filepath.Ext(baseName)) == b.TemplateName {
					matchedBindings = append(matchedBindings, b)
				}
			}
		}
		for _, b := range matchedBindings {
			if b.DataType == nil {
				continue
			}
			// Resolve dot type considering range/with scopes
			scopedType := resolveDotTypeAtOffset(pt, offset, b.DataType)
			if scopedType == nil {
				continue
			}
			// Walk the field chain to find the target
			current := scopedType
			for i, fname := range n.Ident {
				if current == nil {
					break
				}
				if fi, ok := current.Fields[fname]; ok {
					if i == len(n.Ident)-1 && fi.GoFile != "" {
						uri := workspace.PathToURI(fi.GoFile)
						return &lsp.Location{
							URI: uri,
							Range: lsp.Range{
								Start: lsp.Position{Line: uint32(fi.Line), Character: uint32(fi.Col)},
								End:   lsp.Position{Line: uint32(fi.Line), Character: uint32(fi.Col + len(fi.Name))},
							},
						}
					}
					current = fi.ChildType
				} else if mi, ok := current.Methods[fname]; ok {
					if i == len(n.Ident)-1 && mi.GoFile != "" {
						uri := workspace.PathToURI(mi.GoFile)
						return &lsp.Location{
							URI: uri,
							Range: lsp.Range{
								Start: lsp.Position{Line: uint32(mi.Line), Character: uint32(mi.Col)},
								End:   lsp.Position{Line: uint32(mi.Line), Character: uint32(mi.Col + len(mi.Name))},
							},
						}
					}
					current = mi.ReturnType
				} else {
					break
				}
			}
		}
	}

	return nil
}
