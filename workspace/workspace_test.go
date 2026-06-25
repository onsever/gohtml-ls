package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWorkspace(t *testing.T) {
	ws := NewWorkspace("file:///tmp/project")
	if ws.Index == nil {
		t.Fatal("expected non-nil Index")
	}
}

func TestUpdateDocument_Valid(t *testing.T) {
	ws := NewWorkspace("file:///tmp/project")
	ws.UpdateDocument("file:///tmp/project/page.gohtml", "<h1>{{.Title}}</h1>")
	pt := ws.GetParsed("file:///tmp/project/page.gohtml")
	if pt == nil {
		t.Fatal("expected non-nil ParsedTemplate")
	}
}

func TestUpdateDocument_Invalid(t *testing.T) {
	ws := NewWorkspace("file:///tmp/project")
	pt := ws.UpdateDocument("file:///tmp/project/bad.gohtml", "{{if}}")
	if len(pt.Errors) == 0 {
		t.Fatal("expected parse errors for invalid template")
	}
}

func TestScanTemplates(t *testing.T) {
	dir := t.TempDir()
	content := []byte("<p>{{.Name}}</p>")
	tmplPath := filepath.Join(dir, "test.gohtml")
	if err := os.WriteFile(tmplPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	ws := NewWorkspace(PathToURI(dir))
	ws.ScanTemplates()

	uri := PathToURI(tmplPath)
	if _, ok := ws.Index.Trees[uri]; !ok {
		t.Errorf("expected template %q to be indexed, found keys: %v", uri, keys(ws.Index.Trees))
	}
}

func TestReindexFile_RemovesMissingTemplate(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "gone.gohtml")
	if err := os.WriteFile(tmplPath, []byte(`{{define "gone"}}x{{end}}`), 0644); err != nil {
		t.Fatal(err)
	}

	ws := NewWorkspace(PathToURI(dir))
	ws.ScanTemplates()

	uri := PathToURI(tmplPath)
	if _, ok := ws.Index.Trees[uri]; !ok {
		t.Fatalf("expected %q to be indexed before delete", uri)
	}
	if len(ws.Index.Definitions["gone"]) == 0 {
		t.Fatal("expected definition before delete")
	}

	if err := os.Remove(tmplPath); err != nil {
		t.Fatal(err)
	}
	ws.ReindexFile(tmplPath)

	if _, ok := ws.Index.Trees[uri]; ok {
		t.Fatalf("expected %q to be removed from tree index", uri)
	}
	if len(ws.Index.Definitions["gone"]) != 0 {
		t.Fatalf("expected definition to be removed, got %#v", ws.Index.Definitions["gone"])
	}
}

type keyer interface{ ~string }

func keys[K keyer, V any](m map[K]V) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}
