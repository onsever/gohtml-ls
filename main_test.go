package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/onsever/gohtml-ls/lsp"
	"github.com/onsever/gohtml-ls/workspace"
)

func TestInitializeCompletesInitialScanBeforeReturning(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module app\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 250; i++ {
		if err := os.WriteFile(filepath.Join(tmplDir, fmt.Sprintf("page%d.gohtml", i)), []byte(`{{define "page"}}x{{end}}`), 0644); err != nil {
			t.Fatal(err)
		}
	}

	h := &handler{docs: lsp.NewDocumentStore()}
	if _, err := h.Initialize(lsp.InitializeParams{RootURI: workspace.PathToURI(dir)}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-h.initDone:
	default:
		<-h.initDone
		t.Fatal("Initialize returned before the initial workspace scan completed")
	}
}
