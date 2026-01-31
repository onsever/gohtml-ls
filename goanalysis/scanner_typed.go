package goanalysis

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

// ScanDirectoryTyped uses go/types to scan for template bindings.
// Returns nil if type-checking fails, signaling fallback to AST scanner.
func ScanDirectoryTyped(dir string) []TemplateBinding {
	lr := LoadDirectory(dir)
	if lr == nil {
		fmt.Fprintf(os.Stderr, "[gohtml-lsp] ScanDirectoryTyped: LoadDirectory returned nil\n")
		return nil
	}
	fmt.Fprintf(os.Stderr, "[gohtml-lsp] ScanDirectoryTyped: loaded %d packages\n", len(lr.Packages))
	for path, tp := range lr.Packages {
		fmt.Fprintf(os.Stderr, "[gohtml-lsp]   pkg=%s files=%d typed=%v\n", path, len(tp.Files), tp.Pkg != nil)
	}

	var bindings []TemplateBinding
	for _, tp := range lr.Packages {
		bindings = append(bindings, scanTypedPackage(tp, lr.Fset)...)
	}

	// Cross-package linking: connect template parsed files with Execute data types
	// when they are separated across function/package boundaries.
	linked := linkCrossPackageBindings(lr, bindings)
	if len(linked) > 0 {
		bindings = append(bindings, linked...)
		// Remove orphan bindings: those with a DataType but no TemplateName
		// and no ParsedFiles. These are Execute calls where the template
		// identity couldn't be determined in the per-file scan. The linker
		// has created proper bindings with both template name and data type.
		filtered := bindings[:0]
		for _, b := range bindings {
			if b.DataType != nil && b.TemplateName == "" && len(b.ParsedFiles) == 0 {
				continue
			}
			filtered = append(filtered, b)
		}
		bindings = filtered
	}

	if len(bindings) == 0 {
		return nil
	}
	return bindings
}

func scanTypedPackage(tp *TypedPackage, fset *token.FileSet) []TemplateBinding {
	var bindings []TemplateBinding

	for i, file := range tp.Files {
		goFile := tp.FilePaths[i]
		fb := scanTypedFile(file, tp, fset, goFile)
		bindings = append(bindings, fb...)
	}
	return bindings
}

type typedExecCall struct {
	templateName string
	dataExpr     ast.Expr
	callPos      token.Pos
}

