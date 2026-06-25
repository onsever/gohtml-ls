package features

import (
	"strings"

	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/lsp"
	tmpl "github.com/onsever/gohtml-ls/template"
)

// Completion returns completion items at the given position.
func Completion(pt *tmpl.ParsedTemplate, pos lsp.Position, index *tmpl.TemplateIndex, bindings []goanalysis.TemplateBinding) lsp.CompletionList {
	result := lsp.CompletionList{}

	// Get text before cursor
	offset := pt.PositionToOffset(pos)
	if offset > len(pt.Content) {
		offset = len(pt.Content)
	}
	textBefore := pt.Content[:offset]

	// Check if we're inside {{ }}
	lastOpen := strings.LastIndex(textBefore, "{{")
	lastClose := strings.LastIndex(textBefore, "}}")
	insideAction := lastOpen > lastClose && lastOpen >= 0

	if !insideAction {
		// Offer snippet completions for starting new actions
		result.Items = append(result.Items, snippetCompletions()...)
		return result
	}

	actionText := textBefore[lastOpen:]

	// After a dot - field/method completion
	// Check if the character immediately before cursor is a dot, or we're typing after one
	trimmed := strings.TrimRight(actionText, " \t")
	if strings.HasSuffix(trimmed, ".") {
		// Check if this is a variable dot chain like $v. or $v.Name.
		if varName, varChain := parseVarDotChain(trimmed); varName != "" {
			matchedBindings := matchBindings(pt.URI, bindings)
			if len(matchedBindings) == 0 {
				matchedBindings = bindings
			}
			if varType := resolveVariableType(varName, pt, offset, matchedBindings); varType != nil {
				target := varType
				if len(varChain) > 0 {
					target = goanalysis.ResolveFieldChain(varType, varChain)
				}
				if target != nil {
					seen := make(map[string]bool)
					for name, fi := range target.Fields {
						if !seen[name] {
							seen[name] = true
							result.Items = append(result.Items, lsp.CompletionItem{
								Label: name, Kind: lsp.CompletionItemKindField,
								Detail: fi.TypeName, InsertText: name,
							})
						}
					}
					for name, mi := range target.Methods {
						if !seen[name] {
							seen[name] = true
							result.Items = append(result.Items, lsp.CompletionItem{
								Label: name, Kind: lsp.CompletionItemKindMethod,
								Detail: mi.Signature, InsertText: name,
							})
						}
					}
				}
			}
			return result
		}

		// Parse the dot chain to find the target type for completion
		// e.g., ".User." -> chain is ["User"], complete with User's fields
		// e.g., "." -> chain is [], complete with root fields
		chain := parseDotChain(trimmed)

		matchedBindings := matchBindings(pt.URI, bindings)
		result.Items = appendFieldCompletions(result.Items, matchedBindings, chain, pt, offset)

		// Fallback: try bindings that are not specifically bound to another template.
		// Only use bindings with no TemplateName (generic) or matching this file.
		if len(result.Items) == 0 {
			var generic []goanalysis.TemplateBinding
			for _, b := range bindings {
				if b.TemplateName == "" || bindingMatchesURI(b, pt.URI) {
					generic = append(generic, b)
				}
			}
			result.Items = appendFieldCompletions(result.Items, generic, chain, pt, offset)
		}

		return result
	}

	// Inside {{template " - complete template names
	if strings.Contains(actionText, `template "`) {
		for name := range index.Definitions {
			result.Items = append(result.Items, lsp.CompletionItem{
				Label:      name,
				Kind:       lsp.CompletionItemKindText,
				InsertText: name,
			})
		}
		return result
	}

	// At {{ - keyword and function completions
	keywords := []string{"if", "else", "end", "range", "with", "template", "define", "block"}
	for _, kw := range keywords {
		result.Items = append(result.Items, lsp.CompletionItem{
			Label:      kw,
			Kind:       lsp.CompletionItemKindKeyword,
			InsertText: kw,
		})
	}

	// Built-in functions
	for name, sig := range goanalysis.BuiltinFuncs() {
		result.Items = append(result.Items, lsp.CompletionItem{
			Label:         name,
			Kind:          lsp.CompletionItemKindFunction,
			Detail:        sig.Signature,
			Documentation: sig.Doc,
			InsertText:    name,
		})
	}

	// Custom functions from bindings (deduplicated)
	seenFuncs := make(map[string]bool)
	for _, b := range bindings {
		for name, sig := range b.FuncMaps {
			if seenFuncs[name] {
				continue
			}
			seenFuncs[name] = true
			result.Items = append(result.Items, lsp.CompletionItem{
				Label:      name,
				Kind:       lsp.CompletionItemKindFunction,
				Detail:     sig.Signature,
				InsertText: name,
			})
		}
	}

	// Snippet completions
	result.Items = append(result.Items, snippetCompletions()...)

	return result
}

