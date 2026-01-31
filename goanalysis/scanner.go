package goanalysis

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TemplateBinding links a template name to its Go data type and functions.
type TemplateBinding struct {
	TemplateName string
	DataType     *TypeInfo
	FuncMaps     map[string]FuncSig
	ParsedFiles  []string
	GoFile       string
	HandlerName  string // Name of the Go function containing the Execute call
}

// ScanDirectory scans .go files in a directory for template usage.
// It first tries the go/types-based scanner for accurate type resolution,
// falling back to the AST-only scanner if type-checking fails.
func ScanDirectory(dir string) []TemplateBinding {
	if bindings := ScanDirectoryTyped(dir); bindings != nil {
		fmt.Fprintf(os.Stderr, "[gohtml-lsp] ScanDirectory: typed scanner returned %d bindings\n", len(bindings))
		return bindings
	}
	fmt.Fprintf(os.Stderr, "[gohtml-lsp] ScanDirectory: typed scanner failed, falling back to AST scanner\n")
	// Log why typed scanner failed
	if lr := LoadDirectory(dir); lr == nil {
		// Try go list directly to capture error
		cmd := exec.Command("go", "list", "-json", "./...")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "[gohtml-lsp] go list error: %v\noutput: %s\n", err, string(out[:min(len(out), 500)]))
		} else {
			fmt.Fprintf(os.Stderr, "[gohtml-lsp] go list succeeded but LoadDirectory returned nil\n")
		}
	}
	return scanDirectoryAST(dir)
}

// scanDirectoryAST is the original AST-only scanner.
func scanDirectoryAST(dir string) []TemplateBinding {
	// First pass: collect all structs and methods across the directory
	allStructs := make(map[string][]FieldInfo)
	allMethods := make(map[string][]MethodInfo)
	var goFiles []string

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		goFiles = append(goFiles, path)
		return nil
	})

	// Collect structs and methods from all Go files
	for _, path := range goFiles {
		fset := token.NewFileSet()
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			continue
		}
		for name, fields := range collectStructs(file, fset, path) {
			allStructs[name] = fields
		}
		for name, methods := range CollectMethods(file, fset) {
			allMethods[name] = append(allMethods[name], methods...)
		}
	}

	// Second pass: scan for template usage with full struct+method knowledge
	var bindings []TemplateBinding
	for _, path := range goFiles {
		fileBindings := scanFileWithContext(path, allStructs, allMethods)
		bindings = append(bindings, fileBindings...)
	}
	return bindings
}

// ScanFile scans a single .go file for template usage patterns.
// For cross-file struct resolution, use ScanDirectory instead.
func ScanFile(path string) []TemplateBinding {
	fset := token.NewFileSet()
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil
	}
	structs := collectStructs(file, fset, path)
	methods := CollectMethods(file, fset)
	return scanFileAST(file, path, structs, methods)
}

// scanFileWithContext scans a file for template usage, using cross-file struct+method info.
func scanFileWithContext(path string, allStructs map[string][]FieldInfo, allMethods map[string][]MethodInfo) []TemplateBinding {
	fset := token.NewFileSet()
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil
	}
	return scanFileAST(file, path, allStructs, allMethods)
}

