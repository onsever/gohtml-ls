package goanalysis

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LoadResult holds type-checked packages from a Go project directory.
type LoadResult struct {
	Fset     *token.FileSet
	Packages map[string]*TypedPackage
}

// TypedPackage holds a type-checked Go package.
type TypedPackage struct {
	Pkg       *types.Package
	Info      *types.Info
	Files     []*ast.File
	FilePaths []string
}

// goListPkg is the subset of `go list -json` output we need.
type goListPkg struct {
	Dir        string   `json:"Dir"`
	ImportPath string   `json:"ImportPath"`
	GoFiles    []string `json:"GoFiles"`
}

// LoadDirectory loads and type-checks all Go packages in dir.
// Returns nil if loading fails (no go.mod, no Go binary, etc.).
func LoadDirectory(dir string) *LoadResult {
	pkgs, err := goList(dir)
	if err != nil || len(pkgs) == 0 {
		return nil
	}

	fset := token.NewFileSet()
	result := &LoadResult{
		Fset:     fset,
		Packages: make(map[string]*TypedPackage),
	}

	// Parse all packages first
	for _, gp := range pkgs {
		tp := &TypedPackage{
			Info: &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Defs:  make(map[*ast.Ident]types.Object),
				Uses:  make(map[*ast.Ident]types.Object),
			},
		}
		for _, name := range gp.GoFiles {
			path := filepath.Join(gp.Dir, name)
			f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				continue
			}
			tp.Files = append(tp.Files, f)
			tp.FilePaths = append(tp.FilePaths, path)
		}
		if len(tp.Files) == 0 {
			continue
		}
		result.Packages[gp.ImportPath] = tp
	}

	// Build importer that lazily type-checks local packages on demand.
	// This ensures dependencies are type-checked before dependents.
	localImporter := &localPkgImporter{
		result:   result,
		fset:     fset,
		fallback: importer.Default(),
	}

	// Type-check each package (the importer handles dependency ordering)
	for impPath := range result.Packages {
		localImporter.ensureChecked(impPath)
	}

	return result
}

type localPkgImporter struct {
	result     *LoadResult
	fset       *token.FileSet
	fallback   types.Importer
	inProgress map[string]bool // cycle detection
}

func (i *localPkgImporter) Import(path string) (*types.Package, error) {
	if tp, ok := i.result.Packages[path]; ok {
		if tp.Pkg != nil {
			return tp.Pkg, nil
		}
		// Type-check this package now (lazy/on-demand)
		return i.ensureChecked(path)
	}
	return i.fallback.Import(path)
}

func (i *localPkgImporter) ensureChecked(path string) (*types.Package, error) {
	tp, ok := i.result.Packages[path]
	if !ok {
		return i.fallback.Import(path)
	}
	if tp.Pkg != nil {
		return tp.Pkg, nil
	}
	// Cycle detection
	if i.inProgress == nil {
		i.inProgress = make(map[string]bool)
	}
	if i.inProgress[path] {
		return nil, fmt.Errorf("import cycle: %s", path)
	}
	i.inProgress[path] = true
	defer delete(i.inProgress, path)

	conf := types.Config{
		Importer: i,
		Error:    func(err error) {},
	}
	pkg, _ := conf.Check(path, i.fset, tp.Files, tp.Info)
	if pkg != nil {
		tp.Pkg = pkg
	}
	return pkg, nil
}

// goList runs `go list -json ./...` in dir and returns package info.
func goList(dir string) ([]goListPkg, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var pkgs []goListPkg
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var p goListPkg
		if err := dec.Decode(&p); err != nil {
			break
		}
		// Skip test files by only using GoFiles (not TestGoFiles)
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}
