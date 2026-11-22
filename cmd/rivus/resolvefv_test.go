package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
)

// -resolve-func-values (docs/DESIGN-callouts.md, 5.6): the calls of function values that have no candidate get the
// functions that the analysis of the function values carries to them, before the taint analysis. Off by default: the
// output is then what it was. The tests: the output with the flag off is the same whatever else is on (and whatever
// -workers is); with the flag on, the findings that differ are those through a call that got candidates; the cases of
// the fixture (testdata/dart/funcvalues, as IR in testdata/ir/funcvalues.jsonl) say which calls get a candidate and
// which do not.

// fvReport is what these tests read of a result.
type fvReport struct {
	rep report.Report
	raw map[string]json.RawMessage
}

func fvParse(t *testing.T, out []byte) fvReport {
	t.Helper()
	var r fvReport
	if err := json.Unmarshal(out, &r.rep); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &r.raw); err != nil {
		t.Fatal(err)
	}
	return r
}

// stats is the stats of the result as a map (the keys that are there, not the fields that the code knows).
func (r fvReport) stats(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(r.raw["stats"], &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func (r fvReport) funcValues(t *testing.T) *taint.FuncValuesStats {
	t.Helper()
	raw, ok := r.stats(t)["func_values"]
	if !ok {
		return nil
	}
	var s taint.FuncValuesStats
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return &s
}

type fvPos struct {
	File      string
	Line, Col int
}

func (p fvPos) String() string { return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col) }

// fvFindingKey is the source and the sink of a finding.
func fvFindingKey(f *taint.Finding) string {
	return fvPos{f.Source.Position.File, f.Source.Position.Line, f.Source.Position.Column}.String() + " -> " +
		fvPos{f.Sink.Position.File, f.Sink.Position.Line, f.Sink.Position.Column}.String()
}

// fvDiff compares the findings of the run with the flag off and on, and says what a difference may be: a finding that
// is only in the run with the flag on has the uncertainty of a call that got candidates; one that is only in the run with
// the flag off has the uncertainty "the called function is not known" at the place of such a call (the call it went through
// before the candidates were known). It returns the keys of the findings that are only on, and only off.
func fvDiff(t *testing.T, name string, off, on fvReport) (added, removed []string) {
	t.Helper()
	supplemented := map[string]bool{}
	for _, u := range on.rep.Uncertainties {
		if strings.HasPrefix(u.Detail, "func_values_supplemented:") {
			supplemented[fvPos{u.Position.File, u.Position.Line, u.Position.Column}.String()] = true
		}
	}
	offBy, onBy := map[string]*taint.Finding{}, map[string]*taint.Finding{}
	for _, f := range off.rep.Findings {
		offBy[fvFindingKey(f)] = f
	}
	for _, f := range on.rep.Findings {
		onBy[fvFindingKey(f)] = f
	}
	for k, f := range onBy {
		if offBy[k] != nil {
			continue
		}
		added = append(added, k)
		ok := false
		for _, u := range f.Uncertainties {
			ok = ok || strings.HasPrefix(u.Detail, "func_values_supplemented:")
		}
		if !ok {
			t.Errorf("%s: the finding %s is only there with the flag, and no call that got candidates is on its path", name, k)
		}
	}
	for k, f := range offBy {
		if onBy[k] != nil {
			continue
		}
		removed = append(removed, k)
		ok := false
		for _, u := range f.Uncertainties {
			ok = ok || supplemented[fvPos{u.Position.File, u.Position.Line, u.Position.Column}.String()]
		}
		if !ok {
			t.Errorf("%s: the finding %s is gone with the flag, and no call that got candidates is on its path", name, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// fvRun is `rivus analyze args...` in dir.
func fvRun(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(binary, append([]string{"analyze"}, args...)...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze %v in %s: %v\n%s", args, dir, err, stderr.String())
	}
	return out
}

const (
	fvIR      = "../../testdata/ir/funcvalues.jsonl"
	fvRules   = "../../testdata/ir/funcvalues_rules.yaml"
	fvDartSrc = "../../testdata/dart/funcvalues/lib/cells.dart"
)

// fvTagLines is the lines of the annotations (// @source(tag), // @sink(tag)) of the Dart fixture.
func fvTagLines(t *testing.T) map[string]int {
	t.Helper()
	b, err := os.ReadFile(fvDartSrc)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for i, l := range strings.Split(string(b), "\n") {
		for _, role := range []string{"source", "sink"} {
			if j := strings.Index(l, "// @"+role+"("); j >= 0 {
				tag := l[j+len("// @"+role+"("):]
				tag = tag[:strings.Index(tag, ")")]
				out[role+":"+tag] = i + 1
			}
		}
	}
	return out
}

func fvLines(r fvReport) map[[2]int]bool {
	out := map[[2]int]bool{}
	for _, f := range r.rep.Findings {
		out[[2]int{f.Source.Position.Line, f.Sink.Position.Line}] = true
	}
	return out
}

// TestResolveFuncValuesFixture: the cases of the fixture. (a) a call with no candidate gets the closure that is carried to
// it, and the flow through it is found (also the callback of a callback, which needs a second round); (b) a call that no
// function value reaches gets none, a closure of the same type that does not reach it is not a candidate (the decoy), and
// a closure made in a function of a dependency is not a seed; (c) the differences are the findings through such calls.
func TestResolveFuncValuesFixture(t *testing.T) {
	tags := fvTagLines(t)
	off := fvParse(t, fvRun(t, ".", "-ir", fvIR, "-rules", fvRules))
	on := fvParse(t, fvRun(t, ".", "-ir", fvIR, "-rules", fvRules, "-resolve-func-values"))

	if _, ok := off.stats(t)["func_values"]; ok {
		t.Errorf("stats.func_values is there with the flag off")
	}
	fs := on.funcValues(t)
	if fs == nil {
		t.Fatal("stats.func_values is not there with the flag on")
	}
	// run, run2, runNone, runDep have no candidate; run and run2 get some
	if fs.Skipped || fs.ZeroBefore != 4 || fs.ZeroAfter != 2 || fs.Supplemented != 2 || fs.Rounds < 2 || fs.Violations != 0 {
		t.Errorf("stats.func_values = %+v, want 4 calls with no candidate before and 2 after, 2 supplemented, 2 rounds or more, no violation", fs)
	}

	if got := fvLines(off); len(got) != 0 {
		t.Errorf("the flag off: findings %v, want none (the callbacks are not followed)", got)
	}
	want := map[[2]int]bool{
		{tags["source:pos"], tags["sink:pos"]}:   true,
		{tags["source:pos2"], tags["sink:pos2"]}: true,
	}
	got := fvLines(on)
	for k := range want {
		if !got[k] {
			t.Errorf("the flag on: no finding %v (source line, sink line)", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("the flag on: the finding %v is not one of the cases (a decoy, or a closure of a dependency, became a candidate)", k)
		}
	}
	added, removed := fvDiff(t, "fixture", off, on)
	if len(added) != 2 || len(removed) != 0 {
		t.Errorf("added %v, removed %v", added, removed)
	}

	// the calls that got candidates keep their uncertainty, and say what the candidates are; the others are as they were
	var supplemented, unknown int
	for _, u := range on.rep.Uncertainties {
		switch {
		case strings.HasPrefix(u.Detail, "func_values_supplemented:"):
			supplemented++
			if u.Kind != taint.KindUnresolvedCallee || len(u.Candidates) == 0 {
				t.Errorf("a call with candidates from the function values: %+v", u)
			}
		case u.Detail == "the called function is not known":
			unknown++
		}
	}
	if supplemented != fs.Supplemented || unknown != 2 {
		t.Errorf("uncertainties: %d with candidates from the function values (want %d), %d calls not known (want 2: runNone, runDep)", supplemented, fs.Supplemented, unknown)
	}
	for _, u := range off.rep.Uncertainties {
		if strings.HasPrefix(u.Detail, "func_values") {
			t.Errorf("the flag off: %+v", u)
		}
	}
}

// TestResolveFuncValuesSkipped: with no call of a function value that has no candidate, nothing is run, and the result
// is the same.
func TestResolveFuncValuesSkipped(t *testing.T) {
	ir, rules := "../../testdata/ir/dart_exceptions.jsonl", "../../testdata/ir/dart_exceptions_rules.yaml"
	off := fvParse(t, fvRun(t, ".", "-ir", ir, "-rules", rules))
	on := fvParse(t, fvRun(t, ".", "-ir", ir, "-rules", rules, "-resolve-func-values"))
	fs := on.funcValues(t)
	if fs == nil || !fs.Skipped || fs.Rounds != 0 || fs.ZeroBefore != 0 || fs.Supplemented != 0 {
		t.Fatalf("stats.func_values = %+v, want skipped", fs)
	}
	if added, removed := fvDiff(t, "skipped", off, on); len(added)+len(removed) != 0 || len(off.rep.Findings) != len(on.rep.Findings) {
		t.Errorf("the findings differ: added %v, removed %v", added, removed)
	}
}

// TestResolveFuncValuesOffIsTheSame: with the flag off the output is the same with -func-values (a diagnostic that runs
// the analysis once more) as without, and whatever -workers is; with the flag on it does not depend on -workers either.
func TestResolveFuncValuesOffIsTheSame(t *testing.T) {
	table := filepath.Join(t.TempDir(), "fv.jsonl")
	base := []string{"-ir", fvIR, "-rules", fvRules}
	plain := stable(t, fvRun(t, ".", base...))
	for _, extra := range [][]string{{"-func-values", table}, {"-workers", "1"}, {"-workers", "16"}} {
		got := stable(t, fvRun(t, ".", append(append([]string{}, extra...), base...)...))
		if !bytes.Equal(plain, got) {
			t.Errorf("the flag off, %v: the output differs\n%s", extra, firstDifference(plain, got))
		}
	}
	on := append([]string{"-resolve-func-values"}, base...)
	first := stable(t, fvRun(t, ".", on...))
	for _, w := range []string{"1", "2", "16"} {
		got := stable(t, fvRun(t, ".", append([]string{"-workers", w}, on...)...))
		if !bytes.Equal(first, got) {
			t.Errorf("the flag on, -workers %s: the output differs\n%s", w, firstDifference(first, got))
		}
	}
}

// TestFuncValuesTableDeterministic: the table of -func-values is the same for the same input, whatever -workers is, and
// says where a model drops a function value (the column dropped_at_model, and the symbols on stderr).
func TestFuncValuesTableDeterministic(t *testing.T) {
	var first []byte
	for _, w := range []string{"", "1", "2", "16", "16"} {
		path := filepath.Join(t.TempDir(), "fv.jsonl")
		args := []string{"-func-values", path, "-ir", fvIR, "-rules", fvRules}
		if w != "" {
			args = append([]string{"-workers", w}, args...)
		}
		cmd := exec.Command(binary, append([]string{"analyze"}, args...)...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if _, err := cmd.Output(); err != nil {
			t.Fatal(err, stderr.String())
		}
		if !strings.Contains(stderr.String(), "cells.dart#keep (1)") {
			t.Errorf("-workers %q: the symbol that dropped a function value is not said: %s", w, stderr.String())
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = got
			continue
		}
		if !bytes.Equal(first, got) {
			t.Errorf("-workers %q: the table differs", w)
		}
	}
	// the rows are sorted by the site, and there is one for every call of a function value
	rows := strings.Split(strings.TrimSpace(string(first)), "\n")
	for _, line := range rows {
		var r funcValueRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) < 4 {
		t.Errorf("the table has %d rows", len(rows))
	}
}

// TestFuncValuesMonotone: the candidates of a call only grow from one round to the next; a round that loses one is a
// defect, and the check says so.
func TestFuncValuesMonotone(t *testing.T) {
	prev := map[string][]string{"a": {"f", "g"}, "b": {"h"}, "c": nil}
	if bad := taint.CheckMonotone(prev, map[string][]string{"a": {"f", "g", "i"}, "b": {"h"}, "c": {"j"}}); len(bad) != 0 {
		t.Errorf("growing candidates are reported as a defect: %v", bad)
	}
	if bad := taint.CheckMonotone(prev, map[string][]string{"a": {"f"}, "b": {"h"}}); len(bad) != 1 || bad[0] != "a" {
		t.Errorf("a call that lost a candidate: %v, want [a]", bad)
	}
	if bad := taint.CheckMonotone(prev, map[string][]string{"a": {"f", "g"}}); len(bad) != 1 || bad[0] != "b" {
		t.Errorf("a call that has none any more: %v, want [b]", bad)
	}
	// and a run of the rounds on the fixture has none (stats.func_values.monotone_violations is absent when it is 0)
	on := fvParse(t, fvRun(t, ".", "-ir", fvIR, "-rules", fvRules, "-resolve-func-values"))
	if fs := on.funcValues(t); fs == nil || fs.Violations != 0 {
		t.Errorf("the rounds lost a candidate: %+v", fs)
	}
}

// fvAnalysis is the analysis of the IR in the process, up to the graph: the loaded code, the sites, the rules and the graph.
type fvAnalysis struct {
	l     *loaded
	st    *sites.Result
	rules *model.Rules
	graph *sg.Program
}

func fvLoad(t *testing.T, irFile, rules string) *fvAnalysis {
	t.Helper()
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	var o analyzeOpts
	o.register(fs)
	if err := fs.Parse([]string{"-ir", irFile, "-rules", rules}); err != nil {
		t.Fatal(err)
	}
	a := &fvAnalysis{}
	o.afterRead = func(l *loaded, r *model.Rules, st *sites.Result, g *sg.Program, _ map[string]string) error {
		a.l, a.rules, a.st, a.graph = l, r, st, g
		return nil
	}
	o.structureOnly = true
	if _, err := o.analyze(nil); err != nil {
		t.Fatal(err)
	}
	return a
}

// TestFuncValuesFindingsAreNotReported: a closure given to a real sink is a finding of the run of the function values
// (the label FUNC reaches it); it is not reported there, and not in the result either (FV-1 of the review).
func TestFuncValuesFindingsAreNotReported(t *testing.T) {
	a := fvLoad(t, fvIR, fvRules)
	res, calls := runFuncValues(a.graph, a.l, a.st, a.rules, taint.DefaultOptions())
	if len(calls) == 0 {
		t.Fatal("no call of a function value")
	}
	for _, f := range res.Findings {
		b, _ := json.Marshal(f)
		if bytes.Contains(b, []byte(taint.FuncLabel)) || bytes.Contains(b, []byte(`"fn:`)) {
			t.Errorf("a finding of the function values is reported: %s", b)
		}
		if strings.Contains(f.Sink.Symbol, "#exec") {
			t.Errorf("the closure given to exec is a finding: %s", b)
		}
	}
	// and the result of the analysis does not have it either
	on := fvParse(t, fvRun(t, ".", "-ir", fvIR, "-rules", fvRules, "-resolve-func-values"))
	for _, f := range on.rep.Findings {
		if strings.Contains(f.Sink.Symbol, "#exec") || len(f.Labels) == 0 {
			t.Errorf("finding %+v", f)
		}
		for _, l := range f.Labels {
			if l == taint.FuncLabel {
				t.Errorf("finding with the label %s", l)
			}
		}
	}
}

// TestResolveFuncValuesGo: Go. The call graph of Go resolves nearly every call of a function value, so there is little for
// the flag to do: the cases and the targets of the benchmarks are the same with it and without it (the calls that it
// would resolve are tested on the IR above).
func TestResolveFuncValuesGo(t *testing.T) {
	for _, c := range []struct {
		dir  string
		args []string
	}{
		{"../../testdata/go", []string{"-rules", "rules.yaml", "./cases/resolvefv/...", "./cases/funcvalue/..."}},
		{"../../bench/zoo-go", []string{"-rules", "rules.yaml", "./..."}},
		{"../../bench/go-curated", []string{"-rules", "rules.yaml", "./..."}},
	} {
		if testing.Short() && !strings.Contains(c.dir, "testdata") {
			continue
		}
		off := fvParse(t, fvRun(t, c.dir, c.args...))
		on := fvParse(t, fvRun(t, c.dir, append([]string{"-resolve-func-values"}, c.args...)...))
		if _, ok := off.stats(t)["func_values"]; ok {
			t.Errorf("%s: stats.func_values with the flag off", c.dir)
		}
		fs := on.funcValues(t)
		if fs == nil {
			t.Fatalf("%s: no stats.func_values", c.dir)
		}
		added, removed := fvDiff(t, c.dir, off, on)
		t.Logf("%s: %+v; findings added %d, removed %d", filepath.Base(c.dir), *fs, len(added), len(removed))
		if fs.Violations != 0 {
			t.Errorf("%s: the candidates did not only grow: %+v", c.dir, fs)
		}
		if fs.Supplemented == 0 && (len(added) > 0 || len(removed) > 0) {
			t.Errorf("%s: the findings differ and no call got candidates", c.dir)
		}
	}
}

// TestResolveFuncValuesDart: the Dart zoo and the curated target: the flag off is the same as the output with -func-values,
// and the findings that differ with the flag on are the ones through a call that got candidates. It needs the extractor
// (RIVUS_DART).
func TestResolveFuncValuesDart(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	extractor, err := filepath.Abs("../../extractor/dart/bin/rivus_dart.dart")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"../../bench/zoo-dart/cells", "../../bench/dart-curated"} {
		run := func(args ...string) []byte {
			cmd := exec.Command(binary, append([]string{"analyze"}, args...)...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+extractor)
			var out bytes.Buffer
			cmd.Stdout = &out
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: %v", dir, err)
			}
			return out.Bytes()
		}
		args := []string{"-rules", "rules.yaml", "./..."}
		offRaw := run(args...)
		table := filepath.Join(t.TempDir(), "fv.jsonl")
		withTable := run(append([]string{"-func-values", table}, args...)...)
		if !bytes.Equal(stable(t, offRaw), stable(t, withTable)) {
			t.Errorf("%s: the output differs with -func-values", dir)
		}
		off := fvParse(t, offRaw)
		on := fvParse(t, run(append([]string{"-resolve-func-values"}, args...)...))
		fs := on.funcValues(t)
		if fs == nil || fs.Violations != 0 || fs.Skipped {
			t.Fatalf("%s: stats.func_values = %+v", dir, fs)
		}
		added, removed := fvDiff(t, dir, off, on)
		t.Logf("%s: %+v; findings added %d, removed %d", filepath.Base(dir), *fs, len(added), len(removed))
		if fs.Supplemented == 0 {
			t.Errorf("%s: no call got candidates", dir)
		}
	}
}

// TestResolveFuncValuesRust: the curated target of Rust (RIVUS_RUST).
func TestResolveFuncValuesRust(t *testing.T) {
	stream, dir := rustCuratedStream(t)
	irFile := filepath.Join(t.TempDir(), "rust.jsonl")
	if err := os.WriteFile(irFile, stream, 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-ir", irFile, "-rules", filepath.Join(dir, "rules.yaml")}
	offRaw := fvRun(t, dir, args...)
	withTable := fvRun(t, dir, append([]string{"-func-values", filepath.Join(t.TempDir(), "fv.jsonl")}, args...)...)
	if !bytes.Equal(stable(t, offRaw), stable(t, withTable)) {
		t.Errorf("the output differs with -func-values")
	}
	off := fvParse(t, offRaw)
	on := fvParse(t, fvRun(t, dir, append([]string{"-resolve-func-values"}, args...)...))
	fs := on.funcValues(t)
	if fs == nil || fs.Violations != 0 {
		t.Fatalf("stats.func_values = %+v", fs)
	}
	added, removed := fvDiff(t, "rust-curated", off, on)
	t.Logf("rust-curated: %+v; findings added %d, removed %d", *fs, len(added), len(removed))
}

// fvIndex makes the graph file of the fixture, with the flag on or off (and with -taint or not).
func fvIndex(t *testing.T, on, taintFlag bool) string {
	t.Helper()
	db := filepath.Join(t.TempDir(), "g.db")
	args := []string{"index", "-ir", fvIR, "-rules", fvRules, "-no-project-rules", "-graph", db}
	if on {
		args = append(args, "-resolve-func-values")
	}
	if taintFlag {
		args = append(args, "-taint", "-o", filepath.Join(t.TempDir(), "index.json"))
	}
	runIn(t, binary, ".", nil, args...)
	return db
}

const (
	fvFnRun = "package:fvsample/cells.dart#run"
	fvPosCb = "package:fvsample/cells.dart#pos$1"
	fvDecoy = "package:fvsample/cells.dart#negDecoy$1"
)

type fvPath struct {
	Reachable            bool `json:"reachable"`
	Exhaustive           bool `json:"exhaustive"`
	NotExhaustiveBecause struct {
		Unresolved struct {
			Total int `json:"total"`
		} `json:"unresolved_calls"`
	} `json:"not_exhaustive_because"`
	Paths []struct {
		Supplemented int `json:"supplemented_calls"`
		Steps        []struct {
			Func         string `json:"func"`
			Kind         string `json:"kind"`
			Candidates   int    `json:"candidates"`
			Supplemented bool   `json:"supplemented"`
		} `json:"steps"`
	} `json:"paths"`
}

func fvGraphPath(t *testing.T, db, from, to string) fvPath {
	t.Helper()
	var r fvPath
	if err := json.Unmarshal(runIn(t, binary, ".", nil, "graph", "path", "-graph", db, "--from", from, "--to", to), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// TestIndexHoldsTheEdgesThatGotCandidates: `rivus index -resolve-func-values` gives the calls of function values their
// candidates before the file is written, so they are edges of it, and the answers say which edges they are
// ("supplemented"). The file without the flag has no such edge, and the two files are not of the same key.
func TestIndexHoldsTheEdgesThatGotCandidates(t *testing.T) {
	for _, taintFlag := range []bool{false, true} {
		on, off := fvIndex(t, true, taintFlag), fvIndex(t, false, taintFlag)

		// callees: the edge is there with the flag, and says it was supplemented
		callees := func(db string) (edges []struct {
			To           string `json:"to"`
			Kind         string `json:"kind"`
			Candidates   int    `json:"candidates"`
			Supplemented bool   `json:"supplemented"`
		}) {
			var g struct {
				Edges []struct {
					To           string `json:"to"`
					Kind         string `json:"kind"`
					Candidates   int    `json:"candidates"`
					Supplemented bool   `json:"supplemented"`
				} `json:"edges"`
			}
			if err := json.Unmarshal(runIn(t, binary, ".", nil, "graph", "callees", "-graph", db, fvFnRun), &g); err != nil {
				t.Fatal(err)
			}
			return g.Edges
		}
		if e := callees(on); len(e) != 1 || e[0].To != fvPosCb || e[0].Kind != "value" || e[0].Candidates != 1 || !e[0].Supplemented {
			t.Errorf("-taint=%v: callees of run with the flag: %+v, want one edge of kind value to the closure of pos, supplemented", taintFlag, e)
		}
		if e := callees(off); len(e) != 0 {
			t.Errorf("-taint=%v: callees of run without the flag: %+v, want none", taintFlag, e)
		}

		// callers and dispatchers of the candidate: the edge says it was supplemented
		for _, op := range []string{"callers", "dispatchers"} {
			out := string(runIn(t, binary, ".", nil, "graph", op, "-graph", on, fvPosCb))
			if !strings.Contains(out, `"supplemented":true`) {
				t.Errorf("-taint=%v: graph %s of the candidate does not say the call was supplemented: %s", taintFlag, op, out)
			}
			if out := string(runIn(t, binary, ".", nil, "graph", op, "-graph", off, fvPosCb)); strings.Contains(out, "supplemented") {
				t.Errorf("-taint=%v: graph %s without the flag says supplemented: %s", taintFlag, op, out)
			}
		}

		// the key: another file for another setting (the flag is in the flags the file records, and in the hash)
		info := func(db string) (hash string, flags map[string]string) {
			var in struct {
				Graph struct {
					SourceHash string            `json:"source_hash"`
					Flags      map[string]string `json:"flags"`
				} `json:"graph"`
			}
			if err := json.Unmarshal(runIn(t, binary, ".", nil, "graph", "info", "-graph", db), &in); err != nil {
				t.Fatal(err)
			}
			return in.Graph.SourceHash, in.Graph.Flags
		}
		hOn, fOn := info(on)
		hOff, fOff := info(off)
		if hOn == hOff {
			t.Errorf("-taint=%v: the key of the file with the flag is the key of the file without it", taintFlag)
		}
		if fOn["resolve-func-values"] != "true" || fOff["resolve-func-values"] != "" {
			t.Errorf("-taint=%v: flags with %v, without %v", taintFlag, fOn, fOff)
		}

		// path: a way through the supplemented call is found, says so, and is not exhaustive
		r := fvGraphPath(t, on, fvFnRun, fvPosCb)
		if !r.Reachable || r.Exhaustive || len(r.Paths) != 1 || r.Paths[0].Supplemented != 1 || len(r.Paths[0].Steps) != 2 ||
			!r.Paths[0].Steps[0].Supplemented || r.Paths[0].Steps[0].Kind != "value" || r.Paths[0].Steps[0].Candidates != 1 {
			t.Errorf("-taint=%v: path with the flag: %+v, want one path through the supplemented call, not exhaustive", taintFlag, r)
		}
		// no path: not exhaustive through the call that got candidates (they are not all that it may reach), as through a call with none
		r = fvGraphPath(t, on, fvFnRun, fvDecoy)
		if r.Reachable || r.Exhaustive || r.NotExhaustiveBecause.Unresolved.Total != 1 {
			t.Errorf("-taint=%v: path to the decoy with the flag: reachable %v, exhaustive %v, unresolved calls %d; want no path, not exhaustive, 1", taintFlag, r.Reachable, r.Exhaustive, r.NotExhaustiveBecause.Unresolved.Total)
		}
		// the file without the flag: no path, not exhaustive, the call has no candidate (as before)
		r = fvGraphPath(t, off, fvFnRun, fvPosCb)
		if r.Reachable || r.Exhaustive || r.NotExhaustiveBecause.Unresolved.Total != 1 {
			t.Errorf("-taint=%v: path without the flag: reachable %v, exhaustive %v, unresolved calls %d; want no path, not exhaustive, 1 call with no candidate", taintFlag, r.Reachable, r.Exhaustive, r.NotExhaustiveBecause.Unresolved.Total)
		}
	}
}

// TestIndexKeyDiffersByFlag: a question about an index made with the flag, asked without it (or the other way round),
// says that the file is stale (the key differs), as for any other flag of the analysis.
func TestIndexKeyDiffersByFlag(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module m\n\ngo 1.22\n")
	write(t, filepath.Join(dir, "a.go"), "package m\n\nfunc F() {}\n")
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, ".", nil, "index", "-C", dir, "-graph", db, "-resolve-func-values")
	stale := func(extra ...string) (bool, string) {
		args := append([]string{"graph", "info", "-graph", db, "-C", dir}, extra...)
		var in struct {
			Snapshot struct {
				Fresh   *bool    `json:"fresh"`
				Reasons []string `json:"stale_reasons"`
			} `json:"snapshot"`
		}
		if err := json.Unmarshal(runIn(t, binary, ".", nil, args...), &in); err != nil {
			t.Fatal(err)
		}
		return in.Snapshot.Fresh != nil && !*in.Snapshot.Fresh, strings.Join(in.Snapshot.Reasons, ";")
	}
	if s, why := stale("-resolve-func-values"); s {
		t.Errorf("the same flag: stale (%s)", why)
	}
	if s, why := stale("-resolve-func-values=false"); !s || !strings.Contains(why, "resolve-func-values") {
		t.Errorf("the flag off against a file made with it: stale %v (%s), want stale because of resolve-func-values", s, why)
	}
}

// TestResolveFuncValuesTimeout: -timeout is the limit of all the rounds of the analysis of the function values and of
// the taint analysis together. A round that the limit stopped is not used (its candidates would be some of the
// candidates only), and stats.func_values says so (timed_out).
func TestResolveFuncValuesTimeout(t *testing.T) {
	on := fvParse(t, fvRun(t, ".", "-ir", fvIR, "-rules", fvRules, "-resolve-func-values", "-timeout", "1ns"))
	fs := on.funcValues(t)
	if fs == nil || !fs.TimedOut || fs.Supplemented != 0 || fs.ZeroAfter != fs.ZeroBefore {
		t.Fatalf("stats.func_values = %+v, want timed out and no call supplemented", fs)
	}
	for _, u := range on.rep.Uncertainties {
		if strings.HasPrefix(u.Detail, "func_values_supplemented:") {
			t.Errorf("a call has candidates from a round that the limit stopped: %+v", u)
		}
	}
	var st struct {
		TimedOut bool `json:"timed_out"`
	}
	if err := json.Unmarshal(on.raw["stats"], &st); err != nil || !st.TimedOut {
		t.Errorf("stats.timed_out = %v (%v): the taint analysis after the rounds has no time left", st.TimedOut, err)
	}
	// a limit that is not reached changes nothing
	long := fvParse(t, fvRun(t, ".", "-ir", fvIR, "-rules", fvRules, "-resolve-func-values", "-timeout", "10m"))
	if fs := long.funcValues(t); fs == nil || fs.TimedOut || fs.Supplemented != 2 {
		t.Errorf("stats.func_values with a long limit = %+v", fs)
	}
}

// TestResolveFuncValuesRoundsShareTheLimit: the library call. A round gets what is left of Options.Timeout, and one that is
// stopped by it leaves the graph as it was.
func TestResolveFuncValuesRoundsShareTheLimit(t *testing.T) {
	a := fvLoad(t, fvIR, fvRules)
	opts := taint.DefaultOptions()
	opts.Timeout = time.Nanosecond
	fs := taint.ResolveFuncValues(a.graph, a.l.texts, a.st, a.rules, opts, a.l.loadErrors())
	if !fs.TimedOut || fs.Supplemented != 0 || fs.ZeroAfter != fs.ZeroBefore {
		t.Errorf("stats %+v: want timed out, nothing supplemented", fs)
	}
	_, calls := taint.FuncValueSites(a.graph, a.st)
	for i := range calls {
		if calls[i].Instr.Callee.ValueSupplemented {
			t.Errorf("%s: candidates from a round that did not finish", calls[i].Site.ID)
		}
	}
}

// TestResolveFuncValuesRoundsLimit: when the last round still gave a call new candidates (here the rounds are cut to one: the
// first round always adds candidates, and a second is needed to see that nothing more comes), that call, and only that one, says so (depth_limit, func_values_rounds); it is not
// a note of the whole result.
func TestResolveFuncValuesRoundsLimit(t *testing.T) {
	a := fvLoad(t, fvIR, fvRules)
	fs := taint.ResolveFuncValuesRounds(a.graph, a.l.texts, a.st, a.rules, taint.DefaultOptions(), a.l.loadErrors(), 1)
	if fs.Rounds != 1 || fs.RoundsCut == 0 || fs.RoundsCut > fs.Supplemented {
		t.Fatalf("stats %+v: want one round, and some of the supplemented calls cut", fs)
	}
	res := taint.New(a.graph, a.l.texts, a.st, a.rules, taint.DefaultOptions()).Run()
	var cut, supplemented int
	for _, u := range res.Uncertainties {
		if strings.HasPrefix(u.Detail, "func_values_rounds:") {
			cut++
			if u.Kind != taint.KindDepthLimit || u.Position.Line == 0 {
				t.Errorf("the note of the rounds: %+v", u)
			}
		}
		if strings.HasPrefix(u.Detail, "func_values_supplemented:") {
			supplemented++
		}
	}
	if cut != fs.RoundsCut || supplemented != fs.Supplemented {
		t.Errorf("notes: %d cut (want %d), %d supplemented (want %d)", cut, fs.RoundsCut, supplemented, fs.Supplemented)
	}
}