func scanTypedFile(file *ast.File, tp *TypedPackage, fset *token.FileSet, goFile string) []TemplateBinding {
	var bindings []TemplateBinding
	var execCalls []typedExecCall
	var parsedFiles []string
	funcMaps := make(map[string]FuncSig)

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		methodName := sel.Sel.Name

		switch methodName {
		case "Execute":
			if len(call.Args) >= 2 && isTemplateMethod(sel, tp) {
				dataIdx := findDataArgIndex(sel, tp, "Execute")
				if dataIdx >= 0 && dataIdx < len(call.Args) {
					execCalls = append(execCalls, typedExecCall{
						dataExpr: call.Args[dataIdx],
						callPos:  call.Pos(),
					})
				}
			}

		case "ExecuteTemplate":
			if len(call.Args) >= 3 && isTemplateMethod(sel, tp) {
				name := extractStringLit(call.Args[1])
				dataIdx := findDataArgIndex(sel, tp, "ExecuteTemplate")
				if dataIdx >= 0 && dataIdx < len(call.Args) {
					execCalls = append(execCalls, typedExecCall{
						templateName: name,
						dataExpr:     call.Args[dataIdx],
						callPos:      call.Pos(),
					})
				}
			}

		case "ParseFiles":
			if isTemplateMethod(sel, tp) {
				for _, arg := range call.Args {
					if s := extractStringLit(arg); s != "" {
						parsedFiles = append(parsedFiles, s)
					}
				}
			}

		case "ParseGlob":
			if len(call.Args) == 1 && isTemplateMethod(sel, tp) {
				if s := extractStringLit(call.Args[0]); s != "" {
					parsedFiles = append(parsedFiles, s)
				}
			}

		case "ParseFS":
			if isTemplateMethod(sel, tp) {
				// First arg is fs.FS, remaining are patterns
				for j := 1; j < len(call.Args); j++ {
					if s := extractStringLit(call.Args[j]); s != "" {
						parsedFiles = append(parsedFiles, s)
					}
				}
			}

		case "Funcs":
			if len(call.Args) == 1 && isTemplateMethod(sel, tp) {
				fns := extractFuncMapWithPos(call.Args[0], file, fset, goFile)
				for k, v := range fns {
					funcMaps[k] = v
				}
			}

		case "New":
			if len(call.Args) == 1 {
				// Check if this is template.New()
				if isTemplatePkgCall(sel, tp) {
					if name := extractStringLit(call.Args[0]); name != "" {
						bindings = append(bindings, TemplateBinding{
							TemplateName: name,
							GoFile:       goFile,
							FuncMaps:     make(map[string]FuncSig),
						})
					}
				}
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

	// Process execute calls
	for _, ec := range execCalls {
		var dataType *TypeInfo
		if ec.dataExpr != nil {
			dataType = resolveDataTypeViaTypes(ec.dataExpr, tp, fset)
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
			GoFile:       goFile,
			HandlerName:  enclosingFuncName(funcDecls, ec.callPos),
		}
		for k, v := range funcMaps {
			b.FuncMaps[k] = v
		}
		bindings = append(bindings, b)
	}

	// If no execute calls but found parsed files
	if len(execCalls) == 0 && len(parsedFiles) > 0 {
		tmplName := ""
		if len(parsedFiles) > 0 {
			tmplName = filepath.Base(parsedFiles[0])
		}
		b := TemplateBinding{
			TemplateName: tmplName,
			ParsedFiles:  parsedFiles,
			GoFile:       goFile,
			FuncMaps:     make(map[string]FuncSig),
		}
		for k, v := range funcMaps {
			b.FuncMaps[k] = v
		}
		bindings = append(bindings, b)
	}

	// Apply funcmaps to all bindings
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

// isTemplateMethod checks if the selector's receiver is or wraps *template.Template.
func isTemplateMethod(sel *ast.SelectorExpr, tp *TypedPackage) bool {
	// Check via types.Info.Uses — the selector's Sel resolves to a method object
	obj := tp.Info.Uses[sel.Sel]
	if obj == nil {
		// Fallback: check if X is a package-level "template" ident
		return isTemplatePkgCall(sel, tp)
	}

	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}

	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}

	recv := sig.Recv()
	if recv == nil {
		// It's a package-level function, check if it's in html/template or text/template
		pkg := fn.Pkg()
		if pkg != nil {
			path := pkg.Path()
			return path == "html/template" || path == "text/template"
		}
		return false
	}

	recvType := recv.Type()
	if isTemplateType(recvType) {
		return true
	}

	// Also match wrapper types: if the receiver wraps *template.Template
	// and the method name matches Execute/ExecuteTemplate/ParseFiles/etc.,
	// treat it as a template method so we trace through wrapper calls.
	methodName := sel.Sel.Name
	if methodName == "Execute" || methodName == "ExecuteTemplate" ||
		methodName == "ParseFiles" || methodName == "ParseGlob" ||
		methodName == "ParseFS" || methodName == "Funcs" {
		if hasTemplateField(recvType) {
			return true
		}
		// Match interfaces with an Execute method that has a data parameter.
		// This handles controller interfaces like:
		//   type Template interface { Execute(w, r, data interface{}, ...) }
		if isTemplateInterface(recvType, methodName) {
			return true
		}
	}
	return false
}

// isTemplateInterface checks if t is an interface type that has an Execute-like
// method with a data parameter (interface{} or any), indicating it's a template
// execution interface (e.g., controllers.Template).
func isTemplateInterface(t types.Type, methodName string) bool {
	// Only consider Execute/ExecuteTemplate for interface detection
	if methodName != "Execute" && methodName != "ExecuteTemplate" {
		return false
	}
	// Unwrap pointer/named
	t = t.Underlying()
	iface, ok := t.(*types.Interface)
	if !ok {
		return false
	}
	// Look for an Execute or ExecuteTemplate method
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		if m.Name() != "Execute" && m.Name() != "ExecuteTemplate" {
			continue
		}
		sig, ok := m.Type().(*types.Signature)
		if !ok {
			continue
		}
		// Check if any parameter is named "data" or is interface{}
		for j := 0; j < sig.Params().Len(); j++ {
			p := sig.Params().At(j)
			if p.Name() == "data" {
				return true
			}
			// Also accept unnamed interface{} params after position 1
			if j >= 2 {
				if _, ok := p.Type().Underlying().(*types.Interface); ok {
					return true
				}
			}
		}
	}
	return false
}

