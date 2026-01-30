package lsp

import (
	"encoding/json"
	"log"
	"os"

	"github.com/onsever/gohtml-ls/jsonrpc"
)

// Server is the LSP server.
type Server struct {
	transport   *jsonrpc.Transport
	docs        *DocumentStore
	handler     Handler
	initialized bool
	shutdown    bool
	logger      *log.Logger
	nextID      int
}

// Handler interface for routing LSP methods.
type Handler interface {
	Initialize(params InitializeParams) (InitializeResult, error)
	DidOpen(params DidOpenTextDocumentParams)
	DidChange(params DidChangeTextDocumentParams)
	DidClose(params DidCloseTextDocumentParams)
	DidSave(params DidSaveTextDocumentParams)
	DidChangeWatchedFiles(params DidChangeWatchedFilesParams)
	Hover(params TextDocumentPositionParams) (*Hover, error)
	Definition(params TextDocumentPositionParams) (*Location, error)
	Completion(params TextDocumentPositionParams) (CompletionList, error)
	DocumentSymbol(params DocumentSymbolParams) ([]DocumentSymbol, error)
	References(params ReferenceParams) ([]Location, error)
	SemanticTokensFull(params SemanticTokensParams) (SemanticTokens, error)
	Diagnostic(params DocumentDiagnosticParams) (DocumentDiagnosticReport, error)
	WorkspaceSymbol(params WorkspaceSymbolParams) ([]SymbolInformation, error)
	Rename(params RenameParams) (*WorkspaceEdit, error)
	PrepareRename(params PrepareRenameParams) (*Range, error)
	CodeAction(params CodeActionParams) ([]CodeAction, error)
	Formatting(params DocumentFormattingParams) ([]TextEdit, error)
}

// DocumentSymbolParams for textDocument/documentSymbol.
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// SemanticTokensParams for textDocument/semanticTokens/full.
type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

func NewServer(transport *jsonrpc.Transport, handler Handler) *Server {
	return &Server{
		transport: transport,
		docs:      NewDocumentStore(),
		handler:   handler,
		logger:    log.New(os.Stderr, "[gohtml-lsp] ", log.LstdFlags),
	}
}

// Run starts the main message loop.
func (s *Server) Run() error {
	for {
		msg, err := s.transport.ReadMessage()
		if err != nil {
			if s.shutdown {
				return nil
			}
			return err
		}
		s.handleMessage(msg)
	}
}

func (s *Server) handleMessage(msg json.RawMessage) {
	var req jsonrpc.Request
	if err := json.Unmarshal(msg, &req); err != nil {
		s.logger.Printf("failed to parse message: %v", err)
		return
	}

	if req.Method == "" {
		return // response, ignore
	}

	// Notification (no ID)
	if req.ID == nil || string(req.ID) == "null" {
		s.handleNotification(req.Method, req.Params)
		return
	}

	// Request (has ID)
	s.handleRequest(req)
}

func (s *Server) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "initialized":
		// no-op
	case "textDocument/didOpen":
		var p DidOpenTextDocumentParams
		if json.Unmarshal(params, &p) == nil {
			s.handler.DidOpen(p)
		}
	case "textDocument/didChange":
		var p DidChangeTextDocumentParams
		if json.Unmarshal(params, &p) == nil {
			s.handler.DidChange(p)
		}
	case "textDocument/didClose":
		var p DidCloseTextDocumentParams
		if json.Unmarshal(params, &p) == nil {
			s.handler.DidClose(p)
		}
	case "textDocument/didSave":
		var p DidSaveTextDocumentParams
		if json.Unmarshal(params, &p) == nil {
			s.handler.DidSave(p)
		}
	case "workspace/didChangeWatchedFiles":
		var p DidChangeWatchedFilesParams
		if json.Unmarshal(params, &p) == nil {
			s.handler.DidChangeWatchedFiles(p)
		}
	case "exit":
		if s.shutdown {
			os.Exit(0)
		}
		os.Exit(1)
	}
}

