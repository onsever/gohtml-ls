package features

import "github.com/onsever/gohtml-ls/lsp"

func snippetCompletions() []lsp.CompletionItem {
	return []lsp.CompletionItem{
		{
			Label:            "if",
			Kind:             lsp.CompletionItemKindSnippet,
			Detail:           "if block",
			InsertText:       "{{if ${1:cond}}}$0{{end}}",
			InsertTextFormat: lsp.InsertTextFormatSnippet,
		},
		{
			Label:            "range",
			Kind:             lsp.CompletionItemKindSnippet,
			Detail:           "range block",
			InsertText:       "{{range ${1:collection}}}$0{{end}}",
			InsertTextFormat: lsp.InsertTextFormatSnippet,
		},
		{
			Label:            "define",
			Kind:             lsp.CompletionItemKindSnippet,
			Detail:           "define block",
			InsertText:       `{{define "${1:name}"}}$0{{end}}`,
			InsertTextFormat: lsp.InsertTextFormatSnippet,
		},
		{
			Label:            "block",
			Kind:             lsp.CompletionItemKindSnippet,
			Detail:           "block",
			InsertText:       `{{block "${1:name}" ${2:.}}}$0{{end}}`,
			InsertTextFormat: lsp.InsertTextFormatSnippet,
		},
		{
			Label:            "with",
			Kind:             lsp.CompletionItemKindSnippet,
			Detail:           "with block",
			InsertText:       "{{with ${1:value}}}$0{{end}}",
			InsertTextFormat: lsp.InsertTextFormatSnippet,
		},
		{
			Label:            "template",
			Kind:             lsp.CompletionItemKindSnippet,
			Detail:           "template call",
			InsertText:       `{{template "${1:name}" ${2:.}}}`,
			InsertTextFormat: lsp.InsertTextFormatSnippet,
		},
	}
}
