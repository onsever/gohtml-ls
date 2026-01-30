# Go HTML Language Server

Full language support for Go `html/template` and `text/template` files in VS Code.

## Features

### Completions

- **Dot completions** — type `.` to see fields and methods of the bound Go struct
- **Nested chains** — `.User.` lists fields/methods of the `User` struct
- **Scope-aware** — inside `{{range .Items}}`, `.` completes with the element type's fields
- **Template names** — `{{template "` suggests all defined templates
- **Keywords** — `{{` triggers keyword completions (`if`, `range`, `with`, `define`, `block`, etc.)
- **Built-in functions** — `len`, `index`, `printf`, `eq`, etc. with signatures and docs
- **Custom FuncMap functions** — functions registered via `.Funcs()` in your Go code
- **Snippets** — 40+ snippets for common patterns (`if`/`end`, `range`/`end`, `with`/`end`, etc.)
- **HTML completions** — tags, attributes, and values outside `{{ }}` actions

### Hover

- **Fields** — shows type name and doc comment
- **Nested chains** — `.User.Name` shows the deepest field's type
- **Functions** — shows signature and documentation (builtins + custom FuncMap)
- **Variables** — `$v` shows the variable name and resolved type
- **Template references** — `{{template "x"}}` shows the definition location
- **Range variables** — `$i` resolves to `int`, `$v` resolves to element type
- **Method chains** — `.GetUser.Name` follows return types
- **HTML elements** — shows MDN documentation for HTML tags and attributes

### Go to Definition

- **Template references** — `{{template "x"}}` jumps to `{{define "x"}}`
- **Struct fields** — `{{.Field}}` jumps to the Go struct field declaration
- **Nested chains** — `{{.User.Name}}` jumps to the `Name` field on the `User` struct
- **Methods** — jumps to the method declaration in Go source
- **Scope-aware** — works correctly inside `{{range}}` and `{{with}}` blocks

### Diagnostics

- **Syntax errors** — real-time parse error reporting
- **Undefined templates** — `{{template "x"}}` where `x` has no `{{define "x"}}`
- **Undefined fields** — `{{.Foo}}` where `Foo` doesn't exist on the bound struct
- **Chain validation** — `{{.User.Phone}}` reports `Phone` undefined on `User`
- **Scope-aware** — validates fields inside `{{range}}` and `{{with}}` against the correct type

### Find References

- All `{{template "x"}}` call sites and the `{{define "x"}}` location

### Rename

- `F2` on template names renames across all files (`{{define}}`, `{{template}}`, `{{block}}`)

### Code Actions

- **Typo quick fixes** — "Did you mean `.Title`?" for misspelled field/method names

### Formatting

- Normalizes whitespace inside `{{ }}` actions
- Preserves trim markers (`{{-` and `-}}`)
- Does not touch HTML content outside actions

### Syntax Highlighting

- Full TextMate grammar for Go templates in HTML
- Semantic token highlighting for keywords, variables, properties, functions, strings, comments, and operators
- Injection grammar for Go template syntax in `.html` files

## Supported File Extensions

| Extension | Language ID |
|-----------|-------------|
| `.gohtml`  | `gohtml` |
| `.gotmpl`  | `gohtml` |
| `.tmpl`    | `gohtml` |
| `.tpl`     | `gohtml` |

## How It Works

The extension bundles a **zero-dependency LSP server** written in Go. It analyzes your Go source files to understand:

- Which structs are passed to `template.Execute()` / `template.ExecuteTemplate()`
- Which files are parsed via `template.ParseFiles()` / `template.ParseGlob()`
- Which custom functions are registered via `.Funcs(template.FuncMap{...})`

This gives you type-aware completions, hover, diagnostics, and go-to-definition that understands your actual Go types — not just the template syntax.

HTML language features (completions, hover) are provided client-side via `vscode-html-languageservice` for content outside template actions.

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `gohtml-lsp.serverPath` | `""` | Path to the `gohtml-lsp` binary. If empty, uses the bundled binary or `gohtml-lsp` from PATH. |
| `gohtml-lsp.trace.server` | `"off"` | Trace communication between VS Code and the language server (`off`, `messages`, `verbose`). |

## Requirements

- Go project with `html/template` or `text/template` usage
- Go source files in the workspace root (the LSP scans `.go` files for template bindings)

## License

MIT
