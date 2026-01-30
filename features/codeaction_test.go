package features

import (
	"testing"

	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

func TestCodeActions_TypoSuggestion(t *testing.T) {
	content := `{{.Usrname}}`
	pt := tmpl.Parse("file:///test.gohtml", content)

	userType := goanalysis.NewTypeInfo("User", "")
	userType.Fields["Username"] = goanalysis.FieldInfo{Name: "Username", TypeName: "string"}
	userType.Fields["Email"] = goanalysis.FieldInfo{Name: "Email", TypeName: "string"}

	bindings := []goanalysis.TemplateBinding{
		{DataType: userType, TemplateName: "test.gohtml"},
	}

	diag := lsp.Diagnostic{
		Range:    lsp.Range{Start: lsp.Position{Line: 0, Character: 2}, End: lsp.Position{Line: 0, Character: 10}},
		Severity: lsp.DiagnosticSeverityWarning,
		Source:   "gohtml-lsp",
		Message:  "undefined field or method: Usrname on User",
	}

	params := lsp.CodeActionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: "file:///test.gohtml"},
		Range:        diag.Range,
		Context:      lsp.CodeActionContext{Diagnostics: []lsp.Diagnostic{diag}},
	}

	actions := CodeActions(pt, params, bindings)
	if len(actions) == 0 {
		t.Fatal("expected at least one code action")
	}
	if actions[0].Title != "Did you mean 'Username'?" {
		t.Errorf("expected suggestion for 'Username', got %q", actions[0].Title)
	}
	if actions[0].Edit == nil {
		t.Fatal("expected edit in code action")
	}
}

func TestCodeActions_NoSuggestionForDistantName(t *testing.T) {
	content := `{{.XYZ}}`
	pt := tmpl.Parse("file:///test.gohtml", content)

	userType := goanalysis.NewTypeInfo("User", "")
	userType.Fields["Username"] = goanalysis.FieldInfo{Name: "Username", TypeName: "string"}

	bindings := []goanalysis.TemplateBinding{
		{DataType: userType, TemplateName: "test.gohtml"},
	}

	diag := lsp.Diagnostic{
		Range:    lsp.Range{Start: lsp.Position{Line: 0, Character: 2}, End: lsp.Position{Line: 0, Character: 6}},
		Severity: lsp.DiagnosticSeverityWarning,
		Source:   "gohtml-lsp",
		Message:  "undefined field or method: XYZ on User",
	}

	params := lsp.CodeActionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: "file:///test.gohtml"},
		Range:        diag.Range,
		Context:      lsp.CodeActionContext{Diagnostics: []lsp.Diagnostic{diag}},
	}

	actions := CodeActions(pt, params, bindings)
	if len(actions) != 0 {
		t.Errorf("expected no code actions for distant name, got %d", len(actions))
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"usrname", "username", 1},
		{"titl", "title", 1},
		{"xyz", "username", 8},
	}
	for _, tt := range tests {
		got := levenshtein(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