// isTemplatePkgCall checks if sel.X is the "template" package identifier.
func isTemplatePkgCall(sel *ast.SelectorExpr, tp *TypedPackage) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	obj := tp.Info.Uses[ident]
	if obj == nil {
		// Fallback for unresolved: check name
		return ident.Name == "template"
	}
	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}
	path := pkgName.Imported().Path()
	return path == "html/template" || path == "text/template"
}

// isTemplateType checks if t IS or WRAPS *template.Template.
func isTemplateType(t types.Type) bool {
	t = derefType(t)

	// Check if it's directly template.Template
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil && obj.Pkg() != nil {
			path := obj.Pkg().Path()
			if (path == "html/template" || path == "text/template") && obj.Name() == "Template" {
				return true
			}
		}
		// Check underlying struct for embedded template.Template
		return hasTemplateField(t)
	}
	return false
}

// hasTemplateField checks if a struct type embeds or contains *template.Template.
func hasTemplateField(t types.Type) bool {
	t = derefType(t)
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		ft := derefType(f.Type())
		if n, ok := ft.(*types.Named); ok {
			obj := n.Obj()
			if obj != nil && obj.Pkg() != nil {
				path := obj.Pkg().Path()
				if (path == "html/template" || path == "text/template") && obj.Name() == "Template" {
					return true
				}
			}
		}
	}
	return false
}

// findDataArgIndex determines the index of the "data interface{}" parameter
// in an Execute/ExecuteTemplate call. Standard template.Execute has data at index 1,
// but wrapper types may have different signatures (e.g., Execute(w, r, data, ...errs)).
func findDataArgIndex(sel *ast.SelectorExpr, tp *TypedPackage, methodName string) int {
	obj := tp.Info.Uses[sel.Sel]
	if obj == nil {
		// Default: standard template signature
		if methodName == "ExecuteTemplate" {
			return 2
		}
		return 1
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		if methodName == "ExecuteTemplate" {
			return 2
		}
		return 1
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		if methodName == "ExecuteTemplate" {
			return 2
		}
		return 1
	}
	// Find the parameter named "data" or typed "interface{}"/"any"
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		p := params.At(i)
		if p.Name() == "data" {
			return i
		}
		// Check if it's interface{} type (common for template data)
		if iface, ok := p.Type().Underlying().(*types.Interface); ok {
			// Skip io.Writer / http.ResponseWriter (interfaces with methods)
			// The "data any" param is an empty interface
			if iface.NumMethods() == 0 {
				return i
			}
		}
	}
	// Default
	if methodName == "ExecuteTemplate" {
		return 2
	}
	return 1
}

func derefType(t types.Type) types.Type {
	for {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			return t
		}
		t = ptr.Elem()
	}
}

// resolveDataTypeViaTypes resolves the data expression's type using go/types.
func resolveDataTypeViaTypes(expr ast.Expr, tp *TypedPackage, fset *token.FileSet) *TypeInfo {
	tv, ok := tp.Info.Types[expr]
	if ok {
		return convertToTypeInfo(tv.Type, fset, 0)
	}
	// Info.Types excludes identifiers denoting declared objects (variables,
	// parameters, etc.). Fall back to Info.Uses / Info.ObjectOf.
	if ident, ok := expr.(*ast.Ident); ok {
		obj := tp.Info.ObjectOf(ident)
		if obj != nil {
			return convertToTypeInfo(obj.Type(), fset, 0)
		}
	}
	return nil
}

