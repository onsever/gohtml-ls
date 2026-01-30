package template

import (
	"strings"
	"text/template/parse"

	"github.com/onsever/gohtml-ls/lsp"
)

// ParsedTemplate holds a parsed template and its source
type ParsedTemplate struct {
	URI     string
	Content string
	Trees   map[string]*parse.Tree // template name -> parse tree
	Errors  []lsp.Diagnostic
	Lines   []int // byte offset of each line start
}

// Parse parses template content and returns a ParsedTemplate.
// Uses text/template/parse directly.
// extraFuncs are additional function names (e.g., from FuncMap) to register
// so the parser doesn't reject them as undefined.
func Parse(uri, content string, extraFuncs ...string) *ParsedTemplate {
	pt := &ParsedTemplate{
		URI:     uri,
		Content: content,
		Lines:   computeLineOffsets(content),
	}
	// Use text/template/parse.Parse with built-in function names
	builtins := []string{
		"and", "call", "html", "index", "slice", "js", "len",
		"not", "or", "print", "printf", "println", "urlquery",
		"eq", "ne", "lt", "le", "gt", "ge",
	}
	funcMap := make(map[string]interface{})
	for _, name := range builtins {
		funcMap[name] = func() {} // dummy
	}
	for _, name := range extraFuncs {
		funcMap[name] = func() {}
	}
	trees, err := parse.Parse("", content, "{{", "}}", funcMap)
	if err != nil {
		pt.Errors = append(pt.Errors, parseDiagnostic(err))
		return pt
	}
	pt.Trees = trees
	return pt
}

func parseDiagnostic(err error) lsp.Diagnostic {
	msg := err.Error()
	return lsp.Diagnostic{
		Range:    lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 0, Character: 0}},
		Severity: 1, // Error
		Source:   "gohtml-lsp",
		Message:  msg,
	}
}

// OffsetToPosition converts a byte offset to an LSP Position.
func (pt *ParsedTemplate) OffsetToPosition(offset int) lsp.Position {
	if len(pt.Lines) == 0 {
		return lsp.Position{}
	}
	line := 0
	for i, lo := range pt.Lines {
		if lo > offset {
			break
		}
		line = i
	}
	col := offset - pt.Lines[line]
	return lsp.Position{Line: uint32(line), Character: uint32(col)}
}

// PositionToOffset converts an LSP Position to a byte offset.
func (pt *ParsedTemplate) PositionToOffset(pos lsp.Position) int {
	if int(pos.Line) >= len(pt.Lines) {
		return len(pt.Content)
	}
	return pt.Lines[pos.Line] + int(pos.Character)
}

func computeLineOffsets(content string) []int {
	offsets := []int{0}
	for i, ch := range content {
		if ch == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// NodeAtOffset finds the innermost node at the given byte offset.
// Returns the node and its tree name.
func (pt *ParsedTemplate) NodeAtOffset(offset int) (parse.Node, string) {
	if pt.Trees == nil {
		return nil, ""
	}
	for name, tree := range pt.Trees {
		if tree.Root == nil {
			continue
		}
		if node := findNodeAtOffset(tree.Root, offset, pt.Content); node != nil {
			return node, name
		}
	}
	return nil, ""
}

func findNodeAtOffset(node parse.Node, offset int, content string) parse.Node {
	pos := int(node.Position())
	// Check children first for more specific match
	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				if found := findNodeAtOffset(child, offset, content); found != nil {
					return found
				}
			}
		}
	case *parse.ActionNode:
		if n.Pipe != nil {
			if found := findNodeAtOffset(n.Pipe, offset, content); found != nil {
				return found
			}
		}
	case *parse.PipeNode:
		for _, cmd := range n.Cmds {
			if found := findNodeAtOffset(cmd, offset, content); found != nil {
				return found
			}
		}
		for _, v := range n.Decl {
			if found := findNodeAtOffset(v, offset, content); found != nil {
				return found
			}
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			if found := findNodeAtOffset(arg, offset, content); found != nil {
				return found
			}
		}
	case *parse.IfNode:
		if found := findNodeInBranch(&n.BranchNode, offset, content); found != nil {
			return found
		}
		// Cursor is on the keyword itself — return the IfNode
		if isOnBranchKeyword(n, "if", offset, content) {
			return node
		}
	case *parse.RangeNode:
		if found := findNodeInBranch(&n.BranchNode, offset, content); found != nil {
			return found
		}
		if isOnBranchKeyword(n, "range", offset, content) {
			return node
		}
	case *parse.WithNode:
		if found := findNodeInBranch(&n.BranchNode, offset, content); found != nil {
			return found
		}
		if isOnBranchKeyword(n, "with", offset, content) {
			return node
		}
	case *parse.TemplateNode:
		if n.Pipe != nil {
			if found := findNodeAtOffset(n.Pipe, offset, content); found != nil {
				return found
			}
		}
		// If cursor is on the "template" keyword, return the node
		if isOnKeywordNear(int(n.Position()), "template", offset, content) {
			return node
		}
	}
	// Check if offset falls within this node
	nodeLen := len(node.String())
	if pos <= offset && offset < pos+nodeLen {
		// For CommandNode with a single arg, return the arg directly
		// so hover/definition/diagnostics can handle VariableNode/FieldNode etc.
		if cmd, ok := node.(*parse.CommandNode); ok && len(cmd.Args) == 1 {
			return cmd.Args[0]
		}
		return node
	}
	return nil
}

