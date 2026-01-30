package features

import (
	"testing"

	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

func TestReferences_DefineAndCall(t *testing.T) {
	content := `{{define "footer"}}F{{end}}{{template "footer"}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	idx.Index(pt)
	// TemplateNode for "footer" is at position 38
	pos := pt.OffsetToPosition(38)
	result := References(pt, pos, idx, true)
	if len(result) < 2 {
		t.Errorf("expected at least 2 locations (define + call), got %d", len(result))
	}
}

func TestReferences_NoReferences(t *testing.T) {
	content := `<div>hello</div>`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	pos := lsp.Position{Line: 0, Character: 5}
	result := References(pt, pos, idx, true)
	if result != nil {
		t.Errorf("expected nil for no references, got %v", result)
	}
}