// convertToTypeInfo converts a go/types.Type to our TypeInfo structure.
func convertToTypeInfo(t types.Type, fset *token.FileSet, depth int) *TypeInfo {
	if depth > 5 || t == nil {
		return nil
	}

	// Unwrap pointer
	t = derefType(t)

	switch typ := t.(type) {
	case *types.Named:
		name := typ.Obj().Name()
		ti := NewTypeInfo(name, "")
		if typ.Obj().Pkg() != nil {
			ti.Package = typ.Obj().Pkg().Name()
		}

		// Get struct fields from underlying type
		if st, ok := typ.Underlying().(*types.Struct); ok {
			populateFieldsFromStruct(ti, st, fset, depth)
		}

		// Collect methods
		mset := types.NewMethodSet(types.NewPointer(typ))
		for i := 0; i < mset.Len(); i++ {
			sel := mset.At(i)
			fn, ok := sel.Obj().(*types.Func)
			if !ok {
				continue
			}
			// Skip unexported
			if !fn.Exported() {
				continue
			}
			mi := MethodInfo{
				Name:      fn.Name(),
				Signature: fn.Type().String(),
			}
			if fset != nil && fn.Pos().IsValid() {
				pos := fset.Position(fn.Pos())
				mi.GoFile = pos.Filename
				mi.Line = pos.Line - 1
				mi.Col = pos.Column - 1
			}
			// Extract return type and resolve ReturnType
			sig, ok := fn.Type().(*types.Signature)
			if ok && sig.Results() != nil && sig.Results().Len() > 0 {
				var rets []string
				for j := 0; j < sig.Results().Len(); j++ {
					rets = append(rets, sig.Results().At(j).Type().String())
				}
				mi.Returns = strings.Join(rets, ", ")
				// If single return value, try to resolve its TypeInfo
				if sig.Results().Len() == 1 {
					retType := sig.Results().At(0).Type()
					mi.ReturnType = convertToTypeInfo(retType, fset, depth+1)
				}
			}
			ti.Methods[mi.Name] = mi
		}

		return ti

	case *types.Struct:
		// Anonymous struct
		ti := NewTypeInfo("", "")
		populateFieldsFromStruct(ti, typ, fset, depth)
		return ti

	case *types.Slice:
		// For slices, resolve the element type (used by range)
		elem := typ.Elem()
		elemTi := convertToTypeInfo(elem, fset, depth+1)
		if elemTi != nil {
			return elemTi
		}
		return nil

	default:
		return nil
	}
}

func populateFieldsFromStruct(ti *TypeInfo, st *types.Struct, fset *token.FileSet, depth int) {
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Exported() {
			continue
		}
		fi := FieldInfo{
			Name:     f.Name(),
			TypeName: typeString(f.Type()),
		}
		if fset != nil && f.Pos().IsValid() {
			pos := fset.Position(f.Pos())
			fi.GoFile = pos.Filename
			fi.Line = pos.Line - 1
			fi.Col = pos.Column - 1
		}
		// Resolve child type for nested structs
		childT := derefType(f.Type())
		// Unwrap slice for nested resolution
		if sl, ok := childT.(*types.Slice); ok {
			childT = derefType(sl.Elem())
		}
		if named, ok := childT.(*types.Named); ok {
			if _, ok := named.Underlying().(*types.Struct); ok {
				fi.ChildType = convertToTypeInfo(named, fset, depth+1)
			}
		}
		ti.Fields[fi.Name] = fi
	}
}

