package template

import (
	"testing"

	"github.com/onsever/gohtml-ls/lsp"
)

func posOf(line, char uint32) lsp.Position {
	return lsp.Position{Line: line, Character: char}
}

func TestExtractSymbols_OneDefine(t *testing.T) {
	pt := Parse("file:///test.html", `{{define "header"}}<h1/>{{end}}`)
	syms := ExtractSymbols(pt)
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(syms))
	}
	if syms[0].Name != "header" {
		t.Fatalf("expected name 'header', got %q", syms[0].Name)
	}
}

func TestExtractSymbols_MultipleDefines(t *testing.T) {
	content := `{{define "a"}}A{{end}}{{define "b"}}B{{end}}`
	pt := Parse("file:///test.html", content)
	syms := ExtractSymbols(pt)
	if len(syms) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(syms))
	}
	names := map[string]bool{}
	for _, s := range syms {
		names[s.Name] = true
	}
	if !names["a"] || !names["b"] {
		t.Fatalf("expected symbols 'a' and 'b', got %v", names)
	}
}

func TestExtractSymbols_NoDefines(t *testing.T) {
	pt := Parse("file:///test.html", `<h1>{{.Title}}</h1>`)
	syms := ExtractSymbols(pt)
	if len(syms) != 0 {
		t.Fatalf("expected 0 symbols, got %d", len(syms))
	}
}

func TestExtractSymbols_Block(t *testing.T) {
	pt := Parse("file:///test.html", `{{block "nav" .}}<nav/>{{end}}`)
	syms := ExtractSymbols(pt)
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(syms))
	}
	if syms[0].Name != "nav" {
		t.Fatalf("expected name 'nav', got %q", syms[0].Name)
	}
}

func TestExtractSymbols_ParseError(t *testing.T) {
	pt := Parse("file:///test.html", `{{define "broken"`)
	syms := ExtractSymbols(pt)
	if len(syms) != 0 {
		t.Fatalf("expected 0 symbols on parse error, got %d", len(syms))
	}
}

func TestExtractSemanticTokens_Field(t *testing.T) {
	pt := Parse("file:///test.html", `{{.Name}}`)
	tokens := ExtractSemanticTokens(pt)
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenProperty {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected TokenProperty for field node")
	}
}

func TestExtractSemanticTokens_Variable(t *testing.T) {
	pt := Parse("file:///test.html", `{{$x := "hi"}}{{$x}}`)
	tokens := ExtractSemanticTokens(pt)
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenVariable {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected TokenVariable for variable node")
	}
}

func TestExtractSemanticTokens_Function(t *testing.T) {
	pt := Parse("file:///test.html", `{{len .Items}}`)
	tokens := ExtractSemanticTokens(pt)
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenFunction {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected TokenFunction for function call")
	}
}

func TestExtractSemanticTokens_String(t *testing.T) {
	pt := Parse("file:///test.html", `{{printf "hello"}}`)
	tokens := ExtractSemanticTokens(pt)
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenString {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected TokenString for string literal")
	}
}

func TestExtractSemanticTokens_Empty(t *testing.T) {
	pt := Parse("file:///test.html", ``)
	tokens := ExtractSemanticTokens(pt)
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens for empty template, got %d", len(tokens))
	}
}
