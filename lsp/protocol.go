package lsp

import "encoding/json"

// ---------------------------------------------------------------------------
// Base types
// ---------------------------------------------------------------------------

type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// ---------------------------------------------------------------------------
// Initialize
// ---------------------------------------------------------------------------

type InitializeParams struct {
	RootURI      string          `json:"rootUri"`
	Capabilities json.RawMessage `json:"capabilities"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	TextDocumentSync       int                    `json:"textDocumentSync"`
	HoverProvider          bool                   `json:"hoverProvider"`
	DefinitionProvider     bool                   `json:"definitionProvider"`
	CompletionProvider     *CompletionOptions     `json:"completionProvider,omitempty"`
	DocumentSymbolProvider bool                   `json:"documentSymbolProvider"`
	ReferencesProvider     bool                   `json:"referencesProvider"`
	SemanticTokensProvider *SemanticTokensOptions `json:"semanticTokensProvider,omitempty"`
	DiagnosticProvider     *DiagnosticOptions     `json:"diagnosticProvider,omitempty"`
	WorkspaceSymbolProvider bool                  `json:"workspaceSymbolProvider,omitempty"`
	DocumentFormattingProvider bool               `json:"documentFormattingProvider,omitempty"`
	CodeActionProvider      bool                  `json:"codeActionProvider,omitempty"`
	RenameProvider          *RenameOptions        `json:"renameProvider,omitempty"`
}

type RenameOptions struct {
	PrepareProvider bool `json:"prepareProvider"`
}

type DiagnosticOptions struct {
	InterFileDependencies bool `json:"interFileDependencies"`
	WorkspaceDiagnostics  bool `json:"workspaceDiagnostics"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type SemanticTokensOptions struct {
	Full   bool                 `json:"full"`
	Legend SemanticTokensLegend `json:"legend"`
}

type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

// ---------------------------------------------------------------------------
// Document synchronization
// ---------------------------------------------------------------------------

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// ---------------------------------------------------------------------------
// Diagnostics
// ---------------------------------------------------------------------------

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

const (
	DiagnosticSeverityError   = 1
	DiagnosticSeverityWarning = 2
	DiagnosticSeverityInfo    = 3
	DiagnosticSeverityHint    = 4
)

// ---------------------------------------------------------------------------
// Hover
// ---------------------------------------------------------------------------

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// ---------------------------------------------------------------------------
// Completion
// ---------------------------------------------------------------------------

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label            string    `json:"label"`
	Detail           string    `json:"detail,omitempty"`
	Documentation    string    `json:"documentation,omitempty"`
	Kind             int       `json:"kind"`
	InsertText       string    `json:"insertText,omitempty"`
	InsertTextFormat int       `json:"insertTextFormat,omitempty"`
	TextEdit         *TextEdit `json:"textEdit,omitempty"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

const (
	CompletionItemKindText     = 1
	CompletionItemKindMethod   = 2
	CompletionItemKindFunction = 3
	CompletionItemKindField    = 5
	CompletionItemKindVariable = 6
	CompletionItemKindProperty = 10
	CompletionItemKindKeyword  = 14
	CompletionItemKindSnippet  = 15
)

const (
	InsertTextFormatPlainText = 1
	InsertTextFormatSnippet   = 2
)

// ---------------------------------------------------------------------------
// Document symbols
// ---------------------------------------------------------------------------

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

const (
	SymbolKindNamespace = 3
	SymbolKindFunction  = 12
	SymbolKindString    = 15
)

// ---------------------------------------------------------------------------
// References
// ---------------------------------------------------------------------------

type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ---------------------------------------------------------------------------
// Semantic tokens
// ---------------------------------------------------------------------------

type SemanticTokens struct {
	Data []uint32 `json:"data"`
}

// ---------------------------------------------------------------------------
// Pull diagnostics
// ---------------------------------------------------------------------------

type DocumentDiagnosticParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DocumentDiagnosticReport struct {
	Kind  string       `json:"kind"`
	Items []Diagnostic `json:"items"`
}

// ---------------------------------------------------------------------------
// Workspace symbol
// ---------------------------------------------------------------------------

type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

type SymbolInformation struct {
	Name     string   `json:"name"`
	Kind     int      `json:"kind"`
	Location Location `json:"location"`
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

type FormattingOptions struct {
	TabSize                uint32 `json:"tabSize"`
	InsertSpaces           bool   `json:"insertSpaces"`
	TrimTrailingWhitespace bool   `json:"trimTrailingWhitespace,omitempty"`
}

// ---------------------------------------------------------------------------
// Code actions
// ---------------------------------------------------------------------------

type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
}

// ---------------------------------------------------------------------------
// Rename
// ---------------------------------------------------------------------------

type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}

type PrepareRenameParams struct {
	TextDocumentPositionParams
}

type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
}

// ---------------------------------------------------------------------------
// File watching
// ---------------------------------------------------------------------------

type DidChangeWatchedFilesParams struct {
	Changes []FileEvent `json:"changes"`
}

type FileEvent struct {
	URI  string `json:"uri"`
	Type int    `json:"type"`
}
