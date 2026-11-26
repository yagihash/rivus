package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"github.com/yagihash/rivus/internal/frontend/accept"
	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irspill"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
	"github.com/yagihash/rivus/internal/vcs"
)

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

// common holds the flags shared by the commands that analyse code.
type common struct {
	dir string
	// dirSet is whether -C was given. The IR of -ir does not come from the
	// current directory, so the directory names no revision unless it is asked.
	dirSet    bool
	rules     multiFlag
	noBuiltin bool
	// noBuiltinSS leaves out the sources and the sinks of the built-in rules; the other sections (propagators, sanitizers and
	// so on) are read. -no-builtin-rules, which leaves out all of them, wins.
	noBuiltinSS bool
	// noProject leaves out the rules found in the analysed directory.
	noProject bool
	tests     bool
	// noDeps leaves out the source of the dependencies (the default reads it, the standard library excepted).
	noDeps bool
	// deterministic leaves out the rules decided by an LLM.
	deterministic bool
	// allowDepsUnder: directories outside the project from which the Dart extractor may read dependencies.
	allowDepsUnder multiFlag
	// depDirs: directories whose Go packages are read as dependencies, although a replace or the module makes them the project (ADR-114).
	depDirs multiFlag
	// bodyReplaced are packages whose body the analysis does not read, besides the ones the rule files declare.
	bodyReplaced multiFlag
	// readDeclared reads the bodies of the packages that are declared (the declarations of the rule files and of
	// -body-replaced are left out): the run that a declaration is checked against.
	readDeclared bool
	// dropAfterRead leaves the declared packages to the filter of the IR (dropReplacedBodies) alone: the extractors read
	// them, as they did before the extractors were told not to (docs/DESIGN-package-models.md 2).
	dropAfterRead bool
	// extractorBodyReplaced are the packages the extractors of Dart and Rust are told not to read (loadLangs sets it).
	extractorBodyReplaced []string
	// declIgnored is how many declarations readDeclared left out (loadRules sets it).
	declIgnored int
	// extractor, if set, makes the IR of the Go code in place of goir.Extract (a test sets it: extractor.go). No flag.
	extractor *accept.Extractor
	// irSpill keeps the functions of the IR in a file and reads them when a reader asks for them (ADR-123). It is on:
	// a test sets it off to compare with the IR in memory. No flag and no variable: the result is the same.
	irSpill bool
}

func (c *common) register(fs *flag.FlagSet) {
	c.dir = "."
	c.irSpill = true
	fs.Func("C", "directory to analyse from (default \".\")", func(v string) error {
		c.dir, c.dirSet = v, true
		return nil
	})
	fs.Var(&c.rules, "rules", "additional rule file (repeatable)")
	fs.BoolVar(&c.noBuiltin, "no-builtin-rules", false, "do not load the built-in rules")
	fs.BoolVar(&c.noBuiltinSS, "no-builtin-sources-sinks", false, "do not load the sources and the sinks of the built-in rules, so that only the ones of your own rule files are checked; the other sections of the built-in rules (propagators, sanitizers and so on) are still read. Without the sinks, -categories knows only the categories of your sinks; the built-in sanitizers drop the labels of the built-in sources (user_input and so on), so a source of yours with another label needs a sanitizer of yours. -no-builtin-rules, if given too, wins")
	fs.BoolVar(&c.noProject, "no-project-rules", false, "do not load the rules found in <dir>/.rivus/rules/*.yaml")
	fs.BoolVar(&c.tests, "tests", false, "also read what the build does not ship: tests (Go: _test.go files; Dart: test/ and the dependencies declared for development only; Rust: tests, examples, benches and [dev-dependencies])")
	fs.BoolVar(&c.noDeps, "no-deps", false, "do not read the source of the dependencies (which is done by default, except for the standard library, so that calls into them are followed instead of guessed): lighter (about a sixth of the memory), and every call into a dependency is a guess")
	fs.Var(&c.allowDepsUnder, "allow-deps-under", "Dart: a directory outside the project whose packages may be read as dependencies (repeatable). The package_config.json of the project says where its dependencies are, and it is not trusted: only the project, the pub cache and the SDKs are read without being asked (a monorepo's sibling packages need this)")
	fs.Var(&c.depDirs, "dep-dir", "Go: a directory (relative to -C, or absolute) whose packages are read as dependencies (repeatable): their sources and sinks are not listed. Without it, the packages of the main module, of go.work and of a module that a replace points to by a directory are the project (a third-party copy that is put in place by a replace is the case for it). It is in summary.dep_dirs")
	fs.BoolVar(&c.deterministic, "deterministic-only", false, "ignore the rules decided by an LLM (origin: llm)")
	fs.Var(&c.bodyReplaced, "body-replaced", "PACKAGE: do not read the body of this package of the dependencies; the models stand for it (a call of its functions is the model's row, or unmodeled_library_call when there is none). The import path of a Go package, package:name of a Dart package, a Rust crate; the package named and no other, `PATH/...` for it and what is under it. For a try: the rule files declare it with body_replaced_packages, with the reason (repeatable; docs/MODELING.md)")
	fs.BoolVar(&c.dropAfterRead, "drop-after-read", false, "read the packages that are declared as the extractors do without the declaration, and leave their functions out of the program afterwards (the result is the same as when the extractor does not read them; it takes more time and memory; to compare the two) (development)")
	fs.BoolVar(&c.readDeclared, "read-declared-bodies", false, "read the body of the packages that the rule files and -body-replaced declare, as if there were no declaration: to compare the result with the one that leaves them out (summary.body_replaced_ignored says how many were left out)")
	usual(fs, "C", "rules", "no-builtin-rules", "no-builtin-sources-sinks", "no-project-rules", "tests", "no-deps", "allow-deps-under", "dep-dir", "deterministic-only", "body-replaced", "read-declared-bodies")
	diagnostic(fs, "drop-after-read")
	stop := "stops a function that is on by default"
	for _, n := range []string{"no-builtin-rules", "no-builtin-sources-sinks", "no-project-rules", "no-deps", "deterministic-only"} {
		describe(fs, n, flagInfo{Not: stop})
	}
	describe(fs, "tests", flagInfo{Not: "the range of the analysis"})
	describe(fs, "body-replaced", flagInfo{Not: "a way to try: the declaration is decided by package in the rule files"})
	describe(fs, "read-declared-bodies", flagInfo{Not: "a way to try: reads what the declaration leaves out, to compare"})
	kind(fs, "dir", "C", "allow-deps-under", "dep-dir")
	kind(fs, "file", "rules")
}

