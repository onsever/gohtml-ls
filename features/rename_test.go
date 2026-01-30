package features

import (
	"testing"

	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

func TestRename_TemplateNode(t *testing.T) {
	content := `{{define "header"}}Header{{end}}
{{template "header" .}}
{{template "header" .}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	index := tmpl.NewTemplateIndex()
	index.Index(pt)

	// Rename from the template call at line 1
	pos := lsp.Position{Line: 1, Character: 13} // on "header" in {{template "header"}}
	edit := Rename(pt, pos, "nav", index)
	if edit == nil {
		t.Fatal("expected WorkspaceEdit, got nil")
	}
	edits, ok := edit.Changes["file:///test.gohtml"]
	if !ok {
		t.Fatal("expected edits for test file")
	}
	// Should have 3 edits: 1 define + 2 template calls
	if len(edits) != 3 {
		t.Fatalf("expected 3 edits, got %d", len(edits))
	}
	for _, e := range edits {
		if e.NewText != "nav" {
			t.Errorf("expected NewText 'nav', got %q", e.NewText)
		}
	}
}

func TestPrepareRename_OnTemplateName(t *testing.T) {
	content := `{{define "myblock"}}Content{{end}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	index := tmpl.NewTemplateIndex()
	index.Index(pt)

	// Cursor on define name
	pos := lsp.Position{Line: 0, Character: 12} // on "myblock"
	rng := PrepareRename(pt, pos, index)
	if rng == nil {
		t.Fatal("expected Range, got nil")
	}
}

func TestPrepareRename_NotOnName(t *testing.T) {
	content := `<div>Hello</div>`
	pt := tmpl.Parse("file:///test.gohtml", content)
	index := tmpl.NewTemplateIndex()
	index.Index(pt)

	pos := lsp.Position{Line: 0, Character: 3}
	rng := PrepareRename(pt, pos, index)
	if rng != nil {
		t.Fatal("expected nil for non-template position")
	}
}
