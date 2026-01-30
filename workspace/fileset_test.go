package workspace

import (
	"runtime"
	"strings"
	"testing"
)

func TestURIToPath_Windows(t *testing.T) {
	result := URIToPath("file:///C:/Users/test/file.go")
	if runtime.GOOS == "windows" {
		if !strings.Contains(result, "C:") || !strings.Contains(result, "file.go") {
			t.Errorf("expected path containing C: and file.go, got %q", result)
		}
	} else {
		if result != "/C:/Users/test/file.go" {
			t.Errorf("expected /C:/Users/test/file.go, got %q", result)
		}
	}
}

func TestURIToPath_Linux(t *testing.T) {
	result := URIToPath("file:///home/user/file.go")
	if runtime.GOOS == "windows" {
		if !strings.Contains(result, "home") && !strings.Contains(result, "file.go") {
			t.Errorf("expected path containing home and file.go, got %q", result)
		}
	} else {
		if result != "/home/user/file.go" {
			t.Errorf("expected /home/user/file.go, got %q", result)
		}
	}
}

func TestURIToPath_PlainPath(t *testing.T) {
	result := URIToPath("/some/plain/path.go")
	if result != "/some/plain/path.go" {
		t.Errorf("expected /some/plain/path.go, got %q", result)
	}
}

func TestPathToURI(t *testing.T) {
	uri := PathToURI("/home/user/file.go")
	if !strings.HasPrefix(uri, "file://") {
		t.Errorf("expected file:// prefix, got %q", uri)
	}
	if !strings.Contains(uri, "home/user/file.go") {
		t.Errorf("expected URI to contain path, got %q", uri)
	}
}

func TestIsTemplateFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"foo.gohtml", true},
		{"foo.tmpl", true},
		{"foo.html", true},
		{"foo.gotmpl", true},
		{"foo.tpl", true},
		{"foo.go", false},
		{"foo.txt", false},
	}
	for _, tc := range tests {
		if got := IsTemplateFile(tc.path); got != tc.want {
			t.Errorf("IsTemplateFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsGoFile(t *testing.T) {
	if !IsGoFile("main.go") {
		t.Error("expected main.go to be a Go file")
	}
	if IsGoFile("main.js") {
		t.Error("expected main.js to not be a Go file")
	}
}
