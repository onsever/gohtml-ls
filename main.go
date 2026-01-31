package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/onsever/gohtml-ls/features"
	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/jsonrpc"
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
	"github.com/onsever/gohtml-ls/workspace"
)

func main() {
	transport := jsonrpc.NewTransport(os.Stdin, os.Stdout)
	h := &handler{docs: lsp.NewDocumentStore()}
	server := lsp.NewServer(transport, h)
	h.server = server

	if err := server.Run(); err != nil {
		os.Exit(1)
	}
}

type handler struct {
	server       *lsp.Server
	docs         *lsp.DocumentStore
	ws           *workspace.Workspace
	bindings     []goanalysis.TemplateBinding
	initDone     chan struct{}
}

func (h *handler) Initialize(params lsp.InitializeParams) (lsp.InitializeResult, error) {
	h.ws = workspace.NewWorkspace(params.RootURI)
	h.initDone = make(chan struct{})
	go func() {
		h.ws.ScanTemplates()
		h.bindings = goanalysis.ScanDirectory(h.ws.RootPath)
		// Update workspace with custom function names and rescan
		h.ws.ExtraFuncs = h.customFuncNames()
		h.ws.RescanTemplates()
		// Register file watchers for .go files
		if h.server != nil {
			h.server.RegisterFileWatchers()
		}
		// Debug: log binding info
		if h.server != nil {
			h.server.LogMessage(3, fmt.Sprintf("[debug] root=%s bindings=%d", h.ws.RootPath, len(h.bindings)))
			for i, b := range h.bindings {
				dt := "<nil>"
				if b.DataType != nil {
					dt = b.DataType.Name
					if dt == "" {
						dt = fmt.Sprintf("{fields:%d}", len(b.DataType.Fields))
					}
				}
				h.server.LogMessage(3, fmt.Sprintf("[debug] binding[%d] tmpl=%q data=%s files=%v handler=%s gofile=%s",
					i, b.TemplateName, dt, b.ParsedFiles, b.HandlerName, b.GoFile))
			}
		}
		close(h.initDone)
		// Re-publish diagnostics for all open documents now that bindings are ready
		h.republishAllDiagnostics()
	}()
	return lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync:       1,
			HoverProvider:          true,
			DefinitionProvider:     true,
			CompletionProvider:     &lsp.CompletionOptions{TriggerCharacters: []string{".", "\""}},
			DocumentSymbolProvider: true,
			ReferencesProvider:     true,
			DiagnosticProvider:      &lsp.DiagnosticOptions{InterFileDependencies: true, WorkspaceDiagnostics: false},
			DocumentFormattingProvider: true,
			CodeActionProvider:      true,
			WorkspaceSymbolProvider: true,
			RenameProvider:          &lsp.RenameOptions{PrepareProvider: true},
			SemanticTokensProvider: &lsp.SemanticTokensOptions{
				Full: true,
				Legend: lsp.SemanticTokensLegend{
					TokenTypes:     features.SemanticTokenTypes(),
					TokenModifiers: features.SemanticTokenModifiers(),
				},
			},
		},
	}, nil
}

func (h *handler) DidOpen(params lsp.DidOpenTextDocumentParams) {
	uri := params.TextDocument.URI
	content := params.TextDocument.Text
	h.docs.Open(uri, params.TextDocument.Version, content)
	// If init goroutine hasn't finished, skip diagnostics now.
	// The goroutine will republish for all open docs when it completes.
	select {
	case <-h.initDone:
		h.updateAndPublish(uri, content)
	default:
	}
}

func (h *handler) DidChange(params lsp.DidChangeTextDocumentParams) {
	uri := params.TextDocument.URI
	if len(params.ContentChanges) > 0 {
		content := params.ContentChanges[len(params.ContentChanges)-1].Text
		h.docs.Change(uri, params.TextDocument.Version, content)
		h.updateAndPublish(uri, content)
	}
}

func (h *handler) DidClose(params lsp.DidCloseTextDocumentParams) {
	h.docs.Close(params.TextDocument.URI)
}

func (h *handler) DidSave(params lsp.DidSaveTextDocumentParams) {
	if workspace.IsGoFile(workspace.URIToPath(params.TextDocument.URI)) && h.ws != nil {
		h.refreshGoBindings()
	}
}

func (h *handler) DidChangeWatchedFiles(params lsp.DidChangeWatchedFilesParams) {
	if h.ws == nil {
		return
	}
	goChanged := false
	for _, change := range params.Changes {
		path := workspace.URIToPath(change.URI)
		if workspace.IsGoFile(path) {
			goChanged = true
		} else if workspace.IsTemplateFile(path) {
			// Re-index template files changed on disk (e.g. by rename) that aren't open in the editor
			if !h.docs.IsOpen(change.URI) {
				h.ws.ReindexFile(path)
			}
		}
	}
	if goChanged {
		h.refreshGoBindings()
	}
	// Republish diagnostics for all open docs so they pick up the updated index
	h.republishAllDiagnostics()
}

