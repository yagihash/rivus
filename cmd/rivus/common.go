package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

// common holds the flags shared by the commands that analyse code.
type common struct {
	dir       string
	rules     multiFlag
	noBuiltin bool
	// noProject leaves out the rules found in the analysed directory.
	noProject bool
	tests     bool
	// deps reads the source of the dependencies (not the standard library).
	deps bool
	// deterministic leaves out the rules decided by an LLM.
	deterministic bool
}

func (c *common) register(fs *flag.FlagSet) {
	fs.StringVar(&c.dir, "C", ".", "directory to analyse from")
	fs.Var(&c.rules, "rules", "additional rule file (repeatable)")
	fs.BoolVar(&c.noBuiltin, "no-builtin-rules", false, "do not load the built-in rules")
	fs.BoolVar(&c.noProject, "no-project-rules", false, "do not load the rules found in <dir>/.rivus/rules/*.yaml")
	fs.BoolVar(&c.tests, "tests", false, "include _test.go files")
	fs.BoolVar(&c.deps, "deps", false, "read the source of the dependencies (not the standard library), so that calls into them are followed instead of guessed")
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
	if !c.noProject {
		found, _ := filepath.Glob(filepath.Join(c.dir, ".rivus", "rules", "*.y*ml"))
		sort.Strings(found)
		for _, p := range found {
			fmt.Fprintf(os.Stderr, "rivus: project rules: %s\n", p)
		}
		paths = append(found, paths...)
	}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		rs, err := model.Parse(f, path)
		f.Close()
		if err != nil {
			return nil, err
		}
		sets = append(sets, rs)
	}
	sets = append(sets, extra...)
	if c.deterministic {
		return model.WithoutOrigin(model.OriginLLM, sets...), nil
	}
	return model.Merge(sets...), nil
}

// load reads the code: the Go packages, and the neutral IR made of them.
// withGraph also builds what the call graph needs (the analysis does; the
// listing of sites does not).
func (c *common) load(patterns []string, withGraph bool) (*loaded, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	p, err := gossa.Load(c.dir, patterns, gossa.Options{Tests: c.tests, NaiveForm: true, WithLifted: withGraph, Deps: c.deps})
	if err != nil {
		return nil, err
	}
	return &loaded{gossa: p}, nil
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
	return &loaded{ir: prog, texts: ir.NewStreamTexts(prog)}, nil
}

// loadErrors are what could not be read, from the packages or the stream.
func (l *loaded) loadErrors() []ir.LoadError {
	if l.gossa != nil {
		return l.gossa.LoadErrors
	}
	return l.ir.LoadErrors
}

// packages lists the import paths of the packages that were analysed.
func (l *loaded) packages() []string {
	if l.gossa == nil {
		return nil
	}
	return l.gossa.PkgPaths
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
