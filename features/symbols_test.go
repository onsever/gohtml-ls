package features

import (
	"testing"

	tmpl "github.com/onsever/gohtml-ls/template"
)

func TestDocumentSymbols_WithDefine(t *testing.T) {
	content := `{{define "sidebar"}}Sidebar{{end}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	result := DocumentSymbols(pt)
	if len(result) == 0 {
		t.Error("expected at least 1 symbol for define block")
	}
	found := false
	for _, s := range result {
		if s.Name == "sidebar" {
			found = true
		}
	}
	if !found {
		t.Error("expected symbol named 'sidebar'")
	}
}

func TestDocumentSymbols_NoDefine(t *testing.T) {
	content := `<div>{{.Title}}</div>`
	pt := tmpl.Parse("file:///test.gohtml", content)
	result := DocumentSymbols(pt)
	if len(result) != 0 {
		t.Errorf("expected 0 symbols, got %d", len(result))
	}
}
