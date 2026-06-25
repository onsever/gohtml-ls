package features

import (
	"path/filepath"
	"strings"

	"github.com/onsever/gohtml-ls/goanalysis"
	"github.com/onsever/gohtml-ls/workspace"
)

func bindingMatchesURI(binding goanalysis.TemplateBinding, uri string) bool {
	path := filepath.ToSlash(filepath.Clean(workspace.URIToPath(uri)))
	baseName := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	if binding.TemplateName == baseName || binding.TemplateName == nameWithoutExt {
		return true
	}
	for _, parsedFile := range binding.ParsedFiles {
		if parsedFileMatchesPath(parsedFile, path, baseName) {
			return true
		}
	}
	return false
}

func parsedFileMatchesPath(parsedFile, path, baseName string) bool {
	pattern := filepath.ToSlash(filepath.Clean(parsedFile))
	if pattern == "." {
		return false
	}
	if pattern == baseName || path == pattern || strings.HasSuffix(path, "/"+pattern) {
		return true
	}
	if !strings.ContainsAny(pattern, "*?[") {
		return false
	}
	if ok, _ := filepath.Match(pattern, baseName); ok {
		return true
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i := range parts {
		candidate := strings.Join(parts[i:], "/")
		if ok, _ := filepath.Match(pattern, candidate); ok {
			return true
		}
	}
	return false
}
