package features

import (
	"testing"

	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

func testBindings() []goanalysis.TemplateBinding {
	return []goanalysis.TemplateBinding{{
		TemplateName: "test.gohtml",
		ParsedFiles:  []string{"test.gohtml"},
		DataType: &goanalysis.TypeInfo{
			Name: "HomeData",
			Fields: map[string]goanalysis.FieldInfo{
				"Username": {Name: "Username", TypeName: "string"},
				"Age":      {Name: "Age", TypeName: "int"},
			},
			Methods: map[string]goanalysis.MethodInfo{},
		},
		FuncMaps: map[string]goanalysis.FuncSig{},
	}}
}

func TestCompletion_FieldCompletions(t *testing.T) {
	content := `{{.`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	pos := lsp.Position{Line: 0, Character: 3}
	result := Completion(pt, pos, idx, testBindings())
	if len(result.Items) < 2 {
		t.Fatalf("expected at least 2 field completions, got %d", len(result.Items))
	}
	fields := map[string]bool{}
	for _, item := range result.Items {
		fields[item.Label] = true
	}
	if !fields["Username"] || !fields["Age"] {
		t.Errorf("expected Username and Age fields, got %v", fields)
	}
}

func TestCompletion_TemplateNameCompletions(t *testing.T) {
	// First create a template with a define so the index has it
	defined := `{{define "header"}}H{{end}}`
	ptDef := tmpl.Parse("file:///other.gohtml", defined)
	idx := tmpl.NewTemplateIndex()
	idx.Index(ptDef)

	content := `{{template "`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx.Index(pt)
	pos := lsp.Position{Line: 0, Character: uint32(len(content))}
	result := Completion(pt, pos, idx, nil)
	found := false
	for _, item := range result.Items {
		if item.Label == "header" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'header' in template name completions, got %v", result.Items)
	}
}

func TestCompletion_KeywordCompletions(t *testing.T) {
	content := `{{`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	pos := lsp.Position{Line: 0, Character: 2}
	result := Completion(pt, pos, idx, nil)
	keywords := map[string]bool{}
	for _, item := range result.Items {
		if item.Kind == lsp.CompletionItemKindKeyword {
			keywords[item.Label] = true
		}
	}
	for _, kw := range []string{"if", "range", "end", "with", "template"} {
		if !keywords[kw] {
			t.Errorf("expected keyword %q in completions", kw)
		}
	}
}

func TestCompletion_SnippetCompletionsOutsideAction(t *testing.T) {
	content := `<div>`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	pos := lsp.Position{Line: 0, Character: 5}
	result := Completion(pt, pos, idx, nil)
	if len(result.Items) == 0 {
		t.Error("expected snippet completions outside actions")
	}
	foundSnippet := false
	for _, item := range result.Items {
		if item.Kind == lsp.CompletionItemKindSnippet {
			foundSnippet = true
		}
	}
	if !foundSnippet {
		t.Error("expected at least one snippet completion")
	}
}

func TestCompletion_FieldCompletionsNoBindings(t *testing.T) {
	content := `{{.`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	pos := lsp.Position{Line: 0, Character: 3}
	result := Completion(pt, pos, idx, nil)
	if len(result.Items) != 0 {
		t.Errorf("expected 0 field completions with no bindings, got %d", len(result.Items))
	}
}

func TestCompletion_FieldCompletionsForParseGlobBinding(t *testing.T) {
	ti := goanalysis.NewTypeInfo("PageData", "")
	ti.Fields["Title"] = goanalysis.FieldInfo{Name: "Title", TypeName: "string"}

	pt := tmpl.Parse("file:///project/templates/page.gohtml", `{{.`)
	idx := tmpl.NewTemplateIndex()
	bindings := []goanalysis.TemplateBinding{
		{
			TemplateName: "*.gohtml",
			ParsedFiles:  []string{"templates/*.gohtml"},
			DataType:     ti,
			FuncMaps:     map[string]goanalysis.FuncSig{},
		},
	}

	result := Completion(pt, lsp.Position{Line: 0, Character: 3}, idx, bindings)
	if !hasCompletionLabel(result.Items, "Title") {
		t.Fatalf("expected glob binding to provide Title completion, got %#v", result.Items)
	}
}

func hasCompletionLabel(items []lsp.CompletionItem, label string) bool {
	for _, item := range items {
		if item.Label == label {
			return true
		}
	}
	return false
}
