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
	// OnLifted, with WithLifted, is given the lifted program as soon as it is
	// built, and its answer is kept in Program.LiftedResult; the lifted
	// program is then let go of. The lifted program is built and read first,
	// before the program of the naive form is built, so the two are never in
	// memory together. Without it the lifted program is kept in
	// Program.Lifted.
	OnLifted func(*ssa.Program) any
	// Deps also reads the source of the dependencies that are not in the
	// standard library, so that their functions have bodies and the analysis
	// follows data through them (a function without a body is a guess).
	// The standard library keeps to the models: its bodies are large, full of
	// unsafe and assembly, and the common calls are modelled.
	Deps bool
	// ReplacedPackage reports whether the models stand for the body of a dependency's package (a declaration,
	// docs/DESIGN-package-models.md): its syntax and type information are let go of before go/ssa is built, so the
	// package has functions without a body, as with no source. The packages that were asked for are always read.
	// It acts only with Deps. nil: none.
	ReplacedPackage func(path string) bool
	// ReplacedFunc reports whether a function of a dependency (by FuncID) is one of a declared package. The wrappers
	// that go/ssa makes for a type of the package in another package have a body of their own and carry the ID of
	// the package: they are left out of Funcs too, as dropReplacedBodies leaves them out of the IR.
	ReplacedFunc func(id string) bool
	// DepDirs are directories (relative to the load directory, or absolute) whose packages are read as dependencies
	// although they belong to the project by the rules of ADR-114 (a module that a replace points to by a directory, a
	// package of the main module). The packages that the patterns name are not left out.
	DepDirs []string
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
	// IgnoredFiles are the files of the loaded packages that the build constraints leave out of this configuration (up
	// to 5 names in IgnoredFileNames): code that is not in the analysis, and is not a test.
	IgnoredFiles     int
	IgnoredFileNames []string

	// Inputs is what the load read: the files of the packages (the standard
	// library and the dependencies included), the go.mod files of their modules,
	// and the directories of the modules that are not in the module cache.
	Inputs Inputs

	// ModuleVersions are the versions of the modules that the program uses, by module path (from go.mod and go.sum, as `go list`
	// says): the version the build is made with. A module that has no version is left out: the main module, a replacement by a
	// directory, a replacement by another module.
	ModuleVersions map[string]string

	// OwnPkgs are the packages that were loaded from source, as opposed to
	// their dependencies.
	OwnPkgs []*ssa.Package

	// Lifted is the program built with lifting (Options.WithLifted).
	Lifted *ssa.Program
	// LiftedResult is what Options.OnLifted answered.
	LiftedResult any

	// BodiesNotRead is, per package whose body Options.ReplacedPackage said the models replace, how many functions
	// and methods it declares that were left without a body.
	BodiesNotRead map[string]int

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

