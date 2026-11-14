package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
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
	tests     bool
	// deterministic leaves out the rules decided by an LLM.
	deterministic bool
}

func (c *common) register(fs *flag.FlagSet) {
	fs.StringVar(&c.dir, "C", ".", "directory to analyse from")
	fs.Var(&c.rules, "rules", "additional rule file (repeatable)")
	fs.BoolVar(&c.noBuiltin, "no-builtin-rules", false, "do not load the built-in rules")
	fs.BoolVar(&c.tests, "tests", false, "include _test.go files")
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
	for _, path := range c.rules {
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
	p, err := gossa.Load(c.dir, patterns, gossa.Options{Tests: c.tests, NaiveForm: true, WithLifted: withGraph})
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

// packages lists the import paths of the packages that were analysed.
func (l *loaded) packages() []string {
	var out []string
	for _, pkg := range l.gossa.Initial {
		out = append(out, pkg.PkgPath)
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