// projectRuleFiles lists the rule files the analysed directory brings: <dir>/.rivus/rules/*.yaml, in name order.
func (c *common) projectRuleFiles() ([]string, error) {
	// Not filepath.Glob: the directory is a name, and may hold the characters of a pattern.
	rulesDir := filepath.Join(c.dir, ".rivus", "rules")
	if err := insideDir(c.dir, rulesDir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(rulesDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
		return nil, fmt.Errorf("project rules: %w", err)
	}
	var found []string
	for _, e := range entries {
		if ext := filepath.Ext(e.Name()); !e.IsDir() && (ext == ".yaml" || ext == ".yml") {
			found = append(found, filepath.Join(rulesDir, e.Name()))
		}
	}
	sort.Strings(found)
	return found, nil
}

func (c *common) loadRules(extra ...*model.RuleSet) (*model.Rules, error) {
	var sets []*model.RuleSet
	if !c.noBuiltin {
		b, err := builtin.Load()
		if err != nil {
			return nil, err
		}
		if c.noBuiltinSS {
			for i, rs := range b {
				b[i] = rs.WithoutSourcesSinks()
			}
			fmt.Fprintln(os.Stderr, "rivus: the sources and the sinks of the built-in rules are not loaded (-no-builtin-sources-sinks); the other sections are")
		}
		sets = append(sets, b...)
	}
	// The rules of the project: the sources, sinks and sanitizers of its own
	// code, and the models of the libraries it uses that the built-in ones do
	// not cover. They live with the code and are read without a flag.
	paths := append([]string(nil), c.rules...)
	project := map[string]bool{}
	if !c.noProject {
		found, err := c.projectRuleFiles()
		if err != nil {
			return nil, err
		}
		for _, p := range found {
			fmt.Fprintf(os.Stderr, "rivus: project rules: %s\n", p)
			project[p] = true
		}
		paths = append(found, paths...)
	}
	for _, path := range paths {
		// A rule file that the analysed directory brings is not read through a link: it would be the file
		// of another place (its content shows in the message of an error, and is read as rules).
		if project[path] {
			if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("%s: a project rule file is a symbolic link, and is not read (-no-project-rules leaves the project's rules out)", path)
			}
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		rs, err := model.Parse(f, path)
		f.Close()
		if err != nil {
			if project[path] {
				// Not the message of the decoder: it quotes keys and values of the file (#69).
				err = projectRuleError(path, err)
			}
			return nil, err
		}
		rs.Project = project[path]
		sets = append(sets, rs)
	}
	sets = append(sets, extra...)
	flagSet, err := c.bodyReplacedSet()
	if err != nil {
		return nil, err
	}
	if flagSet != nil {
		sets = append(sets, flagSet)
	}
	var rules *model.Rules
	if c.deterministic {
		rules = model.WithoutOrigin(model.OriginLLM, sets...)
	} else {
		rules = model.Merge(sets...)
	}
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	if c.readDeclared {
		c.declIgnored = len(rules.Declared)
		rules = rules.WithDeclarations(nil)
	}
	for _, w := range rules.CallWarnings() {
		fmt.Fprintf(os.Stderr, "rivus: warning: %s\n", w)
	}
	return rules, nil
}

// load reads the code: the Go packages, and the neutral IR made of them.
// withGraph also builds what the call graph needs (the analysis does; the
// listing of sites does not).
func (c *common) load(patterns []string, withGraph bool, rules *model.Rules) (*loaded, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	opts := gossa.Options{Tests: c.tests, NaiveForm: true, WithLifted: withGraph, Deps: !c.noDeps && withGraph, DepDirs: c.depDirs}
	if withGraph {
		opts.OnLifted = goir.LiftedTable
	}
	if rules.HasDeclared() && !c.dropAfterRead {
		// the extractor does not read what the models stand for (docs/DESIGN-package-models.md 2): the IR is the one
		// that dropReplacedBodies would make
		opts.ReplacedPackage = rules.ReplacedBody
		opts.ReplacedFunc = func(id string) bool { return rules.ReplacedBody(taint.UnitOf(id)) }
	}
	p, err := gossa.Load(c.dir, patterns, opts)
	if err != nil {
		return nil, err
	}
	return &loaded{gossa: p}, nil
}

// loadGo reads the Go packages the way every command that lists sites or
// analyses does: the rules that name an interface method (match:
// implementations) are expanded against the program, then the IR is made.
// "sites" and "analyze -sites" list the same sites because both come here.
func (c *common) loadGo(patterns []string, rules *model.Rules, maxImpl int) (*loaded, error) {
	l, err := c.load(patterns, true, rules)
	if err != nil {
		return nil, err
	}
	// the rows of a module that name it by /v* (any major) are made for the majors the program has
	rules.ExpandMajors(l.gossa.PackagePaths())
	for _, e := range rules.Expand(l.gossa, maxImpl) {
		if e.Limited || e.Note != "" {
			fmt.Fprintf(os.Stderr, "rivus: %s\n", e)
		}
	}
	// the expansion copies the rules to the methods: what is wrong in the merged rules shows only now
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	if err := c.extractGo(l, patterns, rules); err != nil {
		return nil, err
	}
	// what the extractor was told not to read is counted for the report (dropReplacedBodies)
	if l.gossa != nil {
		units := make([]string, 0, len(l.gossa.BodiesNotRead))
		for u := range l.gossa.BodiesNotRead {
			units = append(units, u)
		}
		sort.Strings(units)
		for _, u := range units {
			l.ir.BodiesNotRead = append(l.ir.BodiesNotRead, ir.BodyNotRead{Unit: u, Functions: l.gossa.BodiesNotRead[u]})
		}
	}
	return l, nil
}

// loaded is the code read from Go packages.
type loaded struct {
	gossa *gossa.Program
	// ir is the program; when src is set, it is the header of it (its Funcs are not there)
	ir    *ir.Program
	texts ir.Texts
	// src is where the functions are when they are not in ir.Funcs (nil: they are)
	src ir.Source
	// disk is src when it keeps the functions in a file (nil: it does not); the part of a program that has other
	// languages too is in src, which then has them all
	disk *irspill.DiskSource
	// moreDisks are the files of the other languages of a program that has more than one (-verbose says what they did)
	moreDisks []*irspill.DiskSource
}

// source is the Source of the functions of the program.
func (l *loaded) source() ir.Source {
	if l.src != nil {
		return l.src
	}
	return ir.MemSource(l.ir)
}

// readIR reads a stream of the IR written by an extractor. The texts come from
// the stream too.
func readIR(path string) (*loaded, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	prog, err := ir.Read(bufio.NewReaderSize(f, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%s: %w%s", path, err, rebuildHint("", err))
	}
	// an extractor may write the functions of the dependencies (marked dep):
	// the ones nothing of the project reaches change nothing
	prog.Files = prog.SourceFiles()
	prog.PruneDeps()
	return &loaded{ir: prog, texts: ir.NewStreamTexts(prog)}, nil
}

// expandSubtypes makes the rules that speak of the subtypes of a class
// (extends:) from the class hierarchy the extractor wrote, if there is one.
func expandSubtypes(rules *model.Rules, src ir.Source) {
	if src == nil || len(src.Header().Types) == 0 {
		return
	}
	rules.ExpandSubtypes(subtypeResolver{src})
}

type subtypeResolver struct{ src ir.Source }

func (r subtypeResolver) SubtypeMethods(super, with string) []model.MarkedMethod {
	var out []model.MarkedMethod
	for _, m := range ir.SubtypeMethods(r.src, ir.TypeID(super), ir.TypeID(with)) {
		out = append(out, model.MarkedMethod{Symbol: m.Symbol, Params: m.Params})
	}
	return out
}

func (r subtypeResolver) CallsFunction(symbol string) bool { return ir.CallsFunction(r.src, symbol) }

// testsSkipped is what was left out because the build does not ship it (ADR-062): the _test.go files, and what an
// extractor said it did not read. Nil when nothing was.
func (l *loaded) testsSkipped() *ir.TestsSkipped {
	var t *ir.TestsSkipped
	if l.ir != nil {
		t = t.Add(l.ir.TestsSkipped)
	}
	if l.gossa != nil && l.gossa.TestFilesSkipped > 0 {
		t = t.Add(&ir.TestsSkipped{Files: l.gossa.TestFilesSkipped})
	}
	if l.gossa != nil && l.gossa.IgnoredFiles > 0 {
		t = t.Add(&ir.TestsSkipped{OutsideBuildFiles: l.gossa.IgnoredFiles, OutsideBuildFileNames: l.gossa.IgnoredFileNames})
	}
	if t.Empty() && (t == nil || (len(t.Notes) == 0 && t.SkippedDirs == 0 && t.OutsideBuildFiles == 0)) {
		return nil
	}
	return t
}

// loadErrors are what could not be read, from the packages or the stream.
func (l *loaded) loadErrors() []ir.LoadError {
	if l.gossa == nil {
		return l.ir.LoadErrors
	}
	// With several languages, the errors of the Go packages are also in the
	// program (goir.AddTexts copies them): each is counted once (#13).
	out := append([]ir.LoadError(nil), l.gossa.LoadErrors...)
	copied := map[ir.LoadError]int{}
	for _, e := range l.gossa.LoadErrors {
		copied[e]++
	}
	for _, e := range l.ir.LoadErrors {
		if copied[e] > 0 {
			copied[e]--
			continue
		}
		out = append(out, e)
	}
	return out
}

// packages lists the units that were analysed: the import paths of the Go
// packages, and the units (packages, crates) of the functions of an IR that
// did not come from Go packages.
func (l *loaded) packages(src ir.Source) []string {
	seen := map[string]bool{}
	var out []string
	add := func(u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	if l.gossa != nil {
		for _, p := range l.gossa.PkgPaths {
			add(p)
		}
	}
	if l.ir != nil {
		for i, n := 0, src.Len(); i < n; i++ {
			h := src.Head(i)
			if h.Gone {
				continue
			}
			id := string(h.ID)
			// the Go functions of a program that has Go packages are listed by them
			if l.gossa != nil && !report.ForeignID(id) {
				continue
			}
			add(report.UnitOf(id))
		}
	}
	return out
}

// extract makes the IR. The analysis needs every function with a body, the
// dependencies included, and the call graph; the listing of sites needs the
// functions of the loaded packages only.
func (l *loaded) extract(withGraph, spill bool) error {
	opts := goir.Options{All: withGraph, NoCandidates: !withGraph}
	var b *irspill.Builder
	if spill {
		// each function goes to the file when it is converted: the program does not hold it
		b = newSpillBuilder()
		opts.Emit = b.Add
	}
	prog, err := goir.Extract(l.gossa, opts)
	if err != nil {
		return err
	}
	l.ir = prog
	if b != nil {
		b.SortByID() // the order of the program that Extract returns
		l.disk = b.Source(prog)
		l.src = l.disk
	}
	l.texts = goir.NewTexts(l.gossa)
	// The go/ssa programs are the largest thing in memory, and nothing needs
	// them from here on.
	l.gossa.ReleaseSSA()
	return nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// target is the revision of the code that was analysed. For an IR stream the
// code is not in the directory the command runs in, so a revision is recorded
// only when -C says where it is (#12).
func (c *common) target(fromIR bool) *vcs.Target {
	if fromIR && !c.dirSet {
		return nil
	}
	return vcs.Detect(c.dir)
}

// insideDir fails when rulesDir, with every link in its path followed, is outside dir: the directory of the
// rules that the analysed code brings must not be another place (yagihash/rivus #69).
func insideDir(dir, rulesDir string) error {
	real, err := filepath.EvalSymlinks(rulesDir)
	if err != nil {
		return nil // not there: nothing is read
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s: the project rules directory is outside the analysed directory (a symbolic link), and is not read (-no-project-rules leaves the project's rules out)", rulesDir)
	}
	return nil
}

var ruleErrLine = regexp.MustCompile(`line (\d+)`)

// projectRuleError is the error of a rule file that the analysed directory brings: the file and the line, and
// not what the parser says of the content.
func projectRuleError(path string, err error) error {
	if m := ruleErrLine.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Errorf("%s: the project rule file is not valid (line %s; run it with -rules to see why)", path, m[1])
	}
	return fmt.Errorf("%s: the project rule file is not valid (run it with -rules to see why)", path)
}