// ScopeChain represents the chain of range/with scopes enclosing a given offset.
// Each entry is a FieldNode from the pipe of a range/with, outermost first.
type ScopeEntry struct {
	Kind  string          // "range" or "with"
	Field *parse.FieldNode // the field being ranged/withed
	Vars  []string        // variable names declared in the pipe (e.g., "$i", "$v" from range)
}

// ScopeAtOffset returns the chain of range/with scopes enclosing the given offset.
func (pt *ParsedTemplate) ScopeAtOffset(offset int) []ScopeEntry {
	if pt.Trees == nil {
		return nil
	}
	for _, tree := range pt.Trees {
		if tree.Root == nil {
			continue
		}
		var scopes []ScopeEntry
		if findScopes(tree.Root, offset, &scopes) {
			return scopes
		}
	}
	return nil
}

func findScopes(node parse.Node, offset int, scopes *[]ScopeEntry) bool {
	if node == nil {
		return false
	}
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return false
		}
		for _, child := range n.Nodes {
			if findScopes(child, offset, scopes) {
				return true
			}
		}
	case *parse.ActionNode:
		pos := int(n.Position())
		if pos <= offset && offset < pos+len(n.String()) {
			return true
		}
	case *parse.RangeNode:
		return findScopesInBranch(&n.BranchNode, "range", offset, scopes)
	case *parse.WithNode:
		return findScopesInBranch(&n.BranchNode, "with", offset, scopes)
	case *parse.IfNode:
		return findScopesInBranch(&n.BranchNode, "", offset, scopes)
	}
	return false
}

func findScopesInBranch(n *parse.BranchNode, kind string, offset int, scopes *[]ScopeEntry) bool {
	// Check if offset is inside the body (List or ElseList)
	inBody := false
	if n.List != nil {
		pos := int(n.List.Position())
		if pos <= offset && offset < pos+len(n.List.String()) {
			inBody = true
			// Add scope entry for range/with before recursing into body
			if kind != "" {
				field := extractPipeField(n.Pipe)
				entry := ScopeEntry{Kind: kind, Field: field}
				if n.Pipe != nil {
					for _, v := range n.Pipe.Decl {
						entry.Vars = append(entry.Vars, v.Ident[0])
					}
				}
				*scopes = append(*scopes, entry)
			}
			findScopes(n.List, offset, scopes)
			return true
		}
	}
	if !inBody && n.ElseList != nil {
		pos := int(n.ElseList.Position())
		if pos <= offset && offset < pos+len(n.ElseList.String()) {
			// else block: don't re-scope (dot reverts to parent)
			findScopes(n.ElseList, offset, scopes)
			return true
		}
	}
	// Check pipe itself
	if n.Pipe != nil {
		pos := int(n.Pipe.Position())
		if pos <= offset && offset < pos+len(n.Pipe.String()) {
			return true
		}
	}
	return false
}

func extractPipeField(pipe *parse.PipeNode) *parse.FieldNode {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return nil
	}
	cmd := pipe.Cmds[0]
	if len(cmd.Args) == 0 {
		return nil
	}
	if fn, ok := cmd.Args[0].(*parse.FieldNode); ok {
		return fn
	}
	return nil
}

func findNodeInBranch(n *parse.BranchNode, offset int, content string) parse.Node {
	if n.Pipe != nil {
		if found := findNodeAtOffset(n.Pipe, offset, content); found != nil {
			return found
		}
	}
	if n.List != nil {
		if found := findNodeAtOffset(n.List, offset, content); found != nil {
			return found
		}
	}
	if n.ElseList != nil {
		if found := findNodeAtOffset(n.ElseList, offset, content); found != nil {
			return found
		}
	}
	return nil
}

// isOnBranchKeyword checks if offset is on the keyword of a branch node.
func isOnBranchKeyword(node parse.Node, keyword string, offset int, content string) bool {
	return isOnKeywordNear(int(node.Position()), keyword, offset, content)
}

// isOnKeywordNear searches for keyword near nodePos and checks if offset falls on it.
func isOnKeywordNear(nodePos int, keyword string, offset int, content string) bool {
	start := nodePos - len(keyword) - 10
	if start < 0 {
		start = 0
	}
	end := nodePos + len(keyword) + 10
	if end > len(content) {
		end = len(content)
	}
	sub := content[start:end]
	idx := strings.Index(sub, keyword)
	if idx < 0 {
		return false
	}
	kwStart := start + idx
	kwEnd := kwStart + len(keyword)
	return kwStart <= offset && offset < kwEnd
}
