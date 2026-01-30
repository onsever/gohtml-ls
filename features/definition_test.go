package features

import (
	"testing"

	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

func TestDefinition_TemplateCall(t *testing.T) {
	content := `{{define "header"}}<h1>H</h1>{{end}}{{template "header"}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	idx.Index(pt)
	// The TemplateNode position is inside the node, after {{template "
	// Use a position inside the template call action
	pos := pt.OffsetToPosition(47) // position of the template name inside the call
	result := Definition(pt, pos, idx, nil)
	if result == nil {
		t.Fatal("expected definition location, got nil")
	}
	if result.URI != "file:///test.gohtml" {
		t.Errorf("expected URI file:///test.gohtml, got %s", result.URI)
	}
}

func TestDefinition_RegularText(t *testing.T) {
	content := `<div>hello</div>`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	pos := lsp.Position{Line: 0, Character: 5}
	result := Definition(pt, pos, idx, nil)
	if result != nil {
		t.Errorf("expected nil for regular text, got %v", result)
	}
}

func TestDefinition_FieldToGoSource(t *testing.T) {
	content := `{{.Username}}`
	pt := tmpl.Parse("file:///test.gohtml", content)
	idx := tmpl.NewTemplateIndex()
	idx.Index(pt)

	bindings := []goanalysis.TemplateBinding{{
		TemplateName: "test.gohtml",
		ParsedFiles:  []string{"test.gohtml"},
		DataType: &goanalysis.TypeInfo{
			Name: "HomeData",
			Fields: map[string]goanalysis.FieldInfo{
				"Username": {
					Name:     "Username",
					TypeName: "string",
					GoFile:   "/project/types.go",
					Line:     5,
					Col:      1,
				},
			},
			Methods: make(map[string]goanalysis.MethodInfo),
		},
		FuncMaps: make(map[string]goanalysis.FuncSig),
	}}

	// Position inside .Username
	pos := pt.OffsetToPosition(3) // should be inside the field node
	result := Definition(pt, pos, idx, bindings)
	if result == nil {
		t.Fatal("expected definition location for .Username, got nil")
	}
	expectedURI := "file:///project/types.go"
	if result.URI != expectedURI {
		t.Errorf("expected URI %s, got %s", expectedURI, result.URI)
	}
	if result.Range.Start.Line != 5 {
		t.Errorf("expected line 5, got %d", result.Range.Start.Line)
	}
}
