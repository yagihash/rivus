// Package gossa loads Go packages and builds go/ssa programs.
//
// This is the only package (together with the Go call graph builder) that is
// meant to know how go/ssa is constructed. Phase 2 replaces it with an
// extractor that emits the neutral IR.
package gossa

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Options controls loading.
type Options struct {
	// Tests also loads _test.go files of the matched packages.
	Tests bool
}

// LoadError records something rivus could not see. It maps to the
// missing_source terminal kind.
type LoadError struct {
	Package string `json:"package"`
	Message string `json:"message"`
}

// Program is a loaded and built Go program.
type Program struct {
	Dir     string
	Fset    *token.FileSet
	SSA     *ssa.Program
	Initial []*packages.Package

	// Funcs lists every function that has a body, sorted by FuncID.
	Funcs []*ssa.Function
	// Own reports whether a function belongs to one of the loaded packages
	// (as opposed to a dependency or a synthetic wrapper).
	Own map[*ssa.Function]bool

	LoadErrors []LoadError

	files   map[string]*ast.File // absolute file name -> syntax
	sources map[string][]byte
}

// Load loads the packages matching patterns relative to dir.
//
// Load errors in individual packages do not fail the whole load: the
// remaining packages are analysed and the problems are recorded in
// Program.LoadErrors.
func Load(dir string, patterns []string, opts Options) (*Program, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir:   absDir,
		Tests: opts.Tests,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	p := &Program{
		Dir:     absDir,
		Initial: pkgs,
		Own:     map[*ssa.Function]bool{},
		files:   map[string]*ast.File{},
		sources: map[string][]byte{},
	}
	seenErr := map[string]bool{}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, e := range pkg.Errors {
			key := pkg.PkgPath + "\x00" + e.Error()
			if seenErr[key] {
				continue
			}
			seenErr[key] = true
			p.LoadErrors = append(p.LoadErrors, LoadError{Package: pkg.PkgPath, Message: e.Error()})
		}
	})

	var mode ssa.BuilderMode = ssa.InstantiateGenerics
	prog, ssaPkgs := ssautil.Packages(pkgs, mode)
	p.SSA = prog
	if len(pkgs) > 0 {
		p.Fset = pkgs[0].Fset
	} else {
		p.Fset = token.NewFileSet()
	}
	for _, pkg := range pkgs {
		for i, f := range pkg.Syntax {
			if i < len(pkg.CompiledGoFiles) {
				p.files[pkg.CompiledGoFiles[i]] = f
			}
		}
	}
	prog.Build()

	own := map[*ssa.Package]bool{}
	for _, sp := range ssaPkgs {
		if sp != nil {
			own[sp] = true
		}
	}
	for fn := range ssautil.AllFunctions(prog) {
		if len(fn.Blocks) == 0 {
			continue
		}
		p.Funcs = append(p.Funcs, fn)
		if fn.Pkg != nil && own[fn.Pkg] {
			p.Own[fn] = true
		}
	}
	sort.Slice(p.Funcs, func(i, j int) bool { return FuncID(p.Funcs[i]) < FuncID(p.Funcs[j]) })
	return p, nil
}

// FuncID returns the stable identifier of a function: its fully qualified
// name, e.g. "example.com/m/pkg.(*T).Method" or "example.com/m/pkg.F$1" for
// an anonymous function.
func FuncID(fn *ssa.Function) string { return fn.String() }

// Position is a source position relative to the load directory.
type Position struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Offset int    `json:"offset"`
}

func (p Position) String() string { return fmt.Sprintf("%s:%d", p.File, p.Line) }

// Position converts a token.Pos.
func (p *Program) Position(pos token.Pos) Position {
	if !pos.IsValid() {
		return Position{}
	}
	tp := p.Fset.Position(pos)
	return Position{File: p.rel(tp.Filename), Line: tp.Line, Column: tp.Column, Offset: tp.Offset}
}

func (p *Program) rel(name string) string {
	if r, err := filepath.Rel(p.Dir, name); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(name)
}

func (p *Program) source(name string) []byte {
	if b, ok := p.sources[name]; ok {
		return b
	}
	b, _ := os.ReadFile(name)
	p.sources[name] = b
	return b
}

// File returns the syntax tree of the file containing pos, if it was loaded
// from source.
func (p *Program) File(pos token.Pos) *ast.File {
	if !pos.IsValid() {
		return nil
	}
	tf := p.Fset.File(pos)
	if tf == nil {
		return nil
	}
	return p.files[tf.Name()]
}
