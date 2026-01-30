package goanalysis

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// TypeInfo holds resolved Go type information for template dot-context.
type TypeInfo struct {
	Name    string
	Package string
	Fields  map[string]FieldInfo
	Methods map[string]MethodInfo
}

// FieldInfo describes a struct field.
type FieldInfo struct {
	Name      string
	TypeName  string
	Doc       string
	ChildType *TypeInfo // resolved type for nested struct fields
	GoFile    string    // source file containing the field declaration
	Line      int       // 0-based line number in GoFile
	Col       int       // 0-based column in GoFile
}

// MethodInfo describes a method.
type MethodInfo struct {
	Name       string
	Signature  string
	Doc        string
	Returns    string    // return type name, if simple
	ReturnType *TypeInfo // resolved return type for chain navigation
	GoFile     string    // source file containing the method declaration
	Line       int       // 0-based line number
	Col        int       // 0-based column
}

// FuncSig describes a template function signature.
type FuncSig struct {
	Name      string
	Signature string
	Doc       string
	GoFile    string // source file containing the FuncMap entry
	Line      int    // 0-based line number
	Col       int    // 0-based column
}

// NewTypeInfo creates an empty TypeInfo.
func NewTypeInfo(name, pkg string) *TypeInfo {
	return &TypeInfo{
		Name:    name,
		Package: pkg,
		Fields:  make(map[string]FieldInfo),
		Methods: make(map[string]MethodInfo),
	}
}

// ResolveFieldChain walks a dot-separated field chain (e.g., ["User", "Name"])
// against a TypeInfo, returning the final TypeInfo at the end of the chain.
// Returns nil if any link in the chain can't be resolved.
// Also follows method return types (e.g., .GetUser returns User → .GetUser.Name works).
func ResolveFieldChain(root *TypeInfo, fields []string) *TypeInfo {
	current := root
	for _, name := range fields {
		if current == nil {
			return nil
		}
		if fi, ok := current.Fields[name]; ok {
			if fi.ChildType != nil {
				current = fi.ChildType
			} else {
				return nil // leaf field, no further nesting
			}
		} else if mi, ok := current.Methods[name]; ok {
			if mi.ReturnType != nil {
				current = mi.ReturnType
			} else {
				return nil // method without resolved return type
			}
		} else {
			return nil
		}
	}
	return current
}

// ElementTypeName extracts the element type from a slice/array type name.
// e.g., "[]Item" -> "Item", "[]*Item" -> "Item", "[]string" -> "string"
// Returns the original name if it's not a slice/array type.
func ElementTypeName(typeName string) string {
	s := typeName
	s = strings.TrimPrefix(s, "[]")
	s = strings.TrimPrefix(s, "*")
	return s
}

// ResolveTypeInfo builds a fully resolved TypeInfo for a struct name,
// recursively resolving nested struct fields up to a depth limit.
// structs is the map of all known struct definitions.
// methods is the map of type name -> methods.
func ResolveTypeInfo(name string, structs map[string][]FieldInfo, methods map[string][]MethodInfo) *TypeInfo {
	return resolveTypeInfoRecursive(name, structs, methods, 0)
}

