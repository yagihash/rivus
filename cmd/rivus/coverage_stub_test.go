package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The stub of "rivus models coverage -yaml" is the start of a rule for a call that has no model. Every stub put in must
// lose no finding (it may add some: it is broader than the default on purpose). The first versions did: they left the
// receiver out, then the writes into the arguments, and 3 and 2 of the 27 findings of bench/gcp went.
func TestModelsCoverageStubsLoseNoFinding(t *testing.T) {
	for _, c := range []struct {
		dir  string
		args []string
	}{
		{"../../bench/gcp", []string{"-no-deps", "./..."}},
		{"../../bench/go-curated", []string{"-no-deps", "-rules", "rules.yaml", "./..."}},
	} {
		run := func(args ...string) []byte {
			cmd := exec.Command(binary, args...)
			cmd.Dir = c.dir
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("%s %v: %v", c.dir, args, err)
			}
			return out
		}
		keys := func(out []byte) map[string]bool {
			var j struct {
				Findings []struct {
					Source struct {
						Func string `json:"func"`
					} `json:"source"`
					Sink struct {
						Position struct {
							File string `json:"file"`
							Line int    `json:"line"`
						} `json:"position"`
					} `json:"sink"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(out, &j); err != nil {
				t.Fatal(err)
			}
			m := map[string]bool{}
			for _, f := range j.Findings {
				// a method value is the same flow under another name ($bound)
				m[strings.TrimSuffix(f.Source.Func, "$bound")+" -> "+f.Sink.Position.File+":"+itoaT(f.Sink.Position.Line)] = true
			}
			return m
		}
		yaml := run(append([]string{"models", "coverage", "-yaml", "-top", "1000"}, c.args...)...)
		file := filepath.Join(t.TempDir(), "stubs.yaml")
		if err := os.WriteFile(file, yaml, 0o644); err != nil {
			t.Fatal(err)
		}
		without := keys(run(append([]string{"analyze"}, c.args...)...))
		with := keys(run(append([]string{"analyze", "-rules", file}, c.args...)...))
		var lost []string
		for k := range without {
			if !with[k] {
				lost = append(lost, k)
			}
		}
		sort.Strings(lost)
		if len(lost) > 0 {
			t.Errorf("%s: with every stub, %d of %d findings are gone: %v", c.dir, len(lost), len(without), lost)
		}
	}
}

func itoaT(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