func typeString(t types.Type) string {
	// Produce a short type name similar to what AST scanner produces
	t2 := derefType(t)
	switch typ := t2.(type) {
	case *types.Named:
		return typ.Obj().Name()
	case *types.Basic:
		return typ.Name()
	case *types.Slice:
		return "[]" + typeString(typ.Elem())
	case *types.Map:
		return "map[" + typeString(typ.Key()) + "]" + typeString(typ.Elem())
	case *types.Interface:
		return "interface{}"
	}
	return t.String()
}

// --- Cross-package linking ---
//
// Handles the pattern where template parsed files and Execute data types are
// in different packages/functions:
//
//   main.go:       tpl = views.Must(views.ParseFS(fs, "faq.gohtml"))
//                  controllers.FAQ(tpl)
//   static.go:     func FAQ(tpl views.Template) { tpl.Execute(w, data) }
//
// Steps:
// 1. Find template-producing calls (ParseFS/ParseFiles wrappers) → track variable → parsed files
// 2. Find function calls where template variables are passed as arguments
// 3. Find Execute calls inside those functions on the parameter → create linked bindings

// funcParamKey identifies a specific parameter of a function.
type funcParamKey struct {
	funcPos token.Pos
	index   int
}

// linkCrossPackageBindings creates new bindings by tracing template values
// through function arguments across package boundaries.
func linkCrossPackageBindings(lr *LoadResult, existing []TemplateBinding) []TemplateBinding {
	// Step 1: Track variables that hold templates with known parsed files.
	// varFiles: variable definition position → parsed files
	varFiles := map[token.Pos][]string{}

	for _, tp := range lr.Packages {
		for _, file := range tp.Files {
			collectTemplateVarFiles(file, tp, lr, varFiles)
		}
	}

	if len(varFiles) == 0 {
		return nil
	}

	// Step 2: Find function calls where template variables are passed as arguments.
	// paramFiles: (function def position, param index) → parsed files
	paramFiles := map[funcParamKey][]string{}

	for _, tp := range lr.Packages {
		for _, file := range tp.Files {
			collectFuncCallTemplateArgs(file, tp, lr, varFiles, paramFiles)
		}
	}

	if len(paramFiles) == 0 {
		return nil
	}

	// Step 3: Find Execute calls on function parameters or struct fields
	// and create linked bindings.
	var linked []TemplateBinding
	for _, tp := range lr.Packages {
		for i, file := range tp.Files {
			goFile := tp.FilePaths[i]
			lb := findLinkedExecuteCalls(file, tp, lr.Fset, goFile, paramFiles, varFiles)
			linked = append(linked, lb...)
		}
	}

	// Propagate FuncMaps from existing bindings to linked bindings.
	// Custom functions (csrfField, currentUser, etc.) are defined in the
	// package that parses templates (e.g., views/) but Execute calls are in
	// a different package (e.g., controllers/). Merge all known FuncMaps.
	if len(linked) > 0 {
		allFuncs := map[string]FuncSig{}
		for _, b := range existing {
			for k, v := range b.FuncMaps {
				allFuncs[k] = v
			}
		}
		if len(allFuncs) > 0 {
			for i := range linked {
				if linked[i].FuncMaps == nil {
					linked[i].FuncMaps = make(map[string]FuncSig)
				}
				for k, v := range allFuncs {
					if _, exists := linked[i].FuncMaps[k]; !exists {
						linked[i].FuncMaps[k] = v
					}
				}
			}
		}
	}

	return linked
}

// collectTemplateVarFiles finds assignments where the RHS produces a template
// with known parsed files (e.g., views.Must(views.ParseFS(fs, "faq.gohtml"))).
// Handles both simple ident assignments and selector chain assignments
// (e.g., usersC.Templates.New = views.Must(views.ParseFS(...))).
func collectTemplateVarFiles(file *ast.File, tp *TypedPackage, lr *LoadResult, varFiles map[token.Pos][]string) {
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range assign.Rhs {
			if i >= len(assign.Lhs) {
				continue
			}
			files := extractParsedFilesDeep(rhs, tp, lr)
			if len(files) == 0 {
				continue
			}
			// Try simple ident first
			if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
				obj := tp.Info.ObjectOf(ident)
				if obj != nil {
					varFiles[obj.Pos()] = files
				}
				continue
			}
			// Handle selector chain: a.b.c = expr
			// Resolve the deepest selector's field object
			if sel, ok := assign.Lhs[i].(*ast.SelectorExpr); ok {
				obj := tp.Info.ObjectOf(sel.Sel)
				if obj != nil {
					varFiles[obj.Pos()] = files
				}
			}
		}
		return true
	})
}

