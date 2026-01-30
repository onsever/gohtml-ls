package features

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"text/template/parse"

	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
	"github.com/onsever/gohtml-ls/workspace"
)

// Hover computes hover information at the given position.
func Hover(pt *tmpl.ParsedTemplate, pos lsp.Position, index *tmpl.TemplateIndex, bindings []goanalysis.TemplateBinding) *lsp.Hover {
	offset := pt.PositionToOffset(pos)

	// Pre-check: cursor on {{define/block}} keyword or name (not represented in AST)
	if hover := hoverDefineOrBlock(pt, offset, index); hover != nil {
		return hover
	}

	node, treeName := pt.NodeAtOffset(offset)
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *parse.FieldNode:
		fields := n.Ident
		content := "**Field:** `." + strings.Join(fields, ".") + "`"
		// Try to resolve the dot type from direct bindings or cross-template data flow
		var resolvedType *goanalysis.TypeInfo
		baseName := filepath.Base(workspace.URIToPath(pt.URI))
		for _, b := range bindings {
			if b.DataType == nil {
				continue
			}
			if b.TemplateName != "" && b.TemplateName != treeName &&
				b.TemplateName != baseName &&
				strings.TrimSuffix(baseName, filepath.Ext(baseName)) != b.TemplateName {
				continue
			}
			resolvedType = resolveDotTypeAtOffset(pt, offset, b.DataType)
			break
		}
		// Fall back to cross-template data flow
		if resolvedType == nil && treeName != "" {
			resolvedType = resolveTemplateCallDotType(treeName, index, bindings)
		}
		if resolvedType != nil && len(fields) > 0 {
			current := resolvedType
			for i, fname := range fields {
				if current == nil {
					break
				}
				if fi, ok := current.Fields[fname]; ok {
					if i == len(fields)-1 {
						content += fmt.Sprintf("\n\nType: `%s`", fi.TypeName)
						if fi.Doc != "" {
							content += "\n\n" + fi.Doc
						}
					}
					current = fi.ChildType
				} else if mi, ok := current.Methods[fname]; ok {
					if i == len(fields)-1 {
						content += fmt.Sprintf("\n\nMethod: `%s`", mi.Signature)
						if mi.Doc != "" {
							content += "\n\n" + mi.Doc
						}
					}
					current = mi.ReturnType
				} else {
					break
				}
			}
		}
		return &lsp.Hover{
			Contents: lsp.MarkupContent{Kind: "markdown", Value: content},
		}

	case *parse.IdentifierNode:
		// Function name
		name := n.Ident
		builtins := goanalysis.BuiltinFuncs()
		if sig, ok := builtins[name]; ok {
			content := fmt.Sprintf("**Function:** `%s`\n\n%s", sig.Signature, sig.Doc)
			return &lsp.Hover{
				Contents: lsp.MarkupContent{Kind: "markdown", Value: content},
			}
		}
		// Check custom funcs from bindings
		for _, b := range bindings {
			if sig, ok := b.FuncMaps[name]; ok {
				content := fmt.Sprintf("**Custom Function:** `%s`", sig.Name)
				if sig.Signature != "" {
					content += fmt.Sprintf("\n\nSignature: `%s`", sig.Signature)
				}
				return &lsp.Hover{
					Contents: lsp.MarkupContent{Kind: "markdown", Value: content},
				}
			}
		}
		return &lsp.Hover{
			Contents: lsp.MarkupContent{Kind: "markdown", Value: fmt.Sprintf("**Function:** `%s`", name)},
		}

	case *parse.VariableNode:
		varName := n.Ident[0]
		content := fmt.Sprintf("**Variable:** `%s`", varName)

		// Try to resolve variable type from range declarations
		matchedBindings := matchBindings(pt.URI, bindings)
		if len(matchedBindings) == 0 {
			matchedBindings = bindings
		}
		if varType := resolveVariableType(varName, pt, offset, matchedBindings); varType != nil {
			// If the variable has field chain (e.g., $v.Name), walk it
			if len(n.Ident) > 1 {
				current := varType
				for i := 1; i < len(n.Ident); i++ {
					if current == nil {
						break
					}
					fname := n.Ident[i]
					if fi, ok := current.Fields[fname]; ok {
						if i == len(n.Ident)-1 {
							content += fmt.Sprintf("\n\nType: `%s`", fi.TypeName)
							if fi.Doc != "" {
								content += "\n\n" + fi.Doc
							}
						}
						current = fi.ChildType
					} else if mi, ok := current.Methods[fname]; ok {
						if i == len(n.Ident)-1 {
							content += fmt.Sprintf("\n\nMethod: `%s`", mi.Signature)
							if mi.Doc != "" {
								content += "\n\n" + mi.Doc
							}
						}
						break
					} else {
						break
					}
				}
			} else {
				content += fmt.Sprintf("\n\nType: `%s`", varType.Name)
			}
		}

		return &lsp.Hover{
			Contents: lsp.MarkupContent{Kind: "markdown", Value: content},
		}

	case *parse.TemplateNode:
		// Check if cursor is on the "template" keyword itself
		if isOnKeywordAtNode(pt, offset, int(n.Position()), "template") {
			return &lsp.Hover{
				Contents: lsp.MarkupContent{Kind: "markdown", Value: keywordDoc("template")},
			}
		}
		name := n.Name
		content := fmt.Sprintf("**Template:** `%s`", name)
		if defs, ok := index.Definitions[name]; ok && len(defs) > 0 {
			content += fmt.Sprintf("\n\nDefined in: %s", defs[0].URI)
		}
		return &lsp.Hover{
			Contents: lsp.MarkupContent{Kind: "markdown", Value: content},
		}

	case *parse.DotNode:
		content := "**Dot (.):** Current context value"
		// Resolve the dot type from bindings
		baseName := filepath.Base(workspace.URIToPath(pt.URI))
		for _, b := range bindings {
			if b.DataType == nil {
				continue
			}
			if b.TemplateName != "" && b.TemplateName != treeName &&
				b.TemplateName != baseName &&
				strings.TrimSuffix(baseName, filepath.Ext(baseName)) != b.TemplateName {
				continue
			}
			dotType := resolveDotTypeAtOffset(pt, offset, b.DataType)
			if dotType != nil {
				content += fmt.Sprintf("\n\nType: `%s`", dotType.Name)
				if b.HandlerName != "" {
					goFile := filepath.Base(b.GoFile)
					content += fmt.Sprintf("\n\nBound in `%s` (%s)", b.HandlerName, goFile)
				}
			}
			break
		}
		// Fall back to cross-template data flow
		if treeName != "" && !strings.Contains(content, "Type:") {
			if dotType := resolveTemplateCallDotType(treeName, index, bindings); dotType != nil {
				content += fmt.Sprintf("\n\nType: `%s`", dotType.Name)
			}
		}
		return &lsp.Hover{
			Contents: lsp.MarkupContent{Kind: "markdown", Value: content},
		}

	case *parse.IfNode:
		return &lsp.Hover{
			Contents: lsp.MarkupContent{Kind: "markdown", Value: keywordDoc("if")},
		}

	case *parse.RangeNode:
		return &lsp.Hover{
			Contents: lsp.MarkupContent{Kind: "markdown", Value: keywordDoc("range")},
		}

	case *parse.WithNode:
		return &lsp.Hover{
			Contents: lsp.MarkupContent{Kind: "markdown", Value: keywordDoc("with")},
		}
	}

	return nil
}

