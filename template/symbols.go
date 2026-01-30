package template

import (
	"strings"
	"text/template/parse"

	"github.com/onsever/gohtml-ls/lsp"
)

// ExtractSymbols returns document symbols for define/block in the template.
func ExtractSymbols(pt *ParsedTemplate) []lsp.DocumentSymbol {
	var symbols []lsp.DocumentSymbol
	if pt.Trees == nil {
		return symbols
	}
	for name, tree := range pt.Trees {
		if name == "" || tree.Root == nil {
			continue
		}
		defOffset := findDefineOffset(pt.Content, name)
		if defOffset < 0 {
			defOffset = 0
		}
		startPos := pt.OffsetToPosition(defOffset)
		// Estimate end by the tree's root extent
		endOffset := defOffset + len(tree.Root.String()) + 20
		if endOffset > len(pt.Content) {
			endOffset = len(pt.Content)
		}
		endPos := pt.OffsetToPosition(endOffset)
		selEnd := startPos
		selEnd.Character += uint32(len(name))
		symbols = append(symbols, lsp.DocumentSymbol{
			Name:           name,
			Detail:         "template",
			Kind:           lsp.SymbolKindNamespace,
			Range:          lsp.Range{Start: startPos, End: endPos},
			SelectionRange: lsp.Range{Start: startPos, End: selEnd},
		})
	}
	return symbols
}

// SemanticToken represents a single semantic token for highlighting.
type SemanticToken struct {
	Line      uint32
	StartChar uint32
	Length    uint32
	Type      uint32 // index into token types legend
	Modifiers uint32
}

// Token type indices (must match legend in server capabilities).
const (
	TokenKeyword  = 0
	TokenVariable = 1
	TokenProperty = 2
	TokenFunction = 3
	TokenString   = 4
	TokenComment  = 5
	TokenOperator = 6
	TokenNumber   = 7
)

// Token modifier bitmask values.
const (
	ModDeclaration    = 1 << 0 // 1
	ModDefaultLibrary = 1 << 1 // 2
)

// builtinFuncNames is the set of Go template built-in function names.
var builtinFuncNames = map[string]bool{
	"and": true, "call": true, "html": true, "index": true,
	"slice": true, "js": true, "len": true, "not": true,
	"or": true, "print": true, "printf": true, "println": true,
	"urlquery": true, "eq": true, "ne": true, "lt": true,
	"le": true, "gt": true, "ge": true,
}

// ExtractSemanticTokens walks the parse tree and extracts semantic tokens.
func ExtractSemanticTokens(pt *ParsedTemplate, extraFuncs ...string) []SemanticToken {
	var tokens []SemanticToken
	if pt.Trees == nil {
		return tokens
	}
	customFuncs := make(map[string]bool, len(extraFuncs))
	for _, f := range extraFuncs {
		customFuncs[f] = true
	}
	for name, tree := range pt.Trees {
		if tree.Root == nil {
			continue
		}
		// Emit keyword tokens for {{define "name"}} and {{block "name"}}
		if name != "" {
			emitDefineKeywordTokens(pt, name, &tokens)
		}
		walkSemanticTokens(tree.Root, pt, &tokens, customFuncs)
	}
	sortTokens(tokens)
	return tokens
}

// emitDefineKeywordTokens emits keyword + declaration string tokens for {{define "name"}} or {{block "name"}}.
func emitDefineKeywordTokens(pt *ParsedTemplate, name string, tokens *[]SemanticToken) {
	// Find {{define "name"}} or {{block "name"}} in content
	for _, kw := range []string{"define", "block"} {
		search := kw + " \"" + name + "\""
		idx := strings.Index(pt.Content, search)
		if idx < 0 {
			continue
		}
		// keyword token
		kwPos := pt.OffsetToPosition(idx)
		*tokens = append(*tokens, SemanticToken{
			Line: kwPos.Line, StartChar: kwPos.Character,
			Length: uint32(len(kw)), Type: TokenKeyword,
		})
		// template name string with declaration modifier
		strOffset := idx + len(kw) + 1 // skip keyword + space
		strPos := pt.OffsetToPosition(strOffset)
		*tokens = append(*tokens, SemanticToken{
			Line: strPos.Line, StartChar: strPos.Character,
			Length: uint32(len(name) + 2), Type: TokenString, Modifiers: ModDeclaration,
		})
	}
}