// projectPackages returns the import paths of the loaded packages (the packages asked for and everything they import)
// that belong to the project (ADR-114): the packages asked for, the packages of the main module(s) of the program (go
// list marks a module as main when it is the module of the directory, and every module that a go.work lists), and the
// packages of a module that a replace directive points to by a directory (a monorepo that links its modules so; a
// replace to a version, which is a module of the module cache, is a dependency). depDirs name directories whose
// packages are read as dependencies all the same (a third-party copy that is put in place by a replace, a skeleton): the
// packages that were asked for are not left out by them.
func projectPackages(dir string, pkgs []*packages.Package, depDirs []string) map[string]bool {
	var under []string
	for _, d := range depDirs {
		if !filepath.IsAbs(d) {
			d = filepath.Join(dir, d)
		}
		if r, err := filepath.EvalSymlinks(d); err == nil {
			d = r
		}
		under = append(under, filepath.Clean(d))
	}
	inDepDir := func(pkg *packages.Package) bool {
		if len(under) == 0 || len(pkg.GoFiles) == 0 {
			return false
		}
		f := filepath.Dir(pkg.GoFiles[0])
		if r, err := filepath.EvalSymlinks(f); err == nil {
			f = r
		}
		for _, u := range under {
			if f == u || strings.HasPrefix(f, u+string(filepath.Separator)) {
				return true
			}
		}
		return false
	}
	out := map[string]bool{}
	for _, pkg := range pkgs {
		out[pkg.PkgPath] = true
	}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		m := pkg.Module
		if m == nil || out[pkg.PkgPath] {
			return
		}
		if (m.Main || (m.Replace != nil && m.Replace.Version == "" && m.Replace.Dir != "")) && !inDepDir(pkg) {
			out[pkg.PkgPath] = true
		}
	})
	return out
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
	pkgs, err := packages.Load(cfg, shakeOrder(cfg, patterns)...)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}
	shakePackages(pkgs)

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
	p.ModuleVersions = moduleVersions(pkgs)
	if !opts.Tests {
		p.TestFilesSkipped = countTestFiles(pkgs)
	}
	// the files that the build constraints leave out of the packages that were asked for (not the dependencies: their
	// constraints are the library's own), by the path under the directory when there is one
	for _, pkg := range pkgs {
		for _, f := range pkg.IgnoredFiles {
			p.IgnoredFiles++
			if len(p.IgnoredFileNames) < 5 {
				name := f
				if rel, err := filepath.Rel(absDir, f); err == nil && !strings.HasPrefix(rel, "..") {
					name = filepath.ToSlash(rel)
				}
				p.IgnoredFileNames = append(p.IgnoredFileNames, name)
			}
		}
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
	// ADR-114: the project is the packages asked for and every package of the same main module(s)
	projectPkgs := projectPackages(absDir, pkgs, opts.DepDirs)
	mk := ssautil.Packages
	if opts.Deps {
		mk = ssautil.AllPackages
		if opts.ReplacedPackage != nil {
			initial := map[*packages.Package]bool{}
			for _, pkg := range pkgs {
				initial[pkg] = true
			}
			packages.Visit(pkgs, nil, func(pkg *packages.Package) {
				if !initial[pkg] && !skip[pkg.PkgPath] && opts.ReplacedPackage(pkg.PkgPath) {
					pkg.Syntax, pkg.TypesInfo = nil, nil
				}
			})
		}
	}
	prog, ssaPkgs := mk(pkgs, mode)
	p.SSA = prog
	p.addUnread(pkgs, ssaPkgs)
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
	if opts.WithLifted && opts.OnLifted != nil {
		// The lifted program is built and read before the program of the
		// naive form is: what is taken from it is small, and it is gone
		// before the other grows.
		lifted, _ := mk(pkgs, ssa.InstantiateGenerics)
		buildInOrder([]*ssa.Program{lifted}, skip, nil)
		p.LiftedResult = opts.OnLifted(lifted)
	} else if opts.WithLifted {
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
	// the other packages of the project that were loaded (the pattern only narrows what is asked for)
	// (AllPackages walks a map: in the order of the paths, so that nothing depends on it)
	all := prog.AllPackages()
	sort.Slice(all, func(i, j int) bool { return all[i].Pkg.Path() < all[j].Pkg.Path() })
	for _, sp := range all {
		if sp != nil && !own[sp] && projectPkgs[sp.Pkg.Path()] {
			own[sp] = true
			p.OwnPkgs = append(p.OwnPkgs, sp)
		}
	}
	for fn := range allFunctions(prog) {
		if len(fn.Blocks) == 0 {
			// a declared function of a package that was not read: the count that the report says
			if opts.Deps && opts.ReplacedFunc != nil && fn.Object() != nil && (fn.Synthetic == "" || fn.Synthetic == noSyntax || fn.Synthetic == noSyntaxOld) && fn.Origin() == nil {
				if pkg := ownerPkg(fn); pkg != nil && !own[pkg] && !skip[pkg.Pkg.Path()] && opts.ReplacedPackage != nil && opts.ReplacedPackage(pkg.Pkg.Path()) {
					if p.BodiesNotRead == nil {
						p.BodiesNotRead = map[string]int{}
					}
					p.BodiesNotRead[pkg.Pkg.Path()]++
				}
			}
			continue
		}
		isOwn := false
		if pkg := ownerPkg(fn); pkg != nil && own[pkg] {
			isOwn = true
		}
		if !isOwn && opts.Deps && opts.ReplacedFunc != nil && opts.ReplacedFunc(FuncID(fn)) {
			continue
		}
		p.Funcs = append(p.Funcs, fn)
		if isOwn {
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
		// by the place in the file, not by token.Pos (which depends on the order the files were read into the file set)
		if pa, pb := p.Position(a.Pos()), p.Position(b.Pos()); pa != pb {
			if pa.File != pb.File {
				return pa.File < pb.File
			}
			return pa.Offset < pb.Offset
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

// noSyntax is what go/ssa says (Function.Synthetic) of a declared function that it was given no syntax for.
const (
	noSyntax    = "from type information"
	noSyntaxOld = "loaded from gc object file" // the older word for it
)

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
	p.SSA, p.Lifted, p.LiftedResult = nil, nil, nil
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
		if built != nil {
			built(path)
		}
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

// moduleVersions are the versions of the modules of the packages and of what they import.
func moduleVersions(pkgs []*packages.Package) map[string]string {
	out := map[string]string{}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		m := pkg.Module
		if m == nil || m.Main {
			return
		}
		v := m.Version
		if r := m.Replace; r != nil {
			// a replacement by a module of the same path at a version says the version; a directory, or another module's code, does not
			if r.Path != m.Path || r.Version == "" {
				return
			}
			v = r.Version
		}
		if v != "" {
			out[m.Path] = v
		}
	})
	return out
}

// allFunctions is ssautil.AllFunctions, closed: the functions that have a
// body are the same whatever order they are looked at in.
//
// Asking Origin of a function inside an instance of a generic function (for
// example sync.OnceValues[string, error]$1$1) makes, late, the body of the
// generic function, and that makes the bodies of the methods it calls (the
// methods of sync.Pool, RWMutex). So a loop over a map of functions that
// calls Origin (ownerPkg does) saw a function without a body when it came
// before, and with one when it came after: the functions given to the
// engine changed from run to run (40781 to 40791 for memos), by about 150 to
// 200 functions of the standard library. This asks Origin of all functions
// and takes the set again, until the number of functions that have a body and
// the number of all the functions do not change (ADR-118, (ii): the same set whatever the order and the
// machine).
//
// The standard library is in skip, "its bodies are not read", but this is
// what happens in fact: go/ssa makes late the bodies of generic instances and
// of the methods of types, so some functions of the standard library are
// analyzed. Whether summary should show it is to be decided later.
func allFunctions(prog *ssa.Program) map[*ssa.Function]bool {
	count := func(m map[*ssa.Function]bool) int {
		n := 0
		for fn := range m {
			if len(fn.Blocks) > 0 {
				n++
			}
		}
		return n
	}
	fns := ssautil.AllFunctions(prog)
	n, total := count(fns), len(fns)
	for {
		for fn := range fns {
			fn.Origin()
		}
		fns = ssautil.AllFunctions(prog)
		// both the functions with a body and all the functions must stop growing: a new function
		// without a body has not had Origin asked yet, and the loop of Load would be the first to ask
		m, t := count(fns), len(fns)
		if m == n && t == total {
			return fns
		}
		n, total = m, t
	}
}
