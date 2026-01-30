package features

import (
	"strings"
	"text/template/parse"

	"github.com/onsever/gohtml-ls/goanalysis"
	tmpl "github.com/onsever/gohtml-ls/template"
)

// resolveDotTypeAtOffset determines the dot type at a given offset,
// taking into account range/with re-scoping.
// Returns the re-scoped type, or the root dataType if no scope applies.
func resolveDotTypeAtOffset(pt *tmpl.ParsedTemplate, offset int, dataType *goanalysis.TypeInfo) *goanalysis.TypeInfo {
	if dataType == nil {
		return nil
	}
	scopes := pt.ScopeAtOffset(offset)
	if len(scopes) == 0 {
		return dataType
	}

	current := dataType
	for _, scope := range scopes {
		if scope.Field == nil || current == nil {
			break
		}
		// Walk the field chain in the scope's pipe
		var lastField *goanalysis.FieldInfo
		walk := current
		for _, fname := range scope.Field.Ident {
			if walk == nil {
				break
			}
			fi, ok := walk.Fields[fname]
			if !ok {
				walk = nil
				break
			}
			lastField = &fi
			walk = fi.ChildType
		}
		if lastField == nil {
			break
		}

		if scope.Kind == "range" {
			// For range, dot becomes the element type
			typeName := lastField.TypeName
			if strings.HasPrefix(typeName, "[]") {
				elemName := goanalysis.ElementTypeName(typeName)
				if lastField.ChildType != nil && lastField.ChildType.Name == elemName {
					current = lastField.ChildType
				} else {
					current = goanalysis.NewTypeInfo(elemName, "")
				}
			} else if lastField.ChildType != nil {
				current = lastField.ChildType
			} else {
				current = goanalysis.NewTypeInfo(lastField.TypeName, "")
			}
		} else if scope.Kind == "with" {
			// For with, dot becomes the field's type
			if lastField.ChildType != nil {
				current = lastField.ChildType
			} else {
				current = goanalysis.NewTypeInfo(lastField.TypeName, "")
			}
		}
	}
	return current
}

// resolveVariableType resolves the type of a template variable like $v
// declared in {{range $i, $v := .Items}} by walking the scope chain.
// Returns the resolved TypeInfo and a description of the variable's role.
func resolveVariableType(varName string, pt *tmpl.ParsedTemplate, offset int, bindings []goanalysis.TemplateBinding) *goanalysis.TypeInfo {
	scopes := pt.ScopeAtOffset(offset)
	if len(scopes) == 0 {
		return nil
	}

	// Find the innermost range scope that declares this variable
	// Walk scopes from outermost to innermost, tracking current type
	// We need the dataType from bindings first
	var dataType *goanalysis.TypeInfo
	for _, b := range bindings {
		if b.DataType != nil {
			dataType = b.DataType
			break
		}
	}
	if dataType == nil {
		return nil
	}

	current := dataType
	for _, scope := range scopes {
		if scope.Kind != "range" {
			// with re-scopes dot but doesn't declare vars
			if scope.Kind == "with" && scope.Field != nil && current != nil {
				current = resolveFieldToType(scope.Field, current)
			}
			continue
		}

		// Check if this range scope declares our variable
		varDeclared := false
		varIndex := -1
		for i, v := range scope.Vars {
			if v == varName {
				varDeclared = true
				varIndex = i
				break
			}
		}

		if varDeclared {
			// Resolve the range expression's element type
			if scope.Field == nil || current == nil {
				return nil
			}
			elemType := resolveRangeElementType(scope.Field, current)
			if elemType == nil {
				return nil
			}

			// If 2 vars ($i, $v): first is index (int), second is element
			// If 1 var ($v): it's the element type
			if len(scope.Vars) == 2 {
				if varIndex == 0 {
					return goanalysis.NewTypeInfo("int", "")
				}
				return elemType
			}
			return elemType
		}

		// This range re-scopes dot for deeper scopes
		if scope.Field != nil && current != nil {
			elemType := resolveRangeElementType(scope.Field, current)
			if elemType != nil {
				current = elemType
			}
		}
	}
	return nil
}

// resolveFieldToType resolves a field node to its type given a root type.
func resolveFieldToType(field *parse.FieldNode, current *goanalysis.TypeInfo) *goanalysis.TypeInfo {
	for _, fname := range field.Ident {
		if current == nil {
			return nil
		}
		fi, ok := current.Fields[fname]
		if !ok {
			return nil
		}
		if fi.ChildType != nil {
			current = fi.ChildType
		} else {
			current = goanalysis.NewTypeInfo(fi.TypeName, "")
		}
	}
	return current
}

// resolveRangeElementType resolves the element type when ranging over a field.
func resolveRangeElementType(field *parse.FieldNode, current *goanalysis.TypeInfo) *goanalysis.TypeInfo {
	var lastField *goanalysis.FieldInfo
	walk := current
	for _, fname := range field.Ident {
		if walk == nil {
			return nil
		}
		fi, ok := walk.Fields[fname]
		if !ok {
			return nil
		}
		lastField = &fi
		walk = fi.ChildType
	}
	if lastField == nil {
		return nil
	}
	typeName := lastField.TypeName
	if strings.HasPrefix(typeName, "[]") {
		elemName := goanalysis.ElementTypeName(typeName)
		if lastField.ChildType != nil && lastField.ChildType.Name == elemName {
			return lastField.ChildType
		}
		return goanalysis.NewTypeInfo(elemName, "")
	}
	if lastField.ChildType != nil {
		return lastField.ChildType
	}
	return goanalysis.NewTypeInfo(typeName, "")
}

// resolveTemplateCallDotType resolves the dot type for a child template
// called via {{template "child" .Field}}. It looks up the field chain
// passed to the child template and resolves it against the parent's bindings.
func resolveTemplateCallDotType(treeName string, index *tmpl.TemplateIndex, bindings []goanalysis.TemplateBinding) *goanalysis.TypeInfo {
	if index == nil {
		return nil
	}
	fieldChain, ok := index.TemplateArgs[treeName]
	if !ok || len(fieldChain) == 0 {
		return nil
	}
	// Find a binding that has a DataType (the parent's type)
	for _, b := range bindings {
		if b.DataType == nil {
			continue
		}
		result := goanalysis.ResolveFieldChain(b.DataType, fieldChain)
		if result != nil {
			return result
		}
	}
	return nil
}
