package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// TestScaleSynth is the guard of the scale of the engine (bench/go-scale, bench/README.md): small modules
// of about 2,000 functions, each of one shape that made Gitea and gogs big (a cache of types, os.Stderr,
// a widen that keeps the parts, a ring of 200 functions, ...). Each one carries its answer key in the
// comments of bench/go-curated (flows that must stay, sinks that must stay quiet: checkCuratedReport), and
// bench/go-scale/expected/<profile>-w<workers>.json holds what the analysis gave when the file was written:
//
//   - the set of the findings, the kinds of the uncertainties and "converged" must be the same;
//   - the sizes (passes, reanalyses, summary_elems, the largest summary) must not be more than 10% over the file;
//     when they are different, the difference is logged (a smaller number is an improvement: write the file).
//
// The analysis is deterministic, so the same input gives the same numbers on every run. Time is not looked at.
// RIVUS_SCALE_UPDATE=1 writes the files (for a change that is meant to move them: the diff of the files is
// what a commit shows). -workers is fixed: R-43 (issue #96) makes summary_elems depend on it, and the ring
// profile keeps both numbers, so that the day it is fixed the two files become one.
func TestScaleSynth(t *testing.T) {
	if testing.Short() {
		t.Skip("the scale profiles take a minute (-short skips them)")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	gen := filepath.Join(t.TempDir(), "gen")
	build := exec.Command("go", "build", "-o", gen, "./bench/go-scale/gen")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the generator: %v\n%s", err, out)
	}
	profiles := []struct {
		name    string // the name of the expected file
		shapes  string
		flags   []string
		workers []int
		noconv  bool // the run does not converge (the expected file says so)
	}{
		{"types", "types", nil, []int{1}, false},
		{"typeschain", "typeschain", nil, []int{1}, false},
		{"stderr", "stderr", nil, []int{1}, false},
		{"widen", "widen", nil, []int{1}, false},
		{"scc", "scc", nil, []int{1, 4}, false},
		// the chain of globals, one step a pass: the number of passes follows the number of steps (3 steps 4 passes, 10 steps 11),
		// and at 12 passes (MaxPasses) it stops: 13 and 30 steps do not converge and the flow is lost (@miss).
		{"chain3", "chain", []string{"-chain", "3"}, []int{1}, false},
		{"chain", "chain", nil, []int{1}, false},
		{"chain13", "chain", []string{"-chain", "13"}, []int{1}, true},
		{"chain30", "chain", []string{"-chain", "30"}, []int{1}, true},
		{"dyn", "dyn", nil, []int{1}, false},
		{"glob", "glob", nil, []int{1}, false},
		{"giant", "giant", nil, []int{1}, false},
		{"selfcopy", "selfcopy", nil, []int{1}, false},
	}
	for _, pr := range profiles {
		pr := pr
		t.Run(pr.name, func(t *testing.T) {
			dir := t.TempDir()
			if out, err := exec.Command(gen, append([]string{"-shapes", pr.shapes, "-out", dir}, pr.flags...)...).CombinedOutput(); err != nil {
				t.Fatalf("gen: %v\n%s", err, out)
			}
			for _, w := range pr.workers {
				cmd := exec.Command(binary, "analyze", "-sites", "-workers", fmt.Sprint(w), "-rules", "rules.yaml", "./...")
				cmd.Dir = dir
				out, err := cmd.Output()
				if err != nil {
					t.Fatalf("analyze: %v", err)
				}
				var rep report.Report
				if err := json.Unmarshal(out, &rep); err != nil {
					t.Fatal(err)
				}
				curatedAllowUnconverged = pr.noconv
				checkCuratedReport(t, dir, out, ".go")
				curatedAllowUnconverged = false
				got := scaleResultOf(&rep)
				path := filepath.Join(root, "bench", "go-scale", "expected", fmt.Sprintf("%s-w%d.json", pr.name, w))
				if os.Getenv("RIVUS_SCALE_UPDATE") == "1" {
					b, _ := json.MarshalIndent(got, "", "  ")
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("no expected file (RIVUS_SCALE_UPDATE=1 writes it): %v", err)
				}
				var want scaleResult
				if err := json.Unmarshal(raw, &want); err != nil {
					t.Fatal(err)
				}
				compareScale(t, fmt.Sprintf("%s -workers %d", pr.name, w), want, got)
			}
		})
	}
}

// scaleResult is what an expected file holds.
type scaleResult struct {
	Findings         []string       `json:"findings"` // "source file:line -> sink file:line", sorted
	UncertaintyKinds map[string]int `json:"uncertainty_kinds"`
	Converged        bool           `json:"converged"`
	Passes           int            `json:"passes"`
	Reanalyses       int            `json:"reanalyses"`
	SummaryElems     int            `json:"summary_elems"`
	LargestSummary   int            `json:"largest_summary_elems"`
}

func scaleResultOf(rep *report.Report) scaleResult {
	r := scaleResult{UncertaintyKinds: map[string]int{}, Findings: []string{}}
	for _, f := range rep.Findings {
		r.Findings = append(r.Findings, fmt.Sprintf("%s:%d -> %s:%d", f.Source.Position.File, f.Source.Position.Line, f.Sink.Position.File, f.Sink.Position.Line))
	}
	sort.Strings(r.Findings)
	for _, u := range rep.Uncertainties {
		r.UncertaintyKinds[u.Kind]++
	}
	st := rep.Stats
	r.Converged, r.Passes, r.Reanalyses, r.SummaryElems, r.LargestSummary = st.Converged, st.Passes, st.Reanalyses, st.SummaryElems, st.LargestElems
	return r
}

func compareScale(t *testing.T, name string, want, got scaleResult) {
	t.Helper()
	if fmt.Sprint(want.Findings) != fmt.Sprint(got.Findings) {
		t.Errorf("%s: the findings are not the ones of the expected file:\n want %v\n  got %v", name, want.Findings, got.Findings)
	}
	if fmt.Sprint(want.UncertaintyKinds) != fmt.Sprint(got.UncertaintyKinds) {
		t.Errorf("%s: the uncertainties by kind are not the ones of the expected file: want %v, got %v", name, want.UncertaintyKinds, got.UncertaintyKinds)
	}
	if want.Converged != got.Converged {
		t.Errorf("%s: converged was %v, now %v", name, want.Converged, got.Converged)
	}
	for _, m := range []struct {
		what      string
		want, got int
	}{
		{"passes", want.Passes, got.Passes},
		{"reanalyses", want.Reanalyses, got.Reanalyses},
		{"summary_elems", want.SummaryElems, got.SummaryElems},
		{"largest_summary_elems", want.LargestSummary, got.LargestSummary},
	} {
		switch {
		case m.got*10 > m.want*11: // more than 10% over
			t.Errorf("%s: %s grew from %d to %d (more than 10%%: the engine got worse on this shape, or the change is meant: RIVUS_SCALE_UPDATE=1)", name, m.what, m.want, m.got)
		case m.got != m.want:
			t.Logf("%s: %s is %d, the expected file says %d (inside 10%%: write the file when it is an improvement)", name, m.what, m.got, m.want)
		}
	}
}