func (s *Server) handleRequest(req jsonrpc.Request) {
	var result interface{}
	var err error

	switch req.Method {
	case "initialize":
		var p InitializeParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		s.initialized = true
		result, err = s.handler.Initialize(p)

	case "shutdown":
		s.shutdown = true
		result = nil

	case "textDocument/hover":
		var p TextDocumentPositionParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.Hover(p)

	case "textDocument/definition":
		var p TextDocumentPositionParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.Definition(p)

	case "textDocument/completion":
		var p TextDocumentPositionParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.Completion(p)

	case "textDocument/documentSymbol":
		var p DocumentSymbolParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.DocumentSymbol(p)

	case "textDocument/references":
		var p ReferenceParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.References(p)

	case "textDocument/semanticTokens/full":
		var p SemanticTokensParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.SemanticTokensFull(p)

	case "textDocument/diagnostic":
		var p DocumentDiagnosticParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.Diagnostic(p)

	case "workspace/symbol":
		var p WorkspaceSymbolParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.WorkspaceSymbol(p)

	case "textDocument/formatting":
		var p DocumentFormattingParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.Formatting(p)

	case "textDocument/codeAction":
		var p CodeActionParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.CodeAction(p)

	case "textDocument/rename":
		var p RenameParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.Rename(p)

	case "textDocument/prepareRename":
		var p PrepareRenameParams
		if e := json.Unmarshal(req.Params, &p); e != nil {
			s.sendError(req.ID, jsonrpc.InvalidParams, e.Error())
			return
		}
		result, err = s.handler.PrepareRename(p)

	default:
		s.sendError(req.ID, jsonrpc.MethodNotFound, "method not found: "+req.Method)
		return
	}

	if err != nil {
		s.sendError(req.ID, jsonrpc.InternalError, err.Error())
		return
	}

	s.sendResult(req.ID, result)
}

func (s *Server) sendResult(id json.RawMessage, result interface{}) {
	resp, err := jsonrpc.NewResponse(id, result)
	if err != nil {
		s.logger.Printf("failed to marshal response: %v", err)
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		s.logger.Printf("failed to marshal response: %v", err)
		return
	}
	if err := s.transport.WriteMessage(data); err != nil {
		s.logger.Printf("failed to write response: %v", err)
	}
}

func (s *Server) sendError(id json.RawMessage, code int, message string) {
	resp := jsonrpc.NewErrorResponse(id, code, message)
	data, err := json.Marshal(resp)
	if err != nil {
		s.logger.Printf("failed to marshal error response: %v", err)
		return
	}
	if err := s.transport.WriteMessage(data); err != nil {
		s.logger.Printf("failed to write error response: %v", err)
	}
}

// OpenDocuments returns all currently open documents.
func (s *Server) OpenDocuments() []*Document {
	return s.docs.All()
}

// RegisterFileWatchers sends a client/registerCapability request to watch Go files.
func (s *Server) RegisterFileWatchers() {
	s.nextID++
	params := map[string]interface{}{
		"registrations": []map[string]interface{}{
			{
				"id":     "go-file-watcher",
				"method": "workspace/didChangeWatchedFiles",
				"registerOptions": map[string]interface{}{
					"watchers": []map[string]interface{}{
						{"globPattern": "**/*.go", "kind": 7},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(params)
	idRaw, _ := json.Marshal(s.nextID)
	req := jsonrpc.Request{
		ID:     idRaw,
		Method: "client/registerCapability",
		Params: raw,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return
	}
	_ = s.transport.WriteMessage(data)
}

// Notify sends a notification to the client.
func (s *Server) Notify(method string, params interface{}) {
	notif, err := jsonrpc.NewNotification(method, params)
	if err != nil {
		s.logger.Printf("failed to marshal notification: %v", err)
		return
	}
	data, err := json.Marshal(notif)
	if err != nil {
		s.logger.Printf("failed to marshal notification: %v", err)
		return
	}
	if err := s.transport.WriteMessage(data); err != nil {
		s.logger.Printf("failed to write notification: %v", err)
	}
}
