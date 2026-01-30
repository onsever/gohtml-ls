package features

import (
	"strings"
	"text/template/parse"

	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

// PrepareRename checks if the cursor is on a renameable template name.
// Returns the range of the name if renameable, nil otherwise.
func PrepareRename(pt *tmpl.ParsedTemplate, pos lsp.Position, index *tmpl.TemplateIndex) *lsp.Range {
	name := templateNameAtPosition(pt, pos, index)
	if name == "" {
		return nil
	}
	offset := pt.PositionToOffset(pos)
	// Find the name string at this offset
	nameStart := strings.LastIndex(pt.Content[:offset+len(name)+5], `"`+name+`"`)
	if nameStart < 0 {
		// Try finding the name near the offset
		start := offset - len(name) - 10
		if start < 0 {
			start = 0
		}
		end := offset + len(name) + 10
		if end > len(pt.Content) {
			end = len(pt.Content)
		}
		nameStart = strings.Index(pt.Content[start:end], `"`+name+`"`)
		if nameStart >= 0 {
			nameStart += start
		}
	}
	if nameStart < 0 {
		return nil
	}
	// nameStart points to the opening quote; the name starts after it
	nameOffset := nameStart + 1
	startPos := pt.OffsetToPosition(nameOffset)
	endPos := startPos
	endPos.Character += uint32(len(name))
	return &lsp.Range{Start: startPos, End: endPos}
}

// Rename renames a template name across all definitions and references.
func Rename(pt *tmpl.ParsedTemplate, pos lsp.Position, newName string, index *tmpl.TemplateIndex) *lsp.WorkspaceEdit {
	name := templateNameAtPosition(pt, pos, index)
	if name == "" {
		return nil
	}

	// Collect all unique URIs that contain this template name
	uris := make(map[string]bool)
	if defs, ok := index.Definitions[name]; ok {
		for _, def := range defs {
			uris[def.URI] = true
		}
	}
	if refs, ok := index.References[name]; ok {
		for _, ref := range refs {
			uris[ref.URI] = true
		}
	}

	changes := make(map[string][]lsp.TextEdit)
	for uri := range uris {
		addTemplateNameEdits(changes, uri, name, newName, index)
	}

	if len(changes) == 0 {
		return nil
	}
	return &lsp.WorkspaceEdit{Changes: changes}
}

// addTemplateNameEdits finds all occurrences of the template name in a file and adds edits.
func addTemplateNameEdits(changes map[string][]lsp.TextEdit, uri, oldName, newName string, index *tmpl.TemplateIndex) {
	pt, ok := index.Trees[uri]
	if !ok {
		return
	}

	content := pt.Content
	// Find all occurrences of "oldName" in template/define/block directives
	patterns := []string{
		`define "` + oldName + `"`,
		`template "` + oldName + `"`,
		`block "` + oldName + `"`,
	}

	for _, pat := range patterns {
		search := pat
		offset := 0
		for {
			idx := strings.Index(content[offset:], search)
			if idx < 0 {
				break
			}
			absIdx := offset + idx
			// The name starts after the prefix (e.g., `define "`) and before the closing quote
			prefixLen := strings.Index(search, `"`) + 1
			nameOffset := absIdx + prefixLen
			startPos := pt.OffsetToPosition(nameOffset)
			endPos := startPos
			endPos.Character += uint32(len(oldName))
			changes[uri] = append(changes[uri], lsp.TextEdit{
				Range:   lsp.Range{Start: startPos, End: endPos},
				NewText: newName,
			})
			offset = absIdx + len(search)
		}
	}
}

// templateNameAtPosition returns the template name at the cursor position,
// or empty string if cursor is not on a template name.
func templateNameAtPosition(pt *tmpl.ParsedTemplate, pos lsp.Position, index *tmpl.TemplateIndex) string {
	offset := pt.PositionToOffset(pos)
	node, _ := pt.NodeAtOffset(offset)

	if n, ok := node.(*parse.TemplateNode); ok {
		return n.Name
	}

	// Check if cursor is on a define/block name
	for name, defs := range index.Definitions {
		for _, def := range defs {
			if def.URI != pt.URI {
				continue
			}
			defOffset := pt.PositionToOffset(def.Range.Start)
			endOffset := pt.PositionToOffset(def.Range.End)
			if offset >= defOffset && offset <= endOffset+20 {
				return name
			}
		}
	}

	return ""
}