// extractParsedFilesDeep extracts parsed file names from an expression,
// unwrapping calls like views.Must(views.ParseFS(fs, "faq.gohtml")).
func extractParsedFilesDeep(expr ast.Expr, tp *TypedPackage, lr *LoadResult) []string {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}

	// Check if this call itself is a ParseFS/ParseFiles/ParseGlob
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		name := sel.Sel.Name
		if name == "ParseFS" || name == "ParseFiles" || name == "ParseGlob" {
			if isTemplateRelatedFunc(sel, tp) {
				return extractStringArgsFromCall(call, name)
			}
		}
	}

	// Unwrap wrapper calls: check each argument recursively.
	// Handles patterns like views.Must(views.ParseFS(...)) where Must wraps
	// the inner call.
	for _, arg := range call.Args {
		files := extractParsedFilesDeep(arg, tp, lr)
		if len(files) > 0 {
			return files
		}
	}

	return nil
}

// isTemplateRelatedFunc checks if a selector refers to a function that deals
// with templates — either in html/template, text/template, or a package that
// returns a template-wrapper type.
func isTemplateRelatedFunc(sel *ast.SelectorExpr, tp *TypedPackage) bool {
	// First check: is it a standard template package call?
	if isTemplatePkgCall(sel, tp) {
		return true
	}

	// Second check: does the function return a type that wraps *template.Template?
	obj := tp.Info.Uses[sel.Sel]
	if obj == nil {
		return false
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}
	results := sig.Results()
	if results == nil {
		return false
	}
	for i := 0; i < results.Len(); i++ {
		rt := results.At(i).Type()
		if isTemplateType(rt) {
			return true
		}
	}
	return false
}

// extractStringArgsFromCall extracts string literal arguments from a call.
// For ParseFS, skips the first arg (fs.FS). For ParseFiles/ParseGlob, all args.
func extractStringArgsFromCall(call *ast.CallExpr, funcName string) []string {
	var files []string
	start := 0
	if funcName == "ParseFS" {
		start = 1 // skip fs.FS argument
	}
	for i := start; i < len(call.Args); i++ {
		if s := extractStringLit(call.Args[i]); s != "" {
			files = append(files, s)
		}
	}
	return files
}

// collectFuncCallTemplateArgs finds function calls where a template variable
// (with known parsed files) is passed as an argument. Also handles inline
// expressions like controllers.FAQ(views.Must(views.ParseFS(...))).
func collectFuncCallTemplateArgs(file *ast.File, tp *TypedPackage, lr *LoadResult, varFiles map[token.Pos][]string, paramFiles map[funcParamKey][]string) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Resolve the called function
		calledObj := resolveCalledFunc(call, tp)
		if calledObj == nil {
			return true
		}

		for i, arg := range call.Args {
			var files []string

			// Try simple ident: variable with known parsed files
			if argIdent, ok := arg.(*ast.Ident); ok {
				argObj := tp.Info.ObjectOf(argIdent)
				if argObj != nil {
					files = varFiles[argObj.Pos()]
				}
			}

			// Try inline expression: views.Must(views.ParseFS(...))
			if len(files) == 0 {
				files = extractParsedFilesDeep(arg, tp, lr)
			}

			if len(files) > 0 {
				key := funcParamKey{funcPos: calledObj.Pos(), index: i}
				paramFiles[key] = files
			}
		}
		return true
	})
}

