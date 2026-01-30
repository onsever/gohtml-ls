package features

import (
	"strings"

	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

// Format normalizes whitespace inside Go template actions.
// It does not modify HTML content outside of {{ }}.
// Preserves trim markers ({{- and -}}).
func Format(pt *tmpl.ParsedTemplate) []lsp.TextEdit {
	content := pt.Content
	var edits []lsp.TextEdit

	// Find all template actions and normalize their internal whitespace
	matches := findActions(content)
	for _, m := range matches {
		original := content[m.start:m.end]
		formatted := formatAction(original)
		if formatted != original {
			startPos := pt.OffsetToPosition(m.start)
			endPos := pt.OffsetToPosition(m.end)
			edits = append(edits, lsp.TextEdit{
				Range:   lsp.Range{Start: startPos, End: endPos},
				NewText: formatted,
			})
		}
	}

	return edits
}

type actionMatch struct {
	start, end int
}

// findActions finds all {{ ... }} actions in the content, handling nesting correctly.
func findActions(content string) []actionMatch {
	var matches []actionMatch
	i := 0
	for i < len(content)-1 {
		if content[i] == '{' && content[i+1] == '{' {
			start := i
			// Find matching }}
			j := i + 2
			depth := 1
			for j < len(content)-1 {
				if content[j] == '{' && content[j+1] == '{' {
					depth++
					j += 2
					continue
				}
				if content[j] == '}' && content[j+1] == '}' {
					depth--
					if depth == 0 {
						matches = append(matches, actionMatch{start: start, end: j + 2})
						i = j + 2
						break
					}
					j += 2
					continue
				}
				j++
			}
			if depth > 0 {
				// Unmatched opening
				i += 2
			}
		} else {
			i++
		}
	}
	return matches
}

// formatAction normalizes whitespace in a single template action.
// {{  if   .X  }} -> {{ if .X }}
// {{-  range  .Items  -}} -> {{- range .Items -}}
func formatAction(action string) string {
	if len(action) < 4 {
		return action
	}

	// Extract prefix ({{ or {{-)
	prefix := "{{"
	inner := action[2 : len(action)-2]
	suffix := "}}"

	// Check for trim markers
	if strings.HasPrefix(inner, "-") {
		prefix = "{{-"
		inner = inner[1:]
	}
	if strings.HasSuffix(inner, "-") {
		suffix = "-}}"
		inner = inner[:len(inner)-1]
	}

	// Normalize internal whitespace: collapse multiple spaces to single
	inner = strings.TrimSpace(inner)
	// Handle comments: /* ... */ — preserve as-is
	if strings.HasPrefix(inner, "/*") {
		return prefix + " " + inner + " " + suffix
	}

	// Split into tokens and rejoin with single spaces
	fields := strings.Fields(inner)
	if len(fields) == 0 {
		return prefix + " " + suffix
	}

	normalized := strings.Join(fields, " ")
	return prefix + " " + normalized + " " + suffix
}
