package workspace

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// URIToPath converts a file:// URI to a local file path.
func URIToPath(uri string) string {
	u, err := url.Parse(uri)
	if err == nil && u.Scheme == "file" {
		p := u.Path
		if runtime.GOOS == "windows" {
			if strings.HasPrefix(p, "/") && len(p) >= 3 && p[2] == ':' {
				p = p[1:]
			}
			p = strings.ReplaceAll(p, "/", "\\")
		}
		return p
	}
	return uri
}

// PathToURI converts a local file path to a file:// URI.
func PathToURI(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
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