// scanFileAST is the core scanner that works on a parsed AST.
func scanFileAST(file *ast.File, path string, structs map[string][]FieldInfo, methods map[string][]MethodInfo) []TemplateBinding {
	var bindings []TemplateBinding

	type executeCall struct {
		templateName string
		dataExpr     ast.Expr
		callPos      token.Pos
	}
	var execCalls []executeCall
	var parsedFiles []string
	funcMaps := make(map[string]FuncSig)

	// Collect variable declarations for resolving variable data args
	// e.g., data := HomeData{...}; tpl.Execute(w, data)
	varTypes := collectVarTypes(file)

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Handle template.Must(expr) — unwrap and continue, the inner
		// call is also visited by ast.Inspect so Must itself needs no
		// special handling. But we handle it explicitly for patterns like
		// template.Must(template.New("x").Parse(...))
		// where Must wraps the whole chain.

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		switch sel.Sel.Name {
		case "Must":
			// template.Must(...) — the inner expression is visited
			// by ast.Inspect, so nothing extra needed here.

		case "ParseFiles":
			for _, arg := range call.Args {
				if s := extractStringLit(arg); s != "" {
					parsedFiles = append(parsedFiles, s)
				}
			}

		case "ParseGlob":
			if len(call.Args) == 1 {
				if s := extractStringLit(call.Args[0]); s != "" {
					parsedFiles = append(parsedFiles, s)
				}
			}

		case "Parse":
			// .Parse(templateString) — the template content is inline
			// We don't need to do anything special here since the
			// template is embedded in Go code, not in a file.

		case "New":
			if len(call.Args) == 1 {
				if name := extractStringLit(call.Args[0]); name != "" {
					bindings = append(bindings, TemplateBinding{
						TemplateName: name,
						GoFile:       path,
						FuncMaps:     make(map[string]FuncSig),
					})
				}
			}

		case "Funcs":
			if len(call.Args) == 1 {
				fns := extractFuncMap(call.Args[0], file)
				for k, v := range fns {
					funcMaps[k] = v
				}
			}

		case "Execute":
			if len(call.Args) >= 2 {
				execCalls = append(execCalls, executeCall{
					dataExpr: call.Args[1],
					callPos:  call.Pos(),
				})
			}

		case "ExecuteTemplate":
			if len(call.Args) >= 3 {
				name := extractStringLit(call.Args[1])
				execCalls = append(execCalls, executeCall{
					templateName: name,
					dataExpr:     call.Args[2],
					callPos:      call.Pos(),
				})
			}
		}
		return true
	})

	// Build a list of function declarations for enclosing-function lookup
	var funcDecls []*ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			funcDecls = append(funcDecls, fd)
		}
	}

	// Process execute calls to build bindings with resolved types
	for _, ec := range execCalls {
		typeName := resolveTypeName(ec.dataExpr)
		// If direct resolution failed, try scope-aware variable lookup
		// using the identifier's Obj (declaration pointer set by parser)
		if typeName == "" {
			if ident, ok := ec.dataExpr.(*ast.Ident); ok {
				typeName = resolveVarViaObj(ident)
				if typeName == "" {
					typeName = varTypes[ident.Name]
				}
			}
		}

		var dataType *TypeInfo
		if typeName != "" {
			lookupName := strings.TrimPrefix(typeName, "*")
			dataType = ResolveTypeInfo(lookupName, structs, methods)
		}

		tmplName := ec.templateName
		if tmplName == "" && len(parsedFiles) > 0 {
			tmplName = filepath.Base(parsedFiles[0])
		}

		b := TemplateBinding{
			TemplateName: tmplName,
			DataType:     dataType,
			FuncMaps:     make(map[string]FuncSig),
			ParsedFiles:  parsedFiles,
			GoFile:       path,
			HandlerName:  enclosingFuncName(funcDecls, ec.callPos),
		}
		for k, v := range funcMaps {
			b.FuncMaps[k] = v
		}
		bindings = append(bindings, b)
	}

	// If no execute calls but we found parsed files, still create a binding
	if len(execCalls) == 0 && len(parsedFiles) > 0 {
		tmplName := ""
		if len(parsedFiles) > 0 {
			tmplName = filepath.Base(parsedFiles[0])
		}
		b := TemplateBinding{
			TemplateName: tmplName,
			ParsedFiles:  parsedFiles,
			GoFile:       path,
			FuncMaps:     make(map[string]FuncSig),
		}
		for k, v := range funcMaps {
			b.FuncMaps[k] = v
		}
		bindings = append(bindings, b)
	}

	// Apply funcmaps to all bindings from this file
	for i := range bindings {
		if bindings[i].FuncMaps == nil {
			bindings[i].FuncMaps = make(map[string]FuncSig)
		}
		for k, v := range funcMaps {
			if _, exists := bindings[i].FuncMaps[k]; !exists {
				bindings[i].FuncMaps[k] = v
			}
		}
	}

	return bindings
}

