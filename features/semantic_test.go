package features

import (
	"testing"

	tmpl "github.com/onsever/gohtml-ls/template"
)

func TestSemanticTokensFull_WithField(t *testing.T) {
	content := `{{.Name}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	result := SemanticTokensFull(pt)
	if len(result.Data) == 0 {
		t.Error("expected non-empty semantic tokens data for template with field")
	}
}

func TestSemanticTokensFull_EmptyTemplate(t *testing.T) {
	content := `<div>plain html</div>`
	pt := tmpl.Parse("file:///test.gohtml", content)
	result := SemanticTokensFull(pt)
	if len(result.Data) != 0 {
		t.Errorf("expected empty semantic tokens data, got %v", result.Data)
	}
}

func TestSemanticTokenTypes(t *testing.T) {
	types := SemanticTokenTypes()
	if len(types) != 8 {
		t.Errorf("expected 8 token types, got %d: %v", len(types), types)
	}
}

func TestSemanticTokenModifiers(t *testing.T) {
	mods := SemanticTokenModifiers()
	if len(mods) != 2 {
		t.Errorf("expected 2 token modifiers, got %d: %v", len(mods), mods)
	}
}

func TestSemanticTokensFull_BuiltinModifier(t *testing.T) {
	content := `{{len .Items}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	result := SemanticTokensFull(pt)
	// Should have tokens; check that at least one has defaultLibrary modifier (bit 1 = 2)
	found := false
	for i := 4; i < len(result.Data); i += 5 {
		if result.Data[i]&2 != 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected defaultLibrary modifier on built-in function token")
	}
}

func TestSemanticTokensFull_Number(t *testing.T) {
	content := `{{printf "%d" 42}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	result := SemanticTokensFull(pt)
	// Token type 7 = number
	found := false
	for i := 3; i < len(result.Data); i += 5 {
		if result.Data[i] == 7 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected number token type for numeric literal")
	}
}

func TestSemanticTokensFull_Keyword(t *testing.T) {
	content := `{{if .X}}yes{{end}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	result := SemanticTokensFull(pt)
	// Token type 0 = keyword; should have at least one for "if"
	found := false
	for i := 3; i < len(result.Data); i += 5 {
		if result.Data[i] == 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected keyword token type for 'if'")
	}
}

func TestSemanticTokensFull_PipeOperator(t *testing.T) {
	content := `{{.Name | len}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	result := SemanticTokensFull(pt)
	// Token type 6 = operator
	found := false
	for i := 3; i < len(result.Data); i += 5 {
		if result.Data[i] == 6 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected operator token type for pipe '|'")
	}
}
