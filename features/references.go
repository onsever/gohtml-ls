package features

import (
	"text/template/parse"

	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

// References returns all reference locations for the item at the given position.
func References(pt *tmpl.ParsedTemplate, pos lsp.Position, index *tmpl.TemplateIndex, includeDecl bool) []lsp.Location {
	offset := pt.PositionToOffset(pos)
	node, _ := pt.NodeAtOffset(offset)
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *parse.TemplateNode:
		return templateReferences(n.Name, index, includeDecl)
	}

	// Check if cursor is on a define name (we need to check content)
	// Look through definitions to see if cursor is on one
	for name, defs := range index.Definitions {
		for _, def := range defs {
			if def.URI == pt.URI {
				defOffset := pt.PositionToOffset(def.Range.Start)
				endOffset := pt.PositionToOffset(def.Range.End)
				if offset >= defOffset && offset <= endOffset+20 {
					return templateReferences(name, index, includeDecl)
				}
			}
		}
	}

	return nil
}

func templateReferences(name string, index *tmpl.TemplateIndex, includeDecl bool) []lsp.Location {
	var locs []lsp.Location
	if includeDecl {
		if defs, ok := index.Definitions[name]; ok {
			locs = append(locs, defs...)
		}
	}
	if refs, ok := index.References[name]; ok {
		locs = append(locs, refs...)
	}
	return locs
}
