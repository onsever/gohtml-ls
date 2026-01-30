package features

import (
	"strings"
	"testing"

	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

func TestHover_FieldNode(t *testing.T) {
	content := `{{.Username}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	pos := lsp.Position{Line: 0, Character: 3}
	result := Hover(pt, pos, idx, nil)
	if result == nil {
		t.Fatal("expected hover result, got nil")
	}
	if !strings.Contains(result.Contents.Value, "Username") {
		t.Errorf("expected hover to contain 'Username', got %q", result.Contents.Value)
	}
}

func TestHover_BuiltinFunction(t *testing.T) {
	content := `{{len .Items}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	pos := lsp.Position{Line: 0, Character: 2}
	result := Hover(pt, pos, idx, nil)
	if result == nil {
		t.Fatal("expected hover result, got nil")
	}
	if !strings.Contains(result.Contents.Value, "len") {
		t.Errorf("expected hover to contain 'len', got %q", result.Contents.Value)
	}
}

func TestHover_TemplateNode(t *testing.T) {
	content := `{{define "nav"}}Nav{{end}}{{template "nav"}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	idx.Index(pt)
	// TemplateNode for "nav" is at position 37
	pos := pt.OffsetToPosition(37)
	result := Hover(pt, pos, idx, nil)
	if result == nil {
		t.Fatal("expected hover result for template node, got nil")
	}
	if !strings.Contains(result.Contents.Value, "nav") {
		t.Errorf("expected hover to contain 'nav', got %q", result.Contents.Value)
	}
}

func TestHover_DefineKeyword(t *testing.T) {
	content := `{{define "mytemplate"}}Hello{{end}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	idx.Index(pt)
	// "define" starts at offset 2
	pos := lsp.Position{Line: 0, Character: 3} // middle of "define"
	result := Hover(pt, pos, idx, nil)
	if result == nil {
		t.Fatal("expected hover result for define keyword, got nil")
	}
	if !strings.Contains(result.Contents.Value, "{{define}}") {
		t.Errorf("expected define keyword doc, got %q", result.Contents.Value)
	}
}

func TestHover_DefineTemplateName(t *testing.T) {
	content := `{{define "mytemplate"}}Hello{{end}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	idx.Index(pt)
	// template name "mytemplate" starts at offset 10 (after {{define ")
	pos := lsp.Position{Line: 0, Character: 12} // middle of "mytemplate"
	result := Hover(pt, pos, idx, nil)
	if result == nil {
		t.Fatal("expected hover result for template name, got nil")
	}
	if !strings.Contains(result.Contents.Value, "mytemplate") {
		t.Errorf("expected template name in hover, got %q", result.Contents.Value)
	}
}

func TestHover_BlockKeyword(t *testing.T) {
	content := `{{block "sidebar" .}}Default{{end}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	idx.Index(pt)
	pos := lsp.Position{Line: 0, Character: 3} // middle of "block"
	result := Hover(pt, pos, idx, nil)
	if result == nil {
		t.Fatal("expected hover result for block keyword, got nil")
	}
	if !strings.Contains(result.Contents.Value, "{{block}}") {
		t.Errorf("expected block keyword doc, got %q", result.Contents.Value)
	}
}

func TestHover_OutsideAction(t *testing.T) {
	content := `<div>hello</div>`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	bindings := []goanalysis.TemplateBinding{}
	pos := lsp.Position{Line: 0, Character: 5}
	result := Hover(pt, pos, idx, bindings)
	if result != nil {
		t.Errorf("expected nil hover outside actions, got %v", result)
	}
}
