package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	tmpl "github.com/onsever/gohtml-ls/template"
)

// Workspace manages the project model and template index.
type Workspace struct {
	RootURI    string
	RootPath   string
	Index      *tmpl.TemplateIndex
	ExtraFuncs []string // custom function names from Go FuncMap bindings
	mu         sync.Mutex
}

// NewWorkspace creates a workspace from the root URI.
func NewWorkspace(rootURI string) *Workspace {
	rootPath := URIToPath(rootURI)
	return &Workspace{
		RootURI:  rootURI,
		RootPath: rootPath,
		Index:    tmpl.NewTemplateIndex(),
	}
}

// ScanTemplates walks the workspace for template files and indexes them.
func (w *Workspace) ScanTemplates() {
	w.mu.Lock()
	defer w.mu.Unlock()

	filepath.Walk(w.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Skip hidden dirs and vendor
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if IsTemplateFile(path) {
			w.indexFile(path)
		}
		return nil
	})
}

// UpdateDocument re-parses and re-indexes a document.
// extraFuncs are additional function names (from FuncMap) to register with the parser.
func (w *Workspace) UpdateDocument(uri, content string, extraFuncs ...string) *tmpl.ParsedTemplate {
	w.mu.Lock()
	defer w.mu.Unlock()

	pt := tmpl.Parse(uri, content, extraFuncs...)
	w.Index.Index(pt)
	return pt
}

// GetParsed returns the parsed template for a URI.
func (w *Workspace) GetParsed(uri string) *tmpl.ParsedTemplate {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Index.Trees[uri]
}

// ReindexFile re-reads a template file from disk and re-indexes it.
func (w *Workspace) ReindexFile(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.indexFile(path)
}

func (w *Workspace) indexFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	uri := PathToURI(path)
	pt := tmpl.Parse(uri, string(data), w.ExtraFuncs...)
	w.Index.Index(pt)
}

// RescanTemplates re-scans all template files (e.g., after bindings update with new FuncMap names).
func (w *Workspace) RescanTemplates() {
	w.ScanTemplates()
}
