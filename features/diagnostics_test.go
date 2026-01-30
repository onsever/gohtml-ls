package features

import (
	"strings"
	"testing"

	"github.com/onsever/gohtml-ls/goanalysis"
	tmpl "github.com/onsever/gohtml-ls/template"
)

func setupTemplate(uri, content string) (*tmpl.ParsedTemplate, *tmpl.TemplateIndex) {
	pt := tmpl.Parse(uri, content)
	idx := tmpl.NewTemplateIndex()
	idx.Index(pt)
	return pt, idx
}

func TestComputeDiagnostics_ValidTemplate(t *testing.T) {
	pt, idx := setupTemplate("file:///test.gohtml", `<h1>{{.Title}}</h1>`)
	diags := ComputeDiagnostics(pt, idx, nil)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestComputeDiagnostics_SyntaxError(t *testing.T) {
	pt, idx := setupTemplate("file:///test.gohtml", `{{if}}`)
	diags := ComputeDiagnostics(pt, idx, nil)
	if len(diags) == 0 {
		t.Error("expected at least 1 diagnostic for syntax error")
	}
	found := false
	for _, d := range diags {
		if d.Severity == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected an error severity diagnostic")
	}
}

func TestComputeDiagnostics_UndefinedTemplate(t *testing.T) {
	pt, idx := setupTemplate("file:///test.gohtml", `{{template "missing"}}`)
	diags := ComputeDiagnostics(pt, idx, nil)
	found := false
	for _, d := range diags {
		if d.Message == "undefined template: missing" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about undefined template, got %v", diags)
	}
}

func TestComputeDiagnostics_DefinedTemplate(t *testing.T) {
	content := `{{define "header"}}<h1>Header</h1>{{end}}{{template "header"}}`
	pt, idx := setupTemplate("file:///test.gohtml", content)
	diags := ComputeDiagnostics(pt, idx, nil)
	for _, d := range diags {
		if d.Message == "undefined template: header" {
			t.Error("should not warn about defined template")
		}
	}
}

func makeBindings(typeName string, fields map[string]string) []goanalysis.TemplateBinding {
	fi := make(map[string]goanalysis.FieldInfo)
	for name, typ := range fields {
		fi[name] = goanalysis.FieldInfo{Name: name, TypeName: typ}
	}
	return []goanalysis.TemplateBinding{{
		TemplateName: "test.gohtml",
		ParsedFiles:  []string{"test.gohtml"},
		DataType: &goanalysis.TypeInfo{
			Name:    typeName,
			Fields:  fi,
			Methods: make(map[string]goanalysis.MethodInfo),
		},
		FuncMaps: make(map[string]goanalysis.FuncSig),
	}}
}

func TestComputeDiagnostics_UndefinedField(t *testing.T) {
	pt, idx := setupTemplate("file:///test.gohtml", `<h1>{{.Usrname}}</h1>`)
	bindings := makeBindings("HomeData", map[string]string{
		"Username": "string",
		"Age":      "int",
	})
	diags := ComputeDiagnostics(pt, idx, bindings)

	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "Usrname") && strings.Contains(d.Message, "HomeData") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about undefined field 'Usrname', got %v", diags)
	}
}

func TestComputeDiagnostics_ValidField(t *testing.T) {
	pt, idx := setupTemplate("file:///test.gohtml", `<h1>{{.Username}}</h1>`)
	bindings := makeBindings("HomeData", map[string]string{
		"Username": "string",
		"Age":      "int",
	})
	diags := ComputeDiagnostics(pt, idx, bindings)

	for _, d := range diags {
		if strings.Contains(d.Message, "undefined field") {
			t.Errorf("should not warn about valid field, got: %s", d.Message)
		}
	}
}