// resolveCalledFunc resolves a call expression to its function object.
func resolveCalledFunc(call *ast.CallExpr, tp *TypedPackage) types.Object {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return tp.Info.Uses[fn]
	case *ast.SelectorExpr:
		return tp.Info.Uses[fn.Sel]
	}
	return nil
}

// findLinkedExecuteCalls walks a file looking for Execute calls where the
// receiver is a function parameter or struct field with known parsed files.
func findLinkedExecuteCalls(file *ast.File, tp *TypedPackage, fset *token.FileSet, goFile string, paramFiles map[funcParamKey][]string, varFiles map[token.Pos][]string) []TemplateBinding {
	var bindings []TemplateBinding

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			continue
		}

		// Get the function object
		funcObj := tp.Info.Defs[funcDecl.Name]
		if funcObj == nil {
			continue
		}

		// Build a map: parameter variable position → (funcPos, paramIndex)
		paramPositions := map[token.Pos]funcParamKey{}
		if funcDecl.Type.Params != nil {
			idx := 0
			for _, field := range funcDecl.Type.Params.List {
				for _, name := range field.Names {
					paramObj := tp.Info.Defs[name]
					if paramObj != nil {
						paramPositions[paramObj.Pos()] = funcParamKey{
							funcPos: funcObj.Pos(),
							index:   idx,
						}
					}
					idx++
				}
				if len(field.Names) == 0 {
					idx++
				}
			}
		}

		// Walk the function body for Execute calls
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			methodName := sel.Sel.Name
			if methodName != "Execute" && methodName != "ExecuteTemplate" {
				return true
			}

			if !isTemplateMethod(sel, tp) {
				return true
			}

			// Try to find parsed files for the receiver
			var files []string

			// Strategy 1: receiver is a simple ident (function param or local var)
			recvObj := resolveReceiverObj(sel.X, tp)
			if recvObj != nil {
				// Check function parameter
				if key, ok := paramPositions[recvObj.Pos()]; ok {
					files = paramFiles[key]
				}
				// Check variable files
				if len(files) == 0 {
					files = varFiles[recvObj.Pos()]
				}
			}

			// Strategy 2: receiver is a selector chain (e.g., u.Templates.New)
			if len(files) == 0 {
				files = resolveReceiverFiles(sel.X, tp, varFiles)
			}

			if len(files) == 0 {
				return true
			}

			// Resolve data type
			dataIdx := findDataArgIndex(sel, tp, methodName)
			var dataType *TypeInfo
			if dataIdx >= 0 && dataIdx < len(call.Args) {
				dataType = resolveDataTypeViaTypes(call.Args[dataIdx], tp, fset)
			}

			tmplName := ""
			if methodName == "ExecuteTemplate" && len(call.Args) >= 2 {
				tmplName = extractStringLit(call.Args[1])
			}
			if tmplName == "" && len(files) > 0 {
				tmplName = filepath.Base(files[0])
			}

			bindings = append(bindings, TemplateBinding{
				TemplateName: tmplName,
				DataType:     dataType,
				ParsedFiles:  files,
				GoFile:       goFile,
				FuncMaps:     make(map[string]FuncSig),
				HandlerName:  funcDecl.Name.Name,
			})

			return true
		})
	}

	return bindings
}

// resolveReceiverFiles resolves parsed files for a selector chain receiver
// like u.Templates.New by checking the deepest selector's field object.
func resolveReceiverFiles(expr ast.Expr, tp *TypedPackage, varFiles map[token.Pos][]string) []string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	obj := tp.Info.ObjectOf(sel.Sel)
	if obj == nil {
		return nil
	}
	return varFiles[obj.Pos()]
}

// resolveReceiverObj resolves an expression to its underlying types.Object.
// Handles identifiers directly and method call chains like tpl.Execute
// where tpl is a variable.
func resolveReceiverObj(expr ast.Expr, tp *TypedPackage) types.Object {
	switch e := expr.(type) {
	case *ast.Ident:
		return tp.Info.ObjectOf(e)
	}
	return nil
}
