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
	"sort"
	"strings"
	"syscall"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
	"github.com/yagihash/rivus/internal/report"
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
	fs.BoolVar(&c.tests, "tests", false, "include _test.go files")
	fs.BoolVar(&c.noDeps, "no-deps", false, "do not read the source of the dependencies (which is done by default, except for the standard library, so that calls into them are followed instead of guessed): lighter (about a sixth of the memory), and every call into a dependency is a guess")
	fs.Var(&c.allowDepsUnder, "allow-deps-under", "Dart: a directory outside the project whose packages may be read as dependencies (repeatable). The package_config.json of the project says where its dependencies are, and it is not trusted: only the project, the pub cache and the SDKs are read without being asked (a monorepo's sibling packages need this)")
	fs.BoolVar(&c.deterministic, "deterministic-only", false, "ignore the rules decided by an LLM (origin: llm)")
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
		// Not filepath.Glob: the directory is a name, and may hold the characters of a pattern.
		rulesDir := filepath.Join(c.dir, ".rivus", "rules")
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
				err = model.RedactValues(err)
			}
			return nil, err
		}
		rs.Project = project[path]
		sets = append(sets, rs)
	}
	sets = append(sets, extra...)
	var rules *model.Rules
	if c.deterministic {
		rules = model.WithoutOrigin(model.OriginLLM, sets...)
	} else {
		rules = model.Merge(sets...)
	}
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	return rules, nil
}

// load reads the code: the Go packages, and the neutral IR made of them.
// withGraph also builds what the call graph needs (the analysis does; the
// listing of sites does not).
func (c *common) load(patterns []string, withGraph bool) (*loaded, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	p, err := gossa.Load(c.dir, patterns, gossa.Options{Tests: c.tests, NaiveForm: true, WithLifted: withGraph, Deps: !c.noDeps && withGraph})
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
	l, err := c.load(patterns, true)
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