func TestComputeDiagnostics_MultipleUndefinedFields(t *testing.T) {
	pt, idx := setupTemplate("file:///test.gohtml", `{{.Foo}} {{.Bar}} {{.Username}}`)
	bindings := makeBindings("HomeData", map[string]string{
		"Username": "string",
		"Age":      "int",
	})
	diags := ComputeDiagnostics(pt, idx, bindings)

	var undefinedFields []string
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined field") {
			undefinedFields = append(undefinedFields, d.Message)
		}
	}
	if len(undefinedFields) != 2 {
		t.Errorf("expected 2 undefined field warnings (Foo, Bar), got %d: %v", len(undefinedFields), undefinedFields)
	}
}

func TestComputeDiagnostics_FieldInIfBlock(t *testing.T) {
	pt, idx := setupTemplate("file:///test.gohtml", `{{if .IsAdmin}}admin{{end}}`)
	bindings := makeBindings("User", map[string]string{
		"Name": "string",
	})
	diags := ComputeDiagnostics(pt, idx, bindings)

	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "IsAdmin") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about undefined field 'IsAdmin' inside if block")
	}
}

func TestComputeDiagnostics_FieldInRangeBlock(t *testing.T) {
	pt, idx := setupTemplate("file:///test.gohtml", `{{range .Itemz}}{{end}}`)
	bindings := makeBindings("Page", map[string]string{
		"Items": "[]Item",
	})
	diags := ComputeDiagnostics(pt, idx, bindings)

	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "Itemz") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about undefined field 'Itemz' in range")
	}
}

func TestComputeDiagnostics_NoBindings(t *testing.T) {
	// With no bindings, field validation should be skipped — no false positives
	pt, idx := setupTemplate("file:///test.gohtml", `{{.Whatever}}`)
	diags := ComputeDiagnostics(pt, idx, nil)

	for _, d := range diags {
		if strings.Contains(d.Message, "undefined field") {
			t.Errorf("should not warn about fields when no bindings available, got: %s", d.Message)
		}
	}
}

func TestComputeDiagnostics_ChainedField(t *testing.T) {
	// .User.Name — should only check first field (.User) against the root type
	pt, idx := setupTemplate("file:///test.gohtml", `{{.User.Name}}`)
	bindings := makeBindings("Page", map[string]string{
		"User": "User",
	})
	diags := ComputeDiagnostics(pt, idx, bindings)

	for _, d := range diags {
		if strings.Contains(d.Message, "undefined field") {
			t.Errorf("should not warn about .User.Name when User exists, got: %s", d.Message)
		}
	}
}

func TestComputeDiagnostics_NestedChainWithResolvedType(t *testing.T) {
	// Build bindings with nested ChildType so .User.Email is fully validated
	userType := &goanalysis.TypeInfo{
		Name: "User",
		Fields: map[string]goanalysis.FieldInfo{
			"Name":  {Name: "Name", TypeName: "string"},
			"Email": {Name: "Email", TypeName: "string"},
		},
		Methods: make(map[string]goanalysis.MethodInfo),
	}
	bindings := []goanalysis.TemplateBinding{{
		TemplateName: "test.gohtml",
		ParsedFiles:  []string{"test.gohtml"},
		DataType: &goanalysis.TypeInfo{
			Name: "Page",
			Fields: map[string]goanalysis.FieldInfo{
				"User": {Name: "User", TypeName: "User", ChildType: userType},
			},
			Methods: make(map[string]goanalysis.MethodInfo),
		},
		FuncMaps: make(map[string]goanalysis.FuncSig),
	}}

	// Valid chain: .User.Email
	pt, idx := setupTemplate("file:///test.gohtml", `{{.User.Email}}`)
	diags := ComputeDiagnostics(pt, idx, bindings)
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined") {
			t.Errorf("should not warn about .User.Email, got: %s", d.Message)
		}
	}

	// Invalid chain: .User.Phone (Phone doesn't exist on User)
	pt2, idx2 := setupTemplate("file:///test.gohtml", `{{.User.Phone}}`)
	diags2 := ComputeDiagnostics(pt2, idx2, bindings)
	found := false
	for _, d := range diags2 {
		if strings.Contains(d.Message, "Phone") && strings.Contains(d.Message, "User") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about undefined field 'Phone' on User, got %v", diags2)
	}
}

