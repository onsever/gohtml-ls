package features

import (
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

// SemanticTokensFull returns delta-encoded semantic tokens for the document.
func SemanticTokensFull(pt *tmpl.ParsedTemplate, extraFuncs ...string) lsp.SemanticTokens {
	tokens := tmpl.ExtractSemanticTokens(pt, extraFuncs...)

	// Delta-encode per LSP spec: [deltaLine, deltaStartChar, length, tokenType, tokenModifiers]
	data := make([]uint32, 0, len(tokens)*5)
	var prevLine, prevChar uint32

	for _, t := range tokens {
		deltaLine := t.Line - prevLine
		deltaChar := t.StartChar
		if deltaLine == 0 {
			deltaChar = t.StartChar - prevChar
		}
		data = append(data, deltaLine, deltaChar, t.Length, t.Type, t.Modifiers)
		prevLine = t.Line
		prevChar = t.StartChar
	}

	return lsp.SemanticTokens{Data: data}
}

// SemanticTokenTypes returns the token type legend.
func SemanticTokenTypes() []string {
	return []string{"keyword", "variable", "property", "function", "string", "comment", "operator", "number"}
}

// SemanticTokenModifiers returns the token modifier legend.
func SemanticTokenModifiers() []string {
	return []string{"declaration", "defaultLibrary"}
}
