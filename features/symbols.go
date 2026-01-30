package features

import (
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

// DocumentSymbols returns symbols for a template document.
func DocumentSymbols(pt *tmpl.ParsedTemplate) []lsp.DocumentSymbol {
	return tmpl.ExtractSymbols(pt)
}