// isOnKeywordAtNode checks if offset is on a keyword near nodePos.
func isOnKeywordAtNode(pt *tmpl.ParsedTemplate, offset, nodePos int, keyword string) bool {
	start := nodePos - len(keyword) - 10
	if start < 0 {
		start = 0
	}
	end := nodePos + len(keyword) + 10
	if end > len(pt.Content) {
		end = len(pt.Content)
	}
	sub := pt.Content[start:end]
	idx := strings.Index(sub, keyword)
	if idx < 0 {
		return false
	}
	kwStart := start + idx
	kwEnd := kwStart + len(keyword)
	return kwStart <= offset && offset < kwEnd
}

// keywordDoc returns rich documentation for Go template keywords.
func keywordDoc(keyword string) string {
	docs := map[string]string{
		"if": "## `{{if}}` — Conditional\n\n" +
			"Conditionally renders content. The pipeline is evaluated and if the value is **empty** " +
			"(false, 0, nil, empty string, empty slice/map), the block is skipped.\n\n" +
			"### Syntax\n" +
			"```\n{{if <pipeline>}} T1 {{end}}\n{{if <pipeline>}} T1 {{else}} T0 {{end}}\n{{if <pipeline>}} T1 {{else if <pipeline>}} T2 {{else}} T0 {{end}}\n```\n\n" +
			"### Examples\n" +
			"```html\n" +
			"{{if .IsAdmin}}\n  <span>Admin</span>\n{{end}}\n\n" +
			"{{if eq .Status \"active\"}}\n  <span class=\"green\">Active</span>\n{{else}}\n  <span class=\"red\">Inactive</span>\n{{end}}\n\n" +
			"{{if gt (len .Items) 0}}\n  <ul>...</ul>\n{{else}}\n  <p>No items found.</p>\n{{end}}\n" +
			"```\n\n" +
			"**Empty values:** `false`, `0`, `nil`, `\"\"`, empty slice/array/map.",

		"range": "## `{{range}}` — Iteration\n\n" +
			"Iterates over slices, arrays, maps, or channels. Inside the block, the **dot (`.`)** is set " +
			"to the current element. If the collection is empty, the `{{else}}` block is rendered.\n\n" +
			"### Syntax\n" +
			"```\n{{range <pipeline>}} T1 {{end}}\n{{range <pipeline>}} T1 {{else}} T0 {{end}}\n{{range $index, $element := <pipeline>}} T1 {{end}}\n{{range $element := <pipeline>}} T1 {{end}}\n```\n\n" +
			"### Examples\n" +
			"```html\n" +
			"{{range .Items}}\n  <li>{{.Name}} — {{.Price}}</li>\n{{end}}\n\n" +
			"{{range $i, $item := .Items}}\n  <li>{{$i}}: {{$item.Name}}</li>\n{{end}}\n\n" +
			"{{range .Items}}\n  <li>{{.Name}}</li>\n{{else}}\n  <li>No items available.</li>\n{{end}}\n" +
			"```\n\n" +
			"**Note:** Inside `range`, `.` refers to the current element, not the top-level data. " +
			"Use `$` to access the root context: `{{$.Title}}`.",

		"with": "## `{{with}}` — Context Scoping\n\n" +
			"Sets the dot (`.`) to the value of the pipeline within the block. " +
			"If the value is empty, the block is skipped (like `if`). Useful for reducing repetitive field chains.\n\n" +
			"### Syntax\n" +
			"```\n{{with <pipeline>}} T1 {{end}}\n{{with <pipeline>}} T1 {{else}} T0 {{end}}\n{{with $x := <pipeline>}} T1 {{end}}\n```\n\n" +
			"### Examples\n" +
			"```html\n" +
			"{{with .User}}\n  <p>{{.Name}} ({{.Email}})</p>\n{{end}}\n\n" +
			"{{with .Address}}\n  <p>{{.Street}}, {{.City}}</p>\n{{else}}\n  <p>No address on file.</p>\n{{end}}\n\n" +
			"{{with $addr := .User.Address}}\n  <p>{{$addr.City}}</p>\n{{end}}\n" +
			"```\n\n" +
			"**Note:** Inside `with`, `.` refers to the scoped value. Use `$` for root context.",

		"template": "## `{{template}}` — Template Invocation\n\n" +
			"Executes a named template (defined with `{{define}}`) and optionally passes data to it.\n\n" +
			"### Syntax\n" +
			"```\n{{template \"name\"}}\n{{template \"name\" <pipeline>}}\n```\n\n" +
			"### Examples\n" +
			"```html\n" +
			"{{template \"header\"}}\n\n" +
			"{{template \"user-card\" .User}}\n\n" +
			"{{template \"sidebar\" .}}\n" +
			"```\n\n" +
			"**Note:** The invoked template receives the pipeline value as its dot (`.`). " +
			"If no pipeline is provided, it receives `nil`.",

		"define": "## `{{define}}` — Template Definition\n\n" +
			"Defines a named template that can be invoked with `{{template}}`. " +
			"Everything between `{{define}}` and `{{end}}` becomes the template body.\n\n" +
			"### Syntax\n" +
			"```\n{{define \"name\"}} T1 {{end}}\n```\n\n" +
			"### Examples\n" +
			"```html\n" +
			"{{define \"header\"}}\n  <header><h1>{{.Title}}</h1></header>\n{{end}}\n\n" +
			"{{define \"user-card\"}}\n  <div class=\"card\">\n    <h2>{{.Name}}</h2>\n    <p>{{.Email}}</p>\n  </div>\n{{end}}\n" +
			"```",

		"block": "## `{{block}}` — Block (Define + Invoke)\n\n" +
			"Shorthand for defining and immediately invoking a template. Equivalent to " +
			"`{{define \"name\"}}...{{end}}{{template \"name\" pipeline}}`. Useful for base layouts with overridable sections.\n\n" +
			"### Syntax\n" +
			"```\n{{block \"name\" <pipeline>}} T1 {{end}}\n```\n\n" +
			"### Examples\n" +
			"```html\n" +
			"{{block \"content\" .}}\n  <p>Default content — override in child templates.</p>\n{{end}}\n\n" +
			"{{block \"sidebar\" .}}\n  <nav>Default sidebar</nav>\n{{end}}\n" +
			"```\n\n" +
			"**Note:** Child templates can override a block by defining a template with the same name.",

		"else": "## `{{else}}` — Else Branch\n\n" +
			"Provides an alternative block when the condition of `if`, `with`, or `range` evaluates to empty.\n\n" +
			"### Syntax\n" +
			"```\n{{if <pipeline>}} T1 {{else}} T0 {{end}}\n{{if <pipeline>}} T1 {{else if <pipeline>}} T2 {{end}}\n{{range <pipeline>}} T1 {{else}} T0 {{end}}\n{{with <pipeline>}} T1 {{else}} T0 {{end}}\n```\n\n" +
			"### Examples\n" +
			"```html\n" +
			"{{if .LoggedIn}}\n  Welcome, {{.Name}}!\n{{else}}\n  <a href=\"/login\">Sign in</a>\n{{end}}\n" +
			"```",

		"end": "## `{{end}}` — End Block\n\n" +
			"Closes an `if`, `range`, `with`, `define`, or `block` action.\n\n" +
			"### Syntax\n" +
			"```\n{{if .X}} ... {{end}}\n{{range .Items}} ... {{end}}\n{{define \"name\"}} ... {{end}}\n```",
	}
	if doc, ok := docs[keyword]; ok {
		return doc
	}
	return fmt.Sprintf("**Keyword:** `%s`", keyword)
}