func walkSemanticTokens(node parse.Node, pt *ParsedTemplate, tokens *[]SemanticToken, customFuncs map[string]bool) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				walkSemanticTokens(child, pt, tokens, customFuncs)
			}
		}
	case *parse.ActionNode:
		if n.Pipe != nil {
			walkSemanticTokens(n.Pipe, pt, tokens, customFuncs)
		}
	case *parse.PipeNode:
		// Emit pipe operators between commands
		if len(n.Cmds) > 1 {
			emitPipeOperators(n, pt, tokens)
		}
		for _, cmd := range n.Cmds {
			walkSemanticTokens(cmd, pt, tokens, customFuncs)
		}
		for _, v := range n.Decl {
			walkSemanticTokens(v, pt, tokens, customFuncs)
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			walkSemanticTokens(arg, pt, tokens, customFuncs)
		}
	case *parse.FieldNode:
		pos := pt.OffsetToPosition(int(n.Position()))
		*tokens = append(*tokens, SemanticToken{
			Line: pos.Line, StartChar: pos.Character,
			Length: uint32(len(n.String())), Type: TokenProperty,
		})
	case *parse.VariableNode:
		pos := pt.OffsetToPosition(int(n.Position()))
		*tokens = append(*tokens, SemanticToken{
			Line: pos.Line, StartChar: pos.Character,
			Length: uint32(len(n.String())), Type: TokenVariable,
		})
	case *parse.IdentifierNode:
		pos := pt.OffsetToPosition(int(n.Position()))
		mod := uint32(0)
		if builtinFuncNames[n.Ident] {
			mod = ModDefaultLibrary
		}
		*tokens = append(*tokens, SemanticToken{
			Line: pos.Line, StartChar: pos.Character,
			Length: uint32(len(n.Ident)), Type: TokenFunction, Modifiers: mod,
		})
	case *parse.StringNode:
		pos := pt.OffsetToPosition(int(n.Position()))
		*tokens = append(*tokens, SemanticToken{
			Line: pos.Line, StartChar: pos.Character,
			Length: uint32(len(n.Quoted)), Type: TokenString,
		})
	case *parse.NumberNode:
		pos := pt.OffsetToPosition(int(n.Position()))
		*tokens = append(*tokens, SemanticToken{
			Line: pos.Line, StartChar: pos.Character,
			Length: uint32(len(n.Text)), Type: TokenNumber,
		})
	case *parse.BoolNode:
		pos := pt.OffsetToPosition(int(n.Position()))
		text := "false"
		if n.True {
			text = "true"
		}
		*tokens = append(*tokens, SemanticToken{
			Line: pos.Line, StartChar: pos.Character,
			Length: uint32(len(text)), Type: TokenKeyword,
		})
	case *parse.NilNode:
		pos := pt.OffsetToPosition(int(n.Position()))
		*tokens = append(*tokens, SemanticToken{
			Line: pos.Line, StartChar: pos.Character,
			Length: 3, Type: TokenKeyword,
		})
	case *parse.IfNode:
		emitBranchKeyword(pt, int(n.Position()), "if", tokens)
		walkBranchSemanticTokens(&n.BranchNode, pt, tokens, customFuncs)
	case *parse.RangeNode:
		emitBranchKeyword(pt, int(n.Position()), "range", tokens)
		walkBranchSemanticTokens(&n.BranchNode, pt, tokens, customFuncs)
	case *parse.WithNode:
		emitBranchKeyword(pt, int(n.Position()), "with", tokens)
		walkBranchSemanticTokens(&n.BranchNode, pt, tokens, customFuncs)
	case *parse.TemplateNode:
		// Emit "template" keyword
		emitBranchKeyword(pt, int(n.Position()), "template", tokens)
		// Emit template name as string
		nameOffset := findTemplateNameOffset(pt.Content, int(n.Position()), n.Name)
		if nameOffset >= 0 {
			namePos := pt.OffsetToPosition(nameOffset)
			*tokens = append(*tokens, SemanticToken{
				Line: namePos.Line, StartChar: namePos.Character,
				Length: uint32(len(n.Name) + 2), Type: TokenString,
			})
		}
		if n.Pipe != nil {
			walkSemanticTokens(n.Pipe, pt, tokens, customFuncs)
		}
	case *parse.CommentNode:
		pos := pt.OffsetToPosition(int(n.Position()))
		*tokens = append(*tokens, SemanticToken{
			Line: pos.Line, StartChar: pos.Character,
			Length: uint32(len(n.Text) + 7), Type: TokenComment,
		})
	}
}