func TestComputeDiagnostics_RangeRescope(t *testing.T) {
	// {{range .Items}}{{.Name}}{{end}} — .Name should validate against Item, not Page
	itemType := &goanalysis.TypeInfo{
		Name: "Item",
		Fields: map[string]goanalysis.FieldInfo{
			"Name":  {Name: "Name", TypeName: "string"},
			"Price": {Name: "Price", TypeName: "float64"},
		},
		Methods: make(map[string]goanalysis.MethodInfo),
	}
	bindings := []goanalysis.TemplateBinding{{
		TemplateName: "test.gohtml",
		ParsedFiles:  []string{"test.gohtml"},
		DataType: &goanalysis.TypeInfo{
			Name: "Page",
			Fields: map[string]goanalysis.FieldInfo{
				"Title": {Name: "Title", TypeName: "string"},
				"Items": {Name: "Items", TypeName: "[]Item", ChildType: itemType},
			},
			Methods: make(map[string]goanalysis.MethodInfo),
		},
		FuncMaps: make(map[string]goanalysis.FuncSig),
	}}

	// Valid: .Name exists on Item
	pt, idx := setupTemplate("file:///test.gohtml", `{{range .Items}}{{.Name}}{{end}}`)
	diags := ComputeDiagnostics(pt, idx, bindings)
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined") {
			t.Errorf("should not warn about .Name inside range .Items, got: %s", d.Message)
		}
	}

	// Invalid: .Title does NOT exist on Item (it's on Page)
	pt2, idx2 := setupTemplate("file:///test.gohtml", `{{range .Items}}{{.Title}}{{end}}`)
	diags2 := ComputeDiagnostics(pt2, idx2, bindings)
	found := false
	for _, d := range diags2 {
		if strings.Contains(d.Message, "Title") && strings.Contains(d.Message, "Item") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about undefined field 'Title' on Item inside range, got %v", diags2)
	}
}

func TestComputeDiagnostics_WithRescope(t *testing.T) {
	// {{with .User}}{{.Email}}{{end}} — .Email validates against User
	userType := &goanalysis.TypeInfo{
		Name: "User",
		Fields: map[string]goanalysis.FieldInfo{
			"Email": {Name: "Email", TypeName: "string"},
			"Name":  {Name: "Name", TypeName: "string"},
		},
		Methods: make(map[string]goanalysis.MethodInfo),
	}
	bindings := []goanalysis.TemplateBinding{{
		TemplateName: "test.gohtml",
		ParsedFiles:  []string{"test.gohtml"},
		DataType: &goanalysis.TypeInfo{
			Name: "Page",
			Fields: map[string]goanalysis.FieldInfo{
				"Title": {Name: "Title", TypeName: "string"},
				"User":  {Name: "User", TypeName: "User", ChildType: userType},
			},
			Methods: make(map[string]goanalysis.MethodInfo),
		},
		FuncMaps: make(map[string]goanalysis.FuncSig),
	}}

	// Valid: .Email exists on User
	pt, idx := setupTemplate("file:///test.gohtml", `{{with .User}}{{.Email}}{{end}}`)
	diags := ComputeDiagnostics(pt, idx, bindings)
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined") {
			t.Errorf("should not warn about .Email inside with .User, got: %s", d.Message)
		}
	}

	// Invalid: .Title doesn't exist on User
	pt2, idx2 := setupTemplate("file:///test.gohtml", `{{with .User}}{{.Title}}{{end}}`)
	diags2 := ComputeDiagnostics(pt2, idx2, bindings)
	found := false
	for _, d := range diags2 {
		if strings.Contains(d.Message, "Title") && strings.Contains(d.Message, "User") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about 'Title' on User inside with, got %v", diags2)
	}
}
