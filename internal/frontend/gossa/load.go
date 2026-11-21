// Package gossa loads Go packages and builds go/ssa programs.
//
// This is the only package (together with the Go call graph builder and
// internal/frontend/goir, the Go extractor that turns go/ssa into the neutral
// IR) that is meant to know how go/ssa is constructed. The core (taint, sg,
// sites, report) does not import it; it is the layer under the Go extractor
// (goir), the call graph builder and the command that lists implementations
// (docs/ARCHITECTURE.md, section "Phase 2").
package gossa

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/yagihash/rivus/internal/ir"
)

// Options controls loading.
type Options struct {
	// Tests also loads _test.go files of the matched packages.
	Tests bool
	// NaiveForm builds go/ssa without lifting: local variables stay in
	// memory (Alloc, Store, Load) and there are no φ nodes of its own. This
	// is the form the neutral IR will have (ADR-003); the SSA is then built by
	// the core, and go/ssa's own lifting serves to check it.
	NaiveForm bool
	// WithLifted also builds the program with go/ssa's lifting, for the call
	// graph: variable type analysis is more precise on the lifted form, and
	// the call graph is the one thing that is taken from it.
	WithLifted bool
	// Deps also reads the source of the dependencies that are not in the
	// standard library, so that their functions have bodies and the analysis
	// follows data through them (a function without a body is a guess).
	// The standard library keeps to the models: its bodies are large, full of
	// unsafe and assembly, and the common calls are modelled.
	Deps bool
}

// LoadError records something rivus could not see. It maps to the
// missing_source terminal kind.
type LoadError = ir.LoadError

// Program is a loaded and built Go program.
type Program struct {
	Dir     string
	Fset    *token.FileSet
	SSA     *ssa.Program
	Initial []*packages.Package
	// PkgPaths are the import paths of Initial (which ReleaseSSA lets go of).
	PkgPaths []string

	// Funcs lists every function that has a body, sorted by FuncID.
	Funcs []*ssa.Function
	// Own reports whether a function belongs to one of the loaded packages
	// (as opposed to a dependency or a synthetic wrapper).
	Own map[*ssa.Function]bool

	LoadErrors []LoadError

	// TestFilesSkipped is the number of _test.go files in the directories of the loaded packages that
	// were not read, because Options.Tests did not ask for them (ADR-062).
	TestFilesSkipped int

	// Inputs is what the load read: the files of the packages (the standard
	// library and the dependencies included), the go.mod files of their modules,
	// and the directories of the modules that are not in the module cache.
	Inputs Inputs

	// OwnPkgs are the packages that were loaded from source, as opposed to
	// their dependencies.
	OwnPkgs []*ssa.Package

	// Lifted is the program built with lifting (Options.WithLifted).
	Lifted *ssa.Program

	mu      sync.Mutex           // guards sources and byRel, which fill as they are asked for
	files   map[string]*ast.File // absolute file name -> syntax
	sources map[string][]byte
	byRel   map[string][]*token.File
	byID    map[string]*ssa.Function
}

// Inputs lists the files the Go load read, as absolute paths.
type Inputs struct {
	// Files are the Go files of every package (also those that build
	// constraints left out, which a change of the constraint would bring in),
	// the files of other languages that go with them, and the embedded files.
	Files []string
	// GoMods are the go.mod files of the modules of the packages.
	GoMods []string
	// ModuleDirs are the directories of the modules whose code can change under
	// the user's hands: the modules of the main program, and the replacements
	// that are a directory. The module cache (read only) is not one of them.
	ModuleDirs []string
}

// collectInputs lists what the loaded packages read.
func collectInputs(pkgs []*packages.Package) Inputs {
	files, mods, dirs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, list := range [][]string{pkg.GoFiles, pkg.CompiledGoFiles, pkg.OtherFiles, pkg.EmbedFiles, pkg.IgnoredFiles} {
			for _, f := range list {
				files[f] = true
			}
		}
		if m := pkg.Module; m != nil {
			if m.GoMod != "" {
				mods[m.GoMod] = true
			}
			if m.Main && m.Dir != "" {
				dirs[m.Dir] = true
			}
			if r := m.Replace; r != nil && r.Dir != "" {
				dirs[r.Dir] = true
				if r.GoMod != "" {
					mods[r.GoMod] = true
				}
			}
		}
	})
	keys := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	return Inputs{Files: keys(files), GoMods: keys(mods), ModuleDirs: keys(dirs)}
}