// parseVarDotChain checks if the text ends with a variable dot chain like "$v." or "$v.Name.".
// Returns the variable name and the intermediate field chain.
func parseVarDotChain(text string) (string, []string) {
	// Find the trailing dot
	idx := len(text) - 1
	if idx < 0 {
		return "", nil
	}
	// Walk backwards to find $varName.chain.
	start := idx
	for start > 0 {
		c := text[start-1]
		if c == '.' || c == '$' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			start--
		} else {
			break
		}
	}
	chain := text[start:idx] // e.g., "$v.Name"
	if !strings.HasPrefix(chain, "$") {
		return "", nil
	}
	parts := strings.Split(chain, ".")
	if len(parts) < 1 {
		return "", nil
	}
	varName := parts[0] // "$v"
	if len(parts) == 1 {
		return varName, nil
	}
	return varName, parts[1:]
}

// parseDotChain extracts the field chain from a dot expression ending with ".".
// e.g., ".User.Profile." -> ["User", "Profile"]
// e.g., "." -> []
func parseDotChain(text string) []string {
	// Find the last dot-expression: look backwards for the start
	// We need to find something like ".User.Profile."
	idx := len(text) - 1 // the trailing dot
	if idx < 0 {
		return nil
	}
	// Walk backwards to find the start of the dot chain
	start := idx
	for start > 0 {
		c := text[start-1]
		if c == '.' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			start--
		} else {
			break
		}
	}
	chain := text[start:idx] // e.g., ".User.Profile"
	if !strings.HasPrefix(chain, ".") {
		return nil
	}
	chain = chain[1:] // remove leading dot
	if chain == "" {
		return nil
	}
	return strings.Split(chain, ".")
}

// appendFieldCompletions adds field/method completions for the resolved type at the end of chain.
func appendFieldCompletions(items []lsp.CompletionItem, bindings []goanalysis.TemplateBinding, chain []string, pt *tmpl.ParsedTemplate, offset int) []lsp.CompletionItem {
	seen := make(map[string]bool)
	for _, b := range bindings {
		if b.DataType == nil {
			continue
		}
		// Resolve dot type considering range/with scopes
		scopedType := resolveDotTypeAtOffset(pt, offset, b.DataType)
		target := scopedType
		if len(chain) > 0 {
			target = goanalysis.ResolveFieldChain(scopedType, chain)
		}
		if target == nil {
			continue
		}
		for name, fi := range target.Fields {
			if seen[name] {
				continue
			}
			seen[name] = true
			items = append(items, lsp.CompletionItem{
				Label:      name,
				Kind:       lsp.CompletionItemKindField,
				Detail:     fi.TypeName,
				InsertText: name,
			})
		}
		for name, mi := range target.Methods {
			if seen[name] {
				continue
			}
			seen[name] = true
			items = append(items, lsp.CompletionItem{
				Label:      name,
				Kind:       lsp.CompletionItemKindMethod,
				Detail:     mi.Signature,
				InsertText: name,
			})
		}
	}
	return items
}

// matchBindings finds bindings that apply to a given template file URI.
func matchBindings(uri string, bindings []goanalysis.TemplateBinding) []goanalysis.TemplateBinding {
	var matched []goanalysis.TemplateBinding

	for _, b := range bindings {
		if bindingMatchesURI(b, uri) {
			matched = append(matched, b)
		}
	}
	return matched
}
