package workspace

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// URIToPath converts a file:// URI to a local file path.
func URIToPath(uri string) string {
	if strings.HasPrefix(uri, "file:///") {
		p := uri[len("file:///"):]
		p, _ = url.PathUnescape(p)
		// On Windows, paths start with drive letter
		if runtime.GOOS == "windows" {
			p = strings.ReplaceAll(p, "/", "\\")
		} else {
			p = "/" + p
		}
		return p
	}
	if strings.HasPrefix(uri, "file://") {
		p := uri[len("file://"):]
		p, _ = url.PathUnescape(p)
		return p
	}
	return uri
}

// PathToURI converts a local file path to a file:// URI.
func PathToURI(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		return "file:///" + path
	}
	return "file://" + path
}

// IsTemplateFile returns true if the file extension indicates a Go template.
func IsTemplateFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".gohtml", ".tmpl", ".html", ".gotmpl", ".tpl":
		return true
	}
	return false
}

// IsGoFile returns true if the path is a .go file.
func IsGoFile(path string) bool {
	return strings.HasSuffix(path, ".go")
}