// enclosingFuncName returns the name of the function declaration that contains pos.
func enclosingFuncName(funcDecls []*ast.FuncDecl, pos token.Pos) string {
	for _, fd := range funcDecls {
		if fd.Body != nil && fd.Body.Pos() <= pos && pos <= fd.Body.End() {
			return fd.Name.Name
		}
	}
	return ""
}

// collectVarTypes traces short variable declarations and assignments to
// resolve their type names. Handles:
//
//	data := HomeData{...}       -> varTypes["data"] = "HomeData"
//	data := &HomeData{...}      -> varTypes["data"] = "HomeData"
//	var data HomeData            -> varTypes["data"] = "HomeData"
//	var data *HomeData           -> varTypes["data"] = "HomeData"
//	data := getData()            -> (not resolved)
func collectVarTypes(file *ast.File) map[string]string {
	result := make(map[string]string)

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Short var decl: data := HomeData{...}
			if node.Tok == token.DEFINE && len(node.Lhs) == len(node.Rhs) {
				for i, lhs := range node.Lhs {
					ident, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					typeName := resolveTypeName(node.Rhs[i])
					if typeName != "" {
						result[ident.Name] = typeName
					}
				}
			}
			// Regular assignment: data = HomeData{...}
			if node.Tok == token.ASSIGN && len(node.Lhs) == len(node.Rhs) {
				for i, lhs := range node.Lhs {
					ident, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					typeName := resolveTypeName(node.Rhs[i])
					if typeName != "" {
						result[ident.Name] = typeName
					}
				}
			}

		case *ast.GenDecl:
			// var data HomeData
			if node.Tok == token.VAR {
				for _, spec := range node.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					typeName := exprToString(vs.Type)
					typeName = strings.TrimPrefix(typeName, "*")
					if typeName != "" {
						for _, name := range vs.Names {
							result[name.Name] = typeName
						}
					}
					// var data = HomeData{...}
					if typeName == "" && len(vs.Values) > 0 {
						for i, name := range vs.Names {
							if i < len(vs.Values) {
								tn := resolveTypeName(vs.Values[i])
								if tn != "" {
									result[name.Name] = tn
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	return result
}

// collectStructs finds all struct type declarations in a parsed file.
func collectStructs(file *ast.File, fset *token.FileSet, goFile string) map[string][]FieldInfo {
	result := make(map[string][]FieldInfo)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			name := typeSpec.Name.Name
			var fields []FieldInfo
			for _, field := range structType.Fields.List {
				typeName := exprToString(field.Type)
				doc := ""
				if field.Doc != nil {
					doc = strings.TrimSpace(field.Doc.Text())
				} else if field.Comment != nil {
					doc = strings.TrimSpace(field.Comment.Text())
				}
				for _, ident := range field.Names {
					pos := fset.Position(ident.Pos())
					fields = append(fields, FieldInfo{
						Name:     ident.Name,
						TypeName: typeName,
						Doc:      doc,
						GoFile:   goFile,
						Line:     pos.Line - 1,
						Col:      pos.Column - 1,
					})
				}
				// Embedded field (no names)
				if len(field.Names) == 0 {
					embeddedName := typeName
					if idx := strings.LastIndex(embeddedName, "."); idx >= 0 {
						embeddedName = embeddedName[idx+1:]
					}
					if strings.HasPrefix(embeddedName, "*") {
						embeddedName = embeddedName[1:]
					}
					pos := fset.Position(field.Type.Pos())
					fields = append(fields, FieldInfo{
						Name:     embeddedName,
						TypeName: typeName,
						Doc:      doc,
						GoFile:   goFile,
						Line:     pos.Line - 1,
						Col:      pos.Column - 1,
					})
				}
			}
			result[name] = fields
		}
	}
	return result
}

// resolveVarViaObj resolves a variable's type using the parser's Obj pointer,
// which correctly links each identifier to its own declaration scope.
// This avoids the file-wide varTypes map bug where multiple `data := ...`
// assignments in different function literals overwrite each other.
func resolveVarViaObj(ident *ast.Ident) string {
	if ident.Obj == nil || ident.Obj.Decl == nil {
		return ""
	}
	switch decl := ident.Obj.Decl.(type) {
	case *ast.AssignStmt:
		// data := HomeData{...}
		for i, lhs := range decl.Lhs {
			lhsIdent, ok := lhs.(*ast.Ident)
			if !ok || lhsIdent.Name != ident.Name {
				continue
			}
			if i < len(decl.Rhs) {
				return resolveTypeName(decl.Rhs[i])
			}
		}
	case *ast.ValueSpec:
		// var data HomeData
		typeName := exprToString(decl.Type)
		typeName = strings.TrimPrefix(typeName, "*")
		if typeName != "" {
			return typeName
		}
		// var data = HomeData{...}
		for i, name := range decl.Names {
			if name.Name == ident.Name && i < len(decl.Values) {
				return resolveTypeName(decl.Values[i])
			}
		}
	}
	return ""
}

// resolveTypeName extracts the type name from an expression used as a data argument.
// Handles: Type{}, &Type{}, Type{field: val}, pkg.Type{}, (*Type)(x)
func resolveTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.CompositeLit:
		return exprToString(e.Type)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return resolveTypeName(e.X)
		}
	case *ast.Ident:
		return ""
	case *ast.SelectorExpr:
		return exprToString(e)
	case *ast.CallExpr:
		// Type conversion: HomeData(x) or (*HomeData)(x)
		if len(e.Args) == 1 {
			return exprToString(e.Fun)
		}
	}
	return ""
}

