package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Corpus is what one IR shows of the calls: the names of the callees, counted. It is the input of the
// "dead row" check (a row that no IR hits) and of the demand count (the calls to functions without a body).
type Corpus struct {
	Name string `json:"name"`
	Lang string `json:"lang"`
	Real bool   `json:"real"` // the code of a real project (not a fixture written for the engine)
	// Supp is an input that is shown beside the others and is not part of their sums: the same code read another way
	// (Dart with the tests that the extractor leaves out by default).
	Supp  bool `json:"supp,omitempty"`
	Funcs int  `json:"funcs"`
	// Calls maps a callee symbol to what the IR calls it as.
	Calls map[string]*CallCount `json:"calls"`
	// FuncSyms are the symbols of the functions the IR defines (entry-point rows, kind: param, are hit by them).
	FuncSyms map[string]bool `json:"func_syms"`
	Note     string          `json:"note,omitempty"`
}

// CallCount: Lib are the sites whose callee has no body in the program (a static call to a function the IR
// does not define, or a dynamic call whose candidates the IR does not define); Own are the other sites.
// Test are the sites (of both kinds) in a function of a test directory.
type CallCount struct {
	Lib  int `json:"lib,omitempty"`
	Own  int `json:"own,omitempty"`
	Test int `json:"test,omitempty"`
	// LibTest is the part of Lib that is in tests.
	LibTest int `json:"lib_test,omitempty"`
}

type irLine struct {
	Kind string  `json:"kind"`
	Func *irFunc `json:"func"`
}

type irFunc struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
	File   string `json:"file"`
	Pos    struct {
		File string `json:"file"`
	} `json:"pos"`
	Dep    bool `json:"dep"`
	Blocks []struct {
		Instrs []struct {
			Op     string `json:"op"`
			Callee *struct {
				Kind       string   `json:"kind"`
				Func       string   `json:"func"`
				Symbol     string   `json:"symbol"`
				Candidates []string `json:"candidates"`
			} `json:"callee"`
		} `json:"instrs"`
	} `json:"blocks"`
}

func isTestPath(p string) bool {
	return strings.HasPrefix(p, "test/") || strings.Contains(p, "/test/") || strings.HasSuffix(p, "_test.go") ||
		strings.HasPrefix(p, "tests/") || strings.Contains(p, "/tests/") || strings.HasPrefix(p, "integration_test/")
}

func validSymbol(lang, s string) bool {
	if s == "" || strings.HasPrefix(s, "builtin.") || strings.HasPrefix(s, "rt.") {
		return false
	}
	switch lang {
	case "dart":
		return strings.Contains(s, "#")
	case "rust":
		return strings.Contains(s, "::")
	}
	return true
}

// ScanIR reads an IR stream (JSON Lines).
func ScanIR(r io.Reader, name, lang string, real bool) (*Corpus, error) {
	c := &Corpus{Name: name, Lang: lang, Real: real, Calls: map[string]*CallCount{}, FuncSyms: map[string]bool{}}
	type site struct {
		sym, fn, kind string
		test, dyn     bool
		cands         []string
	}
	var sites []site
	ids := map[string]bool{}
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			var l irLine
			if jerr := json.Unmarshal(line, &l); jerr == nil && l.Kind == "func" && l.Func != nil {
				f := l.Func
				ids[f.ID] = true
				if f.Symbol != "" {
					c.FuncSyms[f.Symbol] = true
				}
				c.Funcs++
				file := f.File
				if file == "" {
					file = f.Pos.File
				}
				test := isTestPath(file)
				for _, b := range f.Blocks {
					for _, in := range b.Instrs {
						if in.Op != "call" || in.Callee == nil {
							continue
						}
						ce := in.Callee
						switch ce.Kind {
						case "static":
							if validSymbol(lang, ce.Symbol) {
								sites = append(sites, site{sym: ce.Symbol, fn: ce.Func, test: test})
							}
						case "dynamic":
							if validSymbol(lang, ce.Symbol) {
								sites = append(sites, site{sym: ce.Symbol, dyn: true, cands: ce.Candidates, test: test})
							}
						}
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	for _, s := range sites {
		cc := c.Calls[s.sym]
		if cc == nil {
			cc = &CallCount{}
			c.Calls[s.sym] = cc
		}
		lib := false
		if s.dyn {
			lib = true
			for _, id := range s.cands {
				if ids[id] {
					lib = false
					break
				}
			}
		} else {
			lib = s.fn == "" || !ids[s.fn]
		}
		if lib {
			cc.Lib++
			if s.test {
				cc.LibTest++
			}
		} else {
			cc.Own++
		}
		if s.test {
			cc.Test++
		}
	}
	return c, nil
}

func scanFile(path, name, lang string, real bool) (*Corpus, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ScanIR(f, name, lang, real)
}

func (c *Corpus) save(path string) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func loadCorpus(path string) (*Corpus, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Corpus
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

func (c *Corpus) symbols() []string {
	out := make([]string, 0, len(c.Calls))
	for s := range c.Calls {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
