package features

import (
	"strings"

	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

// CodeActions returns code actions for the given range and diagnostics.
// Currently supports "did you mean X?" fixes for undefined field/method diagnostics.
func CodeActions(pt *tmpl.ParsedTemplate, params lsp.CodeActionParams, bindings []goanalysis.TemplateBinding) []lsp.CodeAction {
	var actions []lsp.CodeAction

	for _, diag := range params.Context.Diagnostics {
		if diag.Source != "gohtml-lsp" {
			continue
		}
		if !strings.HasPrefix(diag.Message, "undefined field or method: ") {
			continue
		}

		// Parse "undefined field or method: X on Y" or "... (in chain ...)"
		msg := diag.Message
		msg = strings.TrimPrefix(msg, "undefined field or method: ")
		parts := strings.SplitN(msg, " on ", 2)
		if len(parts) != 2 {
			continue
		}
		badName := parts[0]
		typePart := parts[1]
		// Strip "(in chain ...)" suffix
		if idx := strings.Index(typePart, " (in chain"); idx >= 0 {
			typePart = typePart[:idx]
		}

		// Find the type and its fields/methods
		var candidates []string
		for _, b := range bindings {
			if b.DataType == nil {
				continue
			}
			ti := findTypeByName(b.DataType, typePart)
			if ti == nil {
				continue
			}
			for name := range ti.Fields {
				candidates = append(candidates, name)
			}
			for name := range ti.Methods {
				candidates = append(candidates, name)
			}
			break
		}

		// Find closest match
		bestName := ""
		bestDist := 3 // max distance threshold
		for _, name := range candidates {
			d := levenshtein(strings.ToLower(badName), strings.ToLower(name))
			if d < bestDist {
				bestDist = d
				bestName = name
			}
		}

		if bestName != "" {
			actions = append(actions, lsp.CodeAction{
				Title:       "Did you mean '" + bestName + "'?",
				Kind:        "quickfix",
				Diagnostics: []lsp.Diagnostic{diag},
				Edit: &lsp.WorkspaceEdit{
					Changes: map[string][]lsp.TextEdit{
						params.TextDocument.URI: {
							findFieldEdit(pt, diag.Range, badName, bestName),
						},
					},
				},
			})
		}
	}

	return actions
}

// findFieldEdit creates a TextEdit to replace the bad field name in the template.
func findFieldEdit(pt *tmpl.ParsedTemplate, diagRange lsp.Range, oldName, newName string) lsp.TextEdit {
	// The diagnostic range covers the whole node (e.g., .Usrname or .User.Usrname).
	// We need to find just the bad field name within it.
	startOff := pt.PositionToOffset(diagRange.Start)
	endOff := pt.PositionToOffset(diagRange.End)
	if endOff > len(pt.Content) {
		endOff = len(pt.Content)
	}
	nodeText := pt.Content[startOff:endOff]

	// Find the bad field name in the node text
	idx := strings.LastIndex(nodeText, oldName)
	if idx < 0 {
		// Fallback: replace the whole range
		return lsp.TextEdit{Range: diagRange, NewText: strings.Replace(nodeText, oldName, newName, 1)}
	}

	nameStart := startOff + idx
	nameEnd := nameStart + len(oldName)
	return lsp.TextEdit{
		Range: lsp.Range{
			Start: pt.OffsetToPosition(nameStart),
			End:   pt.OffsetToPosition(nameEnd),
		},
		NewText: newName,
	}
}

// findTypeByName searches a TypeInfo tree for a type with the given name.
func findTypeByName(ti *goanalysis.TypeInfo, name string) *goanalysis.TypeInfo {
	if ti == nil {
		return nil
	}
	if ti.Name == name {
		return ti
	}
	for _, fi := range ti.Fields {
		if found := findTypeByName(fi.ChildType, name); found != nil {
			return found
		}
	}
	return nil
}

// levenshtein computes the Levenshtein edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
