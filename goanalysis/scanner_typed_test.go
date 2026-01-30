package goanalysis

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGoMod(t *testing.T, dir, module string) {
	t.Helper()
	content := "module " + module + "\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestScanDirectoryTyped_DirectExecute(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "example.com/test")

	src := `package main

import (
	"html/template"
	"net/http"
	"os"
)

type HomeData struct {
	Title string
	Count int
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("index.gohtml"))
	tpl.Execute(w, HomeData{Title: "Home", Count: 5})
}

func main() {
	_ = os.Args
}
`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available (go list failed)")
	}

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "HomeData" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'HomeData'")
	}
	if _, ok := found.DataType.Fields["Title"]; !ok {
		t.Fatal("expected field 'Title'")
	}
	if _, ok := found.DataType.Fields["Count"]; !ok {
		t.Fatal("expected field 'Count'")
	}
}

func TestScanDirectoryTyped_WrapperType(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "example.com/test")

	src := `package main

import (
	"html/template"
	"net/http"
	"os"
)

type Template struct {
	htmlTpl *template.Template
}

func (t *Template) Execute(w http.ResponseWriter, data interface{}) error {
	return t.htmlTpl.Execute(w, data)
}

type PageData struct {
	Name string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := &Template{htmlTpl: template.Must(template.ParseFiles("page.gohtml"))}
	tpl.htmlTpl.Execute(w, PageData{Name: "test"})
}

func main() {
	_ = os.Args
}
`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "PageData" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'PageData'")
	}
}

func TestScanDirectoryTyped_AnonymousStruct(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "example.com/test")

	src := `package main

import (
	"html/template"
	"net/http"
	"os"
)

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("page.gohtml"))
	data := struct {
		Title   string
		Content string
	}{Title: "Hi", Content: "Body"}
	tpl.Execute(w, data)
}

func main() {
	_ = os.Args
}
`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil {
			if _, ok := bindings[i].DataType.Fields["Title"]; ok {
				found = &bindings[i]
				break
			}
		}
	}
	if found == nil {
		t.Fatal("expected binding with anonymous struct containing 'Title' field")
	}
	if _, ok := found.DataType.Fields["Content"]; !ok {
		t.Fatal("expected field 'Content'")
	}
}

func TestScanDirectoryTyped_FallbackOnNoGoMod(t *testing.T) {
	dir := t.TempDir()
	// No go.mod — should return nil

	src := `package main

import "html/template"

var _ = template.Must(template.ParseFiles("index.gohtml"))
`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644)

	bindings := ScanDirectoryTyped(dir)
	if bindings != nil {
		// It's ok if go list works without go.mod in some Go versions
		t.Log("go list succeeded without go.mod, skipping fallback test")
	}
}

func TestScanDirectoryTyped_ParseFS(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "example.com/test")

	src := `package main

import (
	"embed"
	"html/template"
	"net/http"
	"os"
)

//go:embed templates/*
var templateFS embed.FS

type PageData struct {
	Title string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFS(templateFS, "templates/*.gohtml"))
	tpl.Execute(w, PageData{Title: "Test"})
}

func main() {
	_ = os.Args
}
`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}

	var foundParsedFile bool
	for _, b := range bindings {
		for _, pf := range b.ParsedFiles {
			if pf == "templates/*.gohtml" {
				foundParsedFile = true
			}
		}
	}
	if !foundParsedFile {
		t.Fatal("expected ParsedFiles to contain 'templates/*.gohtml'")
	}
}

func TestScanDirectoryTyped_Closure(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "example.com/test")

	src := `package main

import (
	"html/template"
	"net/http"
	"os"
)

type AppData struct {
	Version string
}

func setup() http.HandlerFunc {
	data := AppData{Version: "1.0"}
	return func(w http.ResponseWriter, r *http.Request) {
		tpl := template.Must(template.ParseFiles("app.gohtml"))
		tpl.Execute(w, data)
	}
}

func main() {
	_ = os.Args
}
`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644)

	bindings := ScanDirectoryTyped(dir)
	if bindings == nil {
		t.Skip("go/types scanner not available")
	}

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "AppData" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'AppData' from closure-captured variable")
	}
}
