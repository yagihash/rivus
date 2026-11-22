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

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
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
	// noProject leaves out the rules found in the analysed directory.
	noProject bool
	tests     bool
	// noDeps leaves out the source of the dependencies (the default reads it, the standard library excepted).
	noDeps bool
	// deterministic leaves out the rules decided by an LLM.
	deterministic bool
	// allowDepsUnder: directories outside the project from which the Dart extractor may read dependencies.
	allowDepsUnder multiFlag
	// bodyReplaced are packages whose body the analysis does not read, besides the ones the rule files declare.
	bodyReplaced multiFlag
	// readDeclared reads the bodies of the packages that are declared (the declarations of the rule files and of
	// -body-replaced are left out): the run that a declaration is checked against.
	readDeclared bool
	// dropAfterRead leaves the declared packages to the filter of the IR (dropReplacedBodies) alone: the extractors read
	// them, as they did before the extractors were told not to (docs/DESIGN-package-models.md 2).
	dropAfterRead bool
	// extractorBodyReplaced are the packages the extractor of Dart is told not to read (loadLangs sets it).
	extractorBodyReplaced []string
	// declIgnored is how many declarations readDeclared left out (loadRules sets it).
	declIgnored int
}

func (c *common) register(fs *flag.FlagSet) {
	c.dir = "."
	fs.Func("C", "directory to analyse from (default \".\")", func(v string) error {
		c.dir, c.dirSet = v, true
		return nil
	})
	fs.Var(&c.rules, "rules", "additional rule file (repeatable)")
	fs.BoolVar(&c.noBuiltin, "no-builtin-rules", false, "do not load the built-in rules")
	fs.BoolVar(&c.noProject, "no-project-rules", false, "do not load the rules found in <dir>/.rivus/rules/*.yaml")
	fs.BoolVar(&c.tests, "tests", false, "also read what the build does not ship: tests (Go: _test.go files; Dart: test/ and the dependencies declared for development only; Rust: tests, examples, benches and [dev-dependencies])")
	fs.BoolVar(&c.noDeps, "no-deps", false, "do not read the source of the dependencies (which is done by default, except for the standard library, so that calls into them are followed instead of guessed): lighter (about a sixth of the memory), and every call into a dependency is a guess")
	fs.Var(&c.allowDepsUnder, "allow-deps-under", "Dart: a directory outside the project whose packages may be read as dependencies (repeatable). The package_config.json of the project says where its dependencies are, and it is not trusted: only the project, the pub cache and the SDKs are read without being asked (a monorepo's sibling packages need this)")
	fs.BoolVar(&c.deterministic, "deterministic-only", false, "ignore the rules decided by an LLM (origin: llm)")
	fs.Var(&c.bodyReplaced, "body-replaced", "PACKAGE: do not read the body of this package of the dependencies; the models stand for it (a call of its functions is the model's row, or unmodeled_library_call when there is none). The import path of a Go package, package:name of a Dart package, a Rust crate; the package named and no other, `PATH/...` for it and what is under it. For a try: the rule files declare it with body_replaced_packages, with the reason (repeatable; docs/MODELING.md)")
	fs.BoolVar(&c.dropAfterRead, "drop-after-read", false, "read the packages that are declared as the extractors do without the declaration, and leave their functions out of the program afterwards (the result is the same as when the extractor does not read them; it takes more time and memory; to compare the two) (development)")
	fs.BoolVar(&c.readDeclared, "read-declared-bodies", false, "read the body of the packages that the rule files and -body-replaced declare, as if there were no declaration: to compare the result with the one that leaves them out (summary.body_replaced_ignored says how many were left out)")
	usual(fs, "C", "rules", "no-builtin-rules", "no-project-rules", "tests", "no-deps", "allow-deps-under", "deterministic-only", "body-replaced", "read-declared-bodies")
	diagnostic(fs, "drop-after-read")
	kind(fs, "dir", "C", "allow-deps-under")
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
	opts := gossa.Options{Tests: c.tests, NaiveForm: true, WithLifted: withGraph, Deps: !c.noDeps && withGraph}
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
	for _, e := range rules.Expand(l.gossa, maxImpl) {
		if e.Limited || e.Note != "" {
			fmt.Fprintf(os.Stderr, "rivus: %s\n", e)
		}
	}
	// the expansion copies the rules to the methods: what is wrong in the merged rules shows only now
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	if err := l.extract(true); err != nil {
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
	ir    *ir.Program
	texts ir.Texts
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
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// an extractor may write the functions of the dependencies (marked dep):
	// the ones nothing of the project reaches change nothing
	prog.Files = prog.SourceFiles()
	prog.PruneDeps()
	return &loaded{ir: prog, texts: ir.NewStreamTexts(prog)}, nil
}

// expandSubtypes makes the rules that speak of the subtypes of a class
// (extends:) from the class hierarchy the extractor wrote, if there is one.
func expandSubtypes(rules *model.Rules, prog *ir.Program) {
	if prog == nil || len(prog.Types) == 0 {
		return
	}
	rules.ExpandSubtypes(subtypeResolver{prog})
}

type subtypeResolver struct{ p *ir.Program }

func (r subtypeResolver) SubtypeMethods(super, with string) []model.MarkedMethod {
	var out []model.MarkedMethod
	for _, m := range r.p.SubtypeMethods(ir.TypeID(super), ir.TypeID(with)) {
		out = append(out, model.MarkedMethod{Symbol: m.Symbol, Params: m.Params})
	}
	return out
}

func (r subtypeResolver) CallsFunction(symbol string) bool { return r.p.CallsFunction(symbol) }

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
func (l *loaded) packages() []string {
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
		for i := range l.ir.Funcs {
			id := string(l.ir.Funcs[i].ID)
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
func (l *loaded) extract(withGraph bool) error {
	prog, err := goir.Extract(l.gossa, goir.Options{All: withGraph, NoCandidates: !withGraph})
	if err != nil {
		return err
	}
	l.ir = prog
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
