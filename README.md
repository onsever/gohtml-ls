# gohtml-lsp

A zero-dependency Language Server Protocol (LSP) implementation for Go `html/template` and `text/template` files. Provides IDE features for `.gohtml`, `.gotmpl`, `.tmpl`, and `.tpl` files.

Written entirely in Go using only the standard library.

## Features

- **Completions** — dot fields/methods, template names, keywords, built-in functions, custom FuncMap functions, snippets
- **Hover** — field types, function signatures, variable types, template definition locations
- **Go to Definition** — jump to `{{define}}` blocks, Go struct fields, and method declarations
- **Diagnostics** — syntax errors, undefined templates, undefined fields with full chain validation
- **Find References** — all template call sites and definitions
- **Rename** — rename template names across all files
- **Code Actions** — typo quick fixes for misspelled fields/methods
- **Formatting** — normalize whitespace in template actions
- **Semantic Highlighting** — keywords, variables, properties, functions, strings, comments, operators
- **HTML Support** — completions and hover for HTML tags/attributes (via VS Code extension)

All features are **scope-aware** — inside `{{range .Items}}`, completions and diagnostics use the element type. Inside `{{with .User}}`, they use the `User` type. Method return types are followed through chains like `.GetUser.Name`.

## How It Works

The LSP server scans your Go source files to find:

- `template.Execute(w, data)` / `template.ExecuteTemplate(w, name, data)` — resolves the data type
- `template.ParseFiles(...)` / `template.ParseGlob(...)` — links templates to files
- `.Funcs(template.FuncMap{...})` — extracts custom function names and signatures

This gives type-aware IDE features that understand your actual Go structs, not just template syntax.

## Building

```bash
go build -o gohtml-lsp .
```

## Running Tests

```bash
go test ./goanalysis/... ./features/... ./lsp/... ./jsonrpc/... ./template/... ./workspace/...
```

The `html/template` internal package error in test output is pre-existing and unrelated to this project.

## VS Code Extension

The `vscode-extension/` directory contains a VS Code extension that bundles the LSP binary and connects via stdio. It also provides:

- TextMate grammar for syntax highlighting
- Injection grammar for `.html` files
- 40+ code snippets
- HTML completions and hover (via `vscode-html-languageservice`)

### Building the Extension

```bash
# Build the LSP binary
go build -o gohtml-lsp .

# Copy binary into extension
cp gohtml-lsp vscode-extension/gohtml-lsp

# Bundle and package
cd vscode-extension
npm install
npm run esbuild
npx vsce package --no-dependencies

# Install
code --install-extension gohtml-lsp-0.1.0.vsix --force
```

## Architecture

```
VS Code  --stdio-->  gohtml-lsp binary
                        |
                        ├── jsonrpc.Transport (Content-Length framed messages)
                        ├── lsp.Server (request/notification dispatch)
                        └── handler
                              ├── workspace.Workspace (template files + index)
                              ├── goanalysis.ScanDirectory() (Go source analysis)
                              └── features.* (hover, completion, definition, diagnostics, etc.)
```

## Supported File Extensions

- `.gohtml`
- `.gotmpl`
- `.tmpl`
- `.tpl`

## License

MIT