func resolveTypeInfoRecursive(name string, structs map[string][]FieldInfo, methods map[string][]MethodInfo, depth int) *TypeInfo {
	if depth > 5 {
		return NewTypeInfo(name, "")
	}
	fields, ok := structs[name]
	if !ok {
		return NewTypeInfo(name, "")
	}
	ti := NewTypeInfo(name, "")
	for _, f := range fields {
		fi := f
		// Resolve nested struct type
		childTypeName := strings.TrimPrefix(f.TypeName, "*")
		childTypeName = strings.TrimPrefix(childTypeName, "[]")
		childTypeName = strings.TrimPrefix(childTypeName, "*")
		if _, hasChild := structs[childTypeName]; hasChild && childTypeName != name {
			fi.ChildType = resolveTypeInfoRecursive(childTypeName, structs, methods, depth+1)
		}
		ti.Fields[fi.Name] = fi
	}
	// Add methods
	if ms, ok := methods[name]; ok {
		for _, m := range ms {
			mi := m
			// Resolve return type if it's a known struct
			retName := strings.TrimPrefix(m.Returns, "*")
			retName = strings.TrimPrefix(retName, "[]")
			retName = strings.TrimPrefix(retName, "*")
			if _, hasRet := structs[retName]; hasRet && retName != name {
				mi.ReturnType = resolveTypeInfoRecursive(retName, structs, methods, depth+1)
			}
			ti.Methods[mi.Name] = mi
		}
	}
	// Also add methods for pointer receiver
	if ms, ok := methods["*"+name]; ok {
		for _, m := range ms {
			mi := m
			retName := strings.TrimPrefix(m.Returns, "*")
			retName = strings.TrimPrefix(retName, "[]")
			retName = strings.TrimPrefix(retName, "*")
			if _, hasRet := structs[retName]; hasRet && retName != name {
				mi.ReturnType = resolveTypeInfoRecursive(retName, structs, methods, depth+1)
			}
			ti.Methods[mi.Name] = mi
		}
	}
	return ti
}

// CollectMethods scans a Go AST file for method declarations and returns
// a map of receiver type name -> methods.
func CollectMethods(file *ast.File, fset *token.FileSet) map[string][]MethodInfo {
	result := make(map[string][]MethodInfo)
	goFile := ""
	if fset != nil {
		f := fset.File(file.Pos())
		if f != nil {
			goFile = f.Name()
		}
	}

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}

		recv := funcDecl.Recv.List[0]
		recvTypeName := receiverTypeName(recv.Type)
		if recvTypeName == "" {
			continue
		}

		sig := buildMethodSignature(funcDecl)

		doc := ""
		if funcDecl.Doc != nil {
			doc = strings.TrimSpace(funcDecl.Doc.Text())
		}

		returns := ""
		if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) > 0 {
			var retTypes []string
			for _, r := range funcDecl.Type.Results.List {
				retTypes = append(retTypes, exprToStr(r.Type))
			}
			returns = strings.Join(retTypes, ", ")
		}

		pos := fset.Position(funcDecl.Name.Pos())
		mi := MethodInfo{
			Name:      funcDecl.Name.Name,
			Signature: sig,
			Doc:       doc,
			Returns:   returns,
			GoFile:    goFile,
			Line:      pos.Line - 1,
			Col:       pos.Column - 1,
		}

		baseName := strings.TrimPrefix(recvTypeName, "*")
		result[baseName] = append(result[baseName], mi)
	}

	return result
}

func receiverTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	case *ast.IndexExpr:
		// Generic type T[X]
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func buildMethodSignature(fn *ast.FuncDecl) string {
	var parts []string
	parts = append(parts, "func")

	// Parameters
	params := "("
	if fn.Type.Params != nil {
		var paramStrs []string
		for _, p := range fn.Type.Params.List {
			typStr := exprToStr(p.Type)
			if len(p.Names) > 0 {
				for _, name := range p.Names {
					paramStrs = append(paramStrs, name.Name+" "+typStr)
				}
			} else {
				paramStrs = append(paramStrs, typStr)
			}
		}
		params += strings.Join(paramStrs, ", ")
	}
	params += ")"
	parts = append(parts, fn.Name.Name+params)

	// Returns
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		var retStrs []string
		for _, r := range fn.Type.Results.List {
			retStrs = append(retStrs, exprToStr(r.Type))
		}
		if len(retStrs) == 1 {
			parts = append(parts, retStrs[0])
		} else {
			parts = append(parts, "("+strings.Join(retStrs, ", ")+")")
		}
	}

	return strings.Join(parts, " ")
}

func exprToStr(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToStr(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToStr(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToStr(e.Elt)
		}
		return fmt.Sprintf("[%s]%s", exprToStr(e.Len), exprToStr(e.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", exprToStr(e.Key), exprToStr(e.Value))
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + exprToStr(e.Elt)
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + exprToStr(e.Value)
	}
	return ""
}