// Rel names a file as the output does: relative to the load directory when it is inside it.
func (p *Program) Rel(name string) string { return p.rel(name) }

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
	if opts.Deps {
		cfg.Mode |= packages.NeedDeps
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
	for _, pkg := range pkgs {
		p.PkgPaths = append(p.PkgPaths, pkg.PkgPath)
	}
	p.Inputs = collectInputs(pkgs)
	if !opts.Tests {
		p.TestFilesSkipped = countTestFiles(pkgs)
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

	// With dependencies read from source, the standard library is not built: the models stand for it.
	// It is told by where it is (GOROOT), not by having no module: a package that is outside a module
	// (a file named on the command line, GOPATH mode) has none either, and it is the code to analyse.
	var skip map[string]bool
	if opts.Deps {
		skip = map[string]bool{}
		initial := map[*packages.Package]bool{}
		for _, pkg := range pkgs {
			initial[pkg] = true
		}
		goroot := goRoot(absDir)
		packages.Visit(pkgs, nil, func(pkg *packages.Package) {
			if pkg.Module == nil && !initial[pkg] && isStdlib(pkg, goroot) {
				skip[pkg.PkgPath] = true
			}
		})
	}
	var mode ssa.BuilderMode = ssa.InstantiateGenerics
	if opts.NaiveForm {
		mode |= ssa.NaiveForm
	}
	mk := ssautil.Packages
	if opts.Deps {
		mk = ssautil.AllPackages
	}
	prog, ssaPkgs := mk(pkgs, mode)
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
	progs := []*ssa.Program{prog}
	if opts.WithLifted {
		lifted, _ := mk(pkgs, ssa.InstantiateGenerics)
		p.Lifted = lifted
		progs = append(progs, lifted)
	}
	// The syntax and the type information of a dependency are of no use once
	// its package is built (go/ssa keeps what it needs): letting go of them
	// package by package keeps the peak low.
	deps := map[string]*packages.Package{}
	if opts.Deps {
		initial := map[*packages.Package]bool{}
		for _, pkg := range pkgs {
			initial[pkg] = true
		}
		packages.Visit(pkgs, nil, func(pkg *packages.Package) {
			if !initial[pkg] {
				deps[pkg.PkgPath] = pkg
			}
		})
	}
	buildInOrder(progs, skip, func(path string) {
		if pkg := deps[path]; pkg != nil {
			pkg.Syntax, pkg.TypesInfo = nil, nil
		}
	})

	own := map[*ssa.Package]bool{}
	for _, sp := range ssaPkgs {
		if sp != nil {
			own[sp] = true
			p.OwnPkgs = append(p.OwnPkgs, sp)
		}
	}
	for fn := range ssautil.AllFunctions(prog) {
		if len(fn.Blocks) == 0 {
			continue
		}
		p.Funcs = append(p.Funcs, fn)
		if pkg := ownerPkg(fn); pkg != nil && own[pkg] {
			p.Own[fn] = true
		}
	}
	// Functions of one name can be several (a wrapper made once for each
	// package that needs it, a generic function and its instances): the
	// order among them must not depend on the order go/ssa made them in, so
	// the bigger body comes first, and then the position.
	size := func(fn *ssa.Function) (blocks, instrs int) {
		for _, b := range fn.Blocks {
			instrs += len(b.Instrs)
		}
		return len(fn.Blocks), instrs
	}
	sort.Slice(p.Funcs, func(i, j int) bool {
		a, b := p.Funcs[i], p.Funcs[j]
		if ia, ib := FuncID(a), FuncID(b); ia != ib {
			return ia < ib
		}
		ab, ai := size(a)
		bb, bi := size(b)
		if ab != bb {
			return ab > bb
		}
		if ai != bi {
			return ai > bi
		}
		if a.Pos() != b.Pos() {
			return a.Pos() < b.Pos()
		}
		// an instance of a generic function after the function itself, then
		// by what each is made of
		if (a.Origin() == nil) != (b.Origin() == nil) {
			return a.Origin() == nil
		}
		if sa, sb := a.Signature.String(), b.Signature.String(); sa != sb {
			return sa < sb
		}
		if len(a.Locals) != len(b.Locals) {
			return len(a.Locals) > len(b.Locals)
		}
		return len(a.AnonFuncs) > len(b.AnonFuncs)
	})
	return p, nil
}

// FuncID returns the stable identifier of a function: its fully qualified
// name, e.g. "example.com/m/pkg.(*T).Method" or "example.com/m/pkg.F$1" for
// an anonymous function.
func FuncID(fn *ssa.Function) string { return fn.String() }

// Position is a source position relative to the load directory.
type Position = ir.Position

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
	// the analysis asks for the text of the code from several goroutines
	p.mu.Lock()
	defer p.mu.Unlock()
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

// ReleaseSSA lets go of the go/ssa programs (and what hangs on them), which
// are large, once the IR has been made and the texts have been taken. What
// is left is what the text of the code and the output need: the syntax, the
// positions, the loading errors.
func (p *Program) ReleaseSSA() {
	p.SSA, p.Lifted = nil, nil
	p.Funcs, p.Own, p.OwnPkgs = nil, nil, nil
	// the loaded packages carry the types and the syntax of every dependency
	p.Initial = nil
	// the loaded packages carry the types and the syntax of every dependency
	p.byID = nil
}

// TokenPos is the inverse of Position: the token.Pos of a byte offset in a
// file given by the name Position uses. It returns token.NoPos for a file it
// does not know.
func (p *Program) TokenPos(file string, offset int) token.Pos {
	return p.TokenPosAt(file, offset, 0)
}

// TokenPosAt is TokenPos for a position whose line is known. The name that Position gives is the one
// after the `//line` directives (and cgo's: the file that is compiled is a generated one, which says by
// directives where it came from), while the offset is of the file that was read: so a name can be
// that of several files, and the line tells which (yagihash/rivus #55).
func (p *Program) TokenPosAt(file string, offset, line int) token.Pos {
	if file == "" {
		return token.NoPos
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.byRel == nil {
		p.byRel = map[string][]*token.File{}
		add := func(name string, f *token.File) {
			for _, g := range p.byRel[name] {
				if g == f {
					return
				}
			}
			p.byRel[name] = append(p.byRel[name], f)
		}
		p.Fset.Iterate(func(f *token.File) bool {
			real := p.rel(f.Name())
			add(real, f)
			// the names that line directives give to the lines of the file
			last := ""
			for l := 1; l <= f.LineCount(); l++ {
				if adj := f.PositionFor(f.LineStart(l), true).Filename; adj != last {
					last = adj
					if r := p.rel(adj); r != real {
						add(r, f)
					}
				}
			}
			return true
		})
		for _, fs := range p.byRel {
			sort.Slice(fs, func(i, j int) bool { return fs[i].Name() < fs[j].Name() })
		}
	}
	var fallback token.Pos
	for _, f := range p.byRel[file] {
		if offset < 0 || offset > f.Size() {
			continue
		}
		pos := f.Pos(offset)
		if line <= 0 || f.PositionFor(pos, true).Line == line {
			return pos
		}
		if fallback == token.NoPos {
			fallback = pos
		}
	}
	return fallback
}

// FuncByID finds a function by its FuncID.
func (p *Program) FuncByID(id string) *ssa.Function {
	if p.byID == nil {
		p.byID = map[string]*ssa.Function{}
		for _, fn := range p.Funcs {
			p.byID[FuncID(fn)] = fn
		}
	}
	return p.byID[id]
}

// buildInOrder builds every package, one after the other, in the order of
// their paths. Program.Build builds them at the same time, and what it makes
// on demand (the instances of generic functions, the wrappers of methods)
// then depends on which package got there first: the same code would not give
// the same IR twice.
func buildInOrder(progs []*ssa.Program, skip map[string]bool, built func(path string)) {
	pkgs := progs[0].AllPackages()
	sort.SliceStable(pkgs, func(i, j int) bool { return pkgs[i].Pkg.Path() < pkgs[j].Pkg.Path() })
	for _, pkg := range pkgs {
		path := pkg.Pkg.Path()
		if skip[path] {
			continue
		}
		pkg.Build()
		for _, prog := range progs[1:] {
			if q := prog.Package(pkg.Pkg); q != nil {
				q.Build()
			}
		}
		built(path)
	}
}

// ownerPkg is the package whose code a function is. go/ssa makes the instance of a generic function or
// of a generic type's method, and the wrappers ($bound, $thunk), without a package: the instance is the
// code of the function it is made from, and a wrapper is the code of the method it wraps, so a sink
// inside either is the program's own (yagihash/rivus #50).
func ownerPkg(fn *ssa.Function) *ssa.Package {
	if fn.Pkg != nil {
		return fn.Pkg
	}
	if o := fn.Origin(); o != nil {
		return ownerPkg(o)
	}
	if fn.Synthetic != "" {
		if obj := fn.Object(); obj != nil && obj.Pkg() != nil {
			if pkg := fn.Prog.ImportedPackage(obj.Pkg().Path()); pkg != nil {
				return pkg
			}
		}
	}
	return nil
}

// goRoot is the GOROOT of the go command that loads the packages.
func goRoot(dir string) string {
	cmd := exec.Command("go", "env", "GOROOT")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		if r := strings.TrimSpace(string(out)); r != "" {
			return filepath.Join(r, "src") + string(filepath.Separator)
		}
	}
	return filepath.Join(runtime.GOROOT(), "src") + string(filepath.Separator)
}

// isStdlib: a package without a module that is in GOROOT (or has no Go file at all, as unsafe has not).
func isStdlib(pkg *packages.Package, gorootSrc string) bool {
	if len(pkg.GoFiles) == 0 && len(pkg.CompiledGoFiles) == 0 {
		return true
	}
	for _, f := range append(append([]string(nil), pkg.GoFiles...), pkg.CompiledGoFiles...) {
		if strings.HasPrefix(f, gorootSrc) {
			return true
		}
	}
	return false
}

// countTestFiles counts the _test.go files in the directories of the packages: what a load without
// Tests leaves out.
func countTestFiles(pkgs []*packages.Package) int {
	dirs := map[string]bool{}
	for _, pkg := range pkgs {
		for _, f := range pkg.GoFiles {
			dirs[filepath.Dir(f)] = true
		}
	}
	n := 0
	for d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.go") {
				n++
			}
		}
	}
	return n
}
