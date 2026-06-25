package goanalysis

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSupportMatrixProject(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	writeGoMod(t, dir, "example.com/support")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func findBinding(t *testing.T, bindings []TemplateBinding, templateName, typeName string) TemplateBinding {
	t.Helper()
	for _, b := range bindings {
		if b.TemplateName == templateName && b.DataType != nil && b.DataType.Name == typeName {
			return b
		}
	}
	t.Fatalf("expected binding template=%q data=%q, got %#v", templateName, typeName, bindings)
	return TemplateBinding{}
}

func TestSupportMatrix_ConstTemplateNameAndConstructedPath(t *testing.T) {
	dir := writeSupportMatrixProject(t, `package main

import (
	"html/template"
	"net/http"
	"path/filepath"
)

const pageName = "home"
const fileName = "home.gohtml"

type PageData struct {
	Title string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles(filepath.Join("templates", fileName)))
	tpl.ExecuteTemplate(w, pageName, PageData{Title: "Home"})
}
`)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}
	b := findBinding(t, bindings, "home", "PageData")
	if len(b.ParsedFiles) != 1 || b.ParsedFiles[0] != "templates/home.gohtml" {
		t.Fatalf("expected constructed parsed file path, got %#v", b.ParsedFiles)
	}
}

func TestSupportMatrix_FunctionReturnAndJSONDecodeData(t *testing.T) {
	dir := writeSupportMatrixProject(t, `package main

import (
	"encoding/json"
	"html/template"
	"net/http"
)

type PageData struct {
	Title string
}

func NewPageData() PageData {
	return PageData{Title: "Home"}
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("home.gohtml", "json.gohtml"))
	tpl.ExecuteTemplate(w, "home", NewPageData())

	var decoded PageData
	_ = json.NewDecoder(r.Body).Decode(&decoded)
	tpl.ExecuteTemplate(w, "json", decoded)
}
`)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}
	findBinding(t, bindings, "home", "PageData")
	findBinding(t, bindings, "json", "PageData")
}

func TestSupportMatrix_StaticMapLiteralData(t *testing.T) {
	dir := writeSupportMatrixProject(t, `package main

import (
	"html/template"
	"net/http"
)

type User struct {
	Name string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("home.gohtml"))
	user := User{Name: "Ada"}
	tpl.Execute(w, map[string]any{
		"Title": "Home",
		"User": user,
	})
}
`)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}
	b := findBinding(t, bindings, "home.gohtml", "map")
	if _, ok := b.DataType.Fields["Title"]; !ok {
		t.Fatalf("expected map field Title, got %#v", b.DataType.Fields)
	}
	userField, ok := b.DataType.Fields["User"]
	if !ok {
		t.Fatalf("expected map field User, got %#v", b.DataType.Fields)
	}
	if userField.ChildType == nil || userField.ChildType.Name != "User" {
		t.Fatalf("expected User child type, got %#v", userField.ChildType)
	}
}

func TestSupportMatrix_StaticSelectorTemplateName(t *testing.T) {
	dir := writeSupportMatrixProject(t, `package main

import (
	"html/template"
	"net/http"
)

type Page struct {
	Name string
}

type PageData struct {
	Title string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("home.gohtml"))
	page := Page{Name: "home"}
	tpl.ExecuteTemplate(w, page.Name, PageData{Title: "Home"})
}
`)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}
	findBinding(t, bindings, "home", "PageData")
}

func TestSupportMatrix_RenderHelperWithAnyAndGenericData(t *testing.T) {
	dir := writeSupportMatrixProject(t, `package main

import (
	"html/template"
	"net/http"
)

var templates = template.Must(template.ParseFiles("home.gohtml", "profile.gohtml"))

type HomeData struct {
	Title string
}

type ProfileData struct {
	Name string
}

func Render(w http.ResponseWriter, name string, data any) error {
	return templates.ExecuteTemplate(w, name, data)
}

func RenderGeneric[T any](w http.ResponseWriter, name string, data T) error {
	return templates.ExecuteTemplate(w, name, data)
}

func handler(w http.ResponseWriter, r *http.Request) {
	_ = Render(w, "home", HomeData{Title: "Home"})
	_ = RenderGeneric(w, "profile", ProfileData{Name: "Ada"})
}
`)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}
	findBinding(t, bindings, "home", "HomeData")
	findBinding(t, bindings, "profile", "ProfileData")
}

func TestSupportMatrix_ParseFSWrapperStaticPattern(t *testing.T) {
	dir := writeSupportMatrixProject(t, `package main

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*
var templateFS embed.FS

type PageData struct {
	Title string
}

func Load(pattern string) *template.Template {
	return template.Must(template.ParseFS(templateFS, pattern))
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := Load("templates/home.gohtml")
	tpl.Execute(w, PageData{Title: "Home"})
}
`)
	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "home.gohtml"), []byte(`{{.Title}}`), 0644); err != nil {
		t.Fatal(err)
	}

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}
	b := findBinding(t, bindings, "home.gohtml", "PageData")
	if len(b.ParsedFiles) != 1 || b.ParsedFiles[0] != "templates/home.gohtml" {
		t.Fatalf("expected ParseFS wrapper parsed file, got %#v", b.ParsedFiles)
	}
}