func (h *handler) republishAllDiagnostics() {
	if h.ws == nil || h.docs == nil {
		return
	}
	for _, doc := range h.docs.All() {
		h.updateAndPublish(doc.URI, doc.Content)
	}
}

func (h *handler) refreshGoBindings() {
	h.bindings = goanalysis.ScanDirectory(h.ws.RootPath)
	h.ws.ExtraFuncs = h.customFuncNames()
	h.ws.RescanTemplates()
	h.republishAllDiagnostics()
}

func (h *handler) Hover(params lsp.TextDocumentPositionParams) (*lsp.Hover, error) {
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return nil, nil
	}
	return features.Hover(pt, params.Position, h.ws.Index, h.bindings), nil
}

func (h *handler) Definition(params lsp.TextDocumentPositionParams) (*lsp.Location, error) {
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return nil, nil
	}
	return features.Definition(pt, params.Position, h.ws.Index, h.bindings), nil
}

func (h *handler) Completion(params lsp.TextDocumentPositionParams) (lsp.CompletionList, error) {
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return lsp.CompletionList{}, nil
	}
	return features.Completion(pt, params.Position, h.ws.Index, h.bindings), nil
}

func (h *handler) DocumentSymbol(params lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return nil, nil
	}
	return features.DocumentSymbols(pt), nil
}

func (h *handler) References(params lsp.ReferenceParams) ([]lsp.Location, error) {
	pt := h.getParsed(params.TextDocumentPositionParams.TextDocument.URI)
	if pt == nil {
		return nil, nil
	}
	return features.References(pt, params.Position, h.ws.Index, params.Context.IncludeDeclaration), nil
}

func (h *handler) SemanticTokensFull(params lsp.SemanticTokensParams) (lsp.SemanticTokens, error) {
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return lsp.SemanticTokens{}, nil
	}
	return features.SemanticTokensFull(pt, h.customFuncNames()...), nil
}

func (h *handler) updateAndPublish(uri, content string) {
	if h.ws == nil {
		return
	}
	pt := h.ws.UpdateDocument(uri, content, h.customFuncNames()...)
	diags := features.ComputeDiagnostics(pt, h.ws.Index, h.bindings)
	if h.server != nil {
		h.server.Notify("textDocument/publishDiagnostics", lsp.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diags,
		})
	}
}

// customFuncNames collects all custom function names from bindings.
func (h *handler) customFuncNames() []string {
	seen := make(map[string]bool)
	for _, b := range h.bindings {
		for name := range b.FuncMaps {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}

func (h *handler) Diagnostic(params lsp.DocumentDiagnosticParams) (lsp.DocumentDiagnosticReport, error) {
	// Wait for init to complete so we have ExtraFuncs for parsing
	if h.initDone != nil {
		<-h.initDone
	}
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return lsp.DocumentDiagnosticReport{Kind: "full"}, nil
	}
	diags := features.ComputeDiagnostics(pt, h.ws.Index, h.bindings)
	return lsp.DocumentDiagnosticReport{Kind: "full", Items: diags}, nil
}

func (h *handler) WorkspaceSymbol(params lsp.WorkspaceSymbolParams) ([]lsp.SymbolInformation, error) {
	if h.ws == nil || h.ws.Index == nil {
		return nil, nil
	}
	var symbols []lsp.SymbolInformation
	query := strings.ToLower(params.Query)
	for name, locs := range h.ws.Index.Definitions {
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			for _, loc := range locs {
				symbols = append(symbols, lsp.SymbolInformation{
					Name:     name,
					Kind:     lsp.SymbolKindFunction,
					Location: loc,
				})
			}
		}
	}
	return symbols, nil
}

func (h *handler) Formatting(params lsp.DocumentFormattingParams) ([]lsp.TextEdit, error) {
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return nil, nil
	}
	return features.Format(pt), nil
}

func (h *handler) CodeAction(params lsp.CodeActionParams) ([]lsp.CodeAction, error) {
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return nil, nil
	}
	return features.CodeActions(pt, params, h.bindings), nil
}

func (h *handler) Rename(params lsp.RenameParams) (*lsp.WorkspaceEdit, error) {
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return nil, nil
	}
	return features.Rename(pt, params.Position, params.NewName, h.ws.Index), nil
}

func (h *handler) PrepareRename(params lsp.PrepareRenameParams) (*lsp.Range, error) {
	pt := h.getParsed(params.TextDocument.URI)
	if pt == nil {
		return nil, nil
	}
	return features.PrepareRename(pt, params.Position, h.ws.Index), nil
}

func (h *handler) getParsed(uri string) *tmpl.ParsedTemplate {
	if h.ws == nil {
		return nil
	}
	return h.ws.GetParsed(uri)
}