// exprToString converts a type expression to its string representation.
func exprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToString(e.Elt)
		}
		return fmt.Sprintf("[%s]%s", exprToString(e.Len), exprToString(e.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", exprToString(e.Key), exprToString(e.Value))
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ParenExpr:
		return exprToString(e.X)
	}
	return ""
}

func extractStringLit(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s := lit.Value
	if len(s) >= 2 {
		if s[0] == '"' {
			s = s[1 : len(s)-1]
		} else if s[0] == '`' {
			s = s[1 : len(s)-1]
		}
	}
	return s
}

func extractFuncMap(expr ast.Expr, file *ast.File) map[string]FuncSig {
	return extractFuncMapWithPos(expr, file, nil, "")
}

func extractFuncMapWithPos(expr ast.Expr, file *ast.File, fset *token.FileSet, goFile string) map[string]FuncSig {
	result := make(map[string]FuncSig)

	// If the argument is a variable reference, resolve it to its initializer
	if ident, ok := expr.(*ast.Ident); ok && file != nil {
		expr = resolveVarInit(ident.Name, file)
		if expr == nil {
			return result
		}
	}

	comp, ok := expr.(*ast.CompositeLit)
	if !ok {
		return result
	}
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		name := extractStringLit(kv.Key)
		if name == "" {
			continue
		}
		sig := FuncSig{Name: name}
		if fset != nil && goFile != "" {
			pos := fset.Position(kv.Key.Pos())
			sig.GoFile = goFile
			sig.Line = pos.Line - 1 // 0-based
			sig.Col = pos.Column - 1
		}
		result[name] = sig
	}
	return result
}

// resolveVarInit finds the initializer expression for a variable name.
// Handles: varName := expr, var varName = expr
func resolveVarInit(name string, file *ast.File) ast.Expr {
	var found ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		switch node := n.(type) {
		case *ast.AssignStmt:
			if len(node.Lhs) == len(node.Rhs) {
				for i, lhs := range node.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
						found = node.Rhs[i]
						return false
					}
				}
			}
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, n := range vs.Names {
					if n.Name == name && i < len(vs.Values) {
						found = vs.Values[i]
						return false
					}
				}
			}
		}
		return true
	})
	return found
}