func walkBranchSemanticTokens(n *parse.BranchNode, pt *ParsedTemplate, tokens *[]SemanticToken, customFuncs map[string]bool) {
	if n.Pipe != nil {
		walkSemanticTokens(n.Pipe, pt, tokens, customFuncs)
	}
	if n.List != nil {
		walkSemanticTokens(n.List, pt, tokens, customFuncs)
	}
	if n.ElseList != nil {
		// Emit "else" keyword
		emitElseKeyword(pt, n, tokens)
		walkSemanticTokens(n.ElseList, pt, tokens, customFuncs)
	}
}

// emitBranchKeyword emits a keyword token for a branch/template node.
// nodePos is the node's Position() which may point to the pipe or keyword area.
func emitBranchKeyword(pt *ParsedTemplate, nodePos int, keyword string, tokens *[]SemanticToken) {
	content := pt.Content
	if nodePos >= len(content) {
		return
	}
	// Search in a window around nodePos (before and after) for the keyword
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
		return
	}
	kwOffset := start + idx
	pos := pt.OffsetToPosition(kwOffset)
	*tokens = append(*tokens, SemanticToken{
		Line: pos.Line, StartChar: pos.Character,
		Length: uint32(len(keyword)), Type: TokenKeyword,
	})
}

// emitElseKeyword finds and emits the "else" keyword between List and ElseList.
func emitElseKeyword(pt *ParsedTemplate, n *parse.BranchNode, tokens *[]SemanticToken) {
	if n.List == nil || n.ElseList == nil {
		return
	}
	// Search for {{else}} or {{- else between end of List and start of ElseList
	listEnd := int(n.List.Position()) + len(n.List.String())
	content := pt.Content
	if listEnd >= len(content) {
		return
	}
	searchEnd := listEnd + 30
	if searchEnd > len(content) {
		searchEnd = len(content)
	}
	sub := content[listEnd:searchEnd]
	idx := strings.Index(sub, "else")
	if idx < 0 {
		return
	}
	elseOffset := listEnd + idx
	pos := pt.OffsetToPosition(elseOffset)
	*tokens = append(*tokens, SemanticToken{
		Line: pos.Line, StartChar: pos.Character,
		Length: 4, Type: TokenKeyword,
	})
}

// emitPipeOperators emits operator tokens for | characters between commands in a pipe.
func emitPipeOperators(n *parse.PipeNode, pt *ParsedTemplate, tokens *[]SemanticToken) {
	content := pt.Content
	for i := 0; i < len(n.Cmds)-1; i++ {
		// Search for | between end of cmd[i] and start of cmd[i+1]
		cmdEnd := int(n.Cmds[i].Position()) + len(n.Cmds[i].String())
		nextStart := int(n.Cmds[i+1].Position())
		if cmdEnd >= len(content) || nextStart > len(content) {
			continue
		}
		sub := content[cmdEnd:nextStart]
		idx := strings.Index(sub, "|")
		if idx >= 0 {
			pipeOffset := cmdEnd + idx
			pos := pt.OffsetToPosition(pipeOffset)
			*tokens = append(*tokens, SemanticToken{
				Line: pos.Line, StartChar: pos.Character,
				Length: 1, Type: TokenOperator,
			})
		}
	}
}

// findTemplateNameOffset finds the offset of the quoted template name string near nodePos.
func findTemplateNameOffset(content string, nodePos int, name string) int {
	search := "\"" + name + "\""
	end := nodePos + len(search) + 30
	if end > len(content) {
		end = len(content)
	}
	sub := content[nodePos:end]
	idx := strings.Index(sub, search)
	if idx < 0 {
		return -1
	}
	return nodePos + idx
}

func sortTokens(tokens []SemanticToken) {
	for i := 1; i < len(tokens); i++ {
		key := tokens[i]
		j := i - 1
		for j >= 0 && (tokens[j].Line > key.Line || (tokens[j].Line == key.Line && tokens[j].StartChar > key.StartChar)) {
			tokens[j+1] = tokens[j]
			j--
		}
		tokens[j+1] = key
	}
}

