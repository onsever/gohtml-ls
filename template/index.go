package template

import (
	"strings"
	"text/template/parse"

	"github.com/onsever/gohtml-ls/lsp"
)

// TemplateIndex maps template names to definition and reference locations.
type TemplateIndex struct {
	Definitions  map[string][]lsp.Location  // "name" -> define locations
	References   map[string][]lsp.Location  // "name" -> template call locations
	Trees        map[string]*ParsedTemplate // URI -> parsed template
	TemplateArgs map[string][]string        // "name" -> field chain passed via {{template "name" .Field}}
}

// NewTemplateIndex creates a new empty TemplateIndex.
func NewTemplateIndex() *TemplateIndex {
	return &TemplateIndex{
		Definitions:  make(map[string][]lsp.Location),
		References:   make(map[string][]lsp.Location),
		Trees:        make(map[string]*ParsedTemplate),
		TemplateArgs: make(map[string][]string),
	}
}

// Index indexes a parsed template, recording definitions and references.
func (idx *TemplateIndex) Index(pt *ParsedTemplate) {
	idx.removeURI(pt.URI)
	idx.Trees[pt.URI] = pt

	if pt.Trees == nil {
		return
	}

	for name, tree := range pt.Trees {
		if tree.Root == nil {
			continue
		}
		// Record named templates (defines)
		if name != "" && name != pt.URI {
			defOffset := findDefineOffset(pt.Content, name)
			pos := pt.OffsetToPosition(max(defOffset, 0))
			endPos := pos
			endPos.Character += uint32(len(name))
			idx.Definitions[name] = append(idx.Definitions[name], lsp.Location{
				URI:   pt.URI,
				Range: lsp.Range{Start: pos, End: endPos},
			})
		}

		// Walk tree for template references
		walkForReferences(tree.Root, pt, idx)
	}
}

func findDefineOffset(content, name string) int {
	patterns := []string{
		`{{define "` + name + `"`,
		`{{- define "` + name + `"`,
		`{{block "` + name + `"`,
		`{{- block "` + name + `"`,
	}
	for _, p := range patterns {
		if off := strings.Index(content, p); off >= 0 {
			return off
		}
	}
	return -1
}

func walkForReferences(node parse.Node, pt *ParsedTemplate, idx *TemplateIndex) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parse.TemplateNode:
		name := n.Name
		pos := pt.OffsetToPosition(int(n.Position()))
		endPos := pos
		endPos.Character += uint32(len(n.String()))
		idx.References[name] = append(idx.References[name], lsp.Location{
			URI:   pt.URI,
			Range: lsp.Range{Start: pos, End: endPos},
		})
		// Track the field chain passed as pipe argument (e.g., {{template "child" .User}})
		if n.Pipe != nil && len(n.Pipe.Cmds) > 0 {
			cmd := n.Pipe.Cmds[0]
			if len(cmd.Args) > 0 {
				if fn, ok := cmd.Args[0].(*parse.FieldNode); ok {
					idx.TemplateArgs[name] = fn.Ident
				}
			}
		}
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				walkForReferences(child, pt, idx)
			}
		}
	case *parse.IfNode:
		walkBranchRefs(&n.BranchNode, pt, idx)
	case *parse.RangeNode:
		walkBranchRefs(&n.BranchNode, pt, idx)
	case *parse.WithNode:
		walkBranchRefs(&n.BranchNode, pt, idx)
	}
}

func walkBranchRefs(n *parse.BranchNode, pt *ParsedTemplate, idx *TemplateIndex) {
	if n.List != nil {
		walkForReferences(n.List, pt, idx)
	}
	if n.ElseList != nil {
		walkForReferences(n.ElseList, pt, idx)
	}
}

func (idx *TemplateIndex) removeURI(uri string) {
	delete(idx.Trees, uri)
	removeURIFromMap(idx.Definitions, uri)
	removeURIFromMap(idx.References, uri)
	// TemplateArgs are overwritten on re-index, no URI-specific cleanup needed
}

func removeURIFromMap(m map[string][]lsp.Location, uri string) {
	for name, locs := range m {
		filtered := locs[:0]
		for _, loc := range locs {
			if loc.URI != uri {
				filtered = append(filtered, loc)
			}
		}
		if len(filtered) == 0 {
			delete(m, name)
		} else {
			m[name] = filtered
		}
	}
}