// defineBlockRe matches {{define "name"}} and {{block "name" ...}} including trim variants.
var defineBlockRe = regexp.MustCompile(`\{\{-?\s*(define|block)\s+"([^"]*)"`)

// hoverDefineOrBlock checks if offset falls on a {{define}}/{{block}} keyword or template name.
func hoverDefineOrBlock(pt *tmpl.ParsedTemplate, offset int, index *tmpl.TemplateIndex) *lsp.Hover {
	for _, m := range defineBlockRe.FindAllStringIndex(pt.Content, -1) {
		// Find submatches for this occurrence
		sub := defineBlockRe.FindStringSubmatch(pt.Content[m[0]:m[1]])
		if sub == nil {
			continue
		}
		// Locate keyword and name within the match
		matchStr := pt.Content[m[0]:m[1]]
		kwIdx := strings.Index(matchStr, sub[1])
		kwStart := m[0] + kwIdx
		kwEnd := kwStart + len(sub[1])

		nameIdx := strings.Index(matchStr, `"`+sub[2]+`"`)
		nameStart := m[0] + nameIdx + 1 // skip opening quote
		nameEnd := nameStart + len(sub[2])

		if offset >= kwStart && offset < kwEnd {
			return &lsp.Hover{
				Contents: lsp.MarkupContent{Kind: "markdown", Value: keywordDoc(sub[1])},
			}
		}
		if offset >= nameStart && offset < nameEnd {
			name := sub[2]
			content := fmt.Sprintf("**Template:** `%s`", name)
			if defs, ok := index.Definitions[name]; ok && len(defs) > 0 {
				content += fmt.Sprintf("\n\nDefined in: %s", defs[0].URI)
			}
			return &lsp.Hover{
				Contents: lsp.MarkupContent{Kind: "markdown", Value: content},
			}
		}
	}
	return nil
}
