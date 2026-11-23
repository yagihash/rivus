package main

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"sort"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// The end of the run says what the memory limit did (ADR-121, "実行の終わりの報告"): summary.memory_peak_bytes,
// memory_limit_exceeded (peak > limit) and gc_cpu_fraction. Only read; the result does not change.

type memSummary struct {
	Limit    int64    `json:"memory_limit_bytes"`
	Source   string   `json:"memory_limit_source"`
	Peak     int64    `json:"memory_peak_bytes"`
	Exceeded *bool    `json:"memory_limit_exceeded"`
	GCCPU    *float64 `json:"gc_cpu_fraction"`
}

// genTarget builds the synthetic target of bench/go-scale (it holds a few tens of MB while it is analysed).
func genTarget(t *testing.T) string {
	t.Helper()
	gen := t.TempDir() + "/gen"
	build := exec.Command("go", "build", "-o", gen, "./bench/go-scale/gen")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the generator: %v\n%s", err, out)
	}
	dir := t.TempDir()
	if out, err := exec.Command(gen, "-shapes", "all", "-out", dir).CombinedOutput(); err != nil {
		t.Fatalf("gen: %v\n%s", err, out)
	}
	return dir
}

func analyzeSummary(t *testing.T, dir string, args ...string) memSummary {
	t.Helper()
	cmd := exec.Command(binary, append([]string{"analyze", "-workers", "1", "-rules", "rules.yaml"}, append(args, "./...")...)...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze %v: %v", args, err)
	}
	var rep struct {
		Summary memSummary `json:"summary"`
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	return rep.Summary
}

func TestMemoryLimitExceededReport(t *testing.T) {
	t.Parallel()
	dir := genTarget(t)

	// (a) gc_cpu_fraction is always there, in [0, 1]: also with no limit
	for _, lim := range []string{"off", "64GiB", "16MiB"} {
		s := analyzeSummary(t, dir, "-max-memory", lim)
		if s.GCCPU == nil || *s.GCCPU < 0 || *s.GCCPU > 1 {
			t.Errorf("-max-memory %s: gc_cpu_fraction = %v", lim, s.GCCPU)
		}
		if s.Peak <= 0 {
			t.Errorf("-max-memory %s: memory_peak_bytes = %d", lim, s.Peak)
		}
	}
	// (b) a small limit is passed
	s := analyzeSummary(t, dir, "-max-memory", "16MiB")
	if s.Exceeded == nil || !*s.Exceeded || s.Peak <= s.Limit {
		t.Errorf("16MiB: exceeded %v, peak %d, limit %d", s.Exceeded, s.Peak, s.Limit)
	}
	// (c) a large one is not
	s = analyzeSummary(t, dir, "-max-memory", "64GiB")
	if s.Exceeded == nil || *s.Exceeded || s.Peak > s.Limit {
		t.Errorf("64GiB: exceeded %v, peak %d, limit %d", s.Exceeded, s.Peak, s.Limit)
	}
	// (d) no limit: the item is not there
	s = analyzeSummary(t, dir, "-max-memory", "off")
	if s.Exceeded != nil {
		t.Errorf("off: memory_limit_exceeded = %v, want none", *s.Exceeded)
	}
}

// (e) the result is the same with and without a limit that is passed.
func TestMemoryLimitExceededKeepsResult(t *testing.T) {
	t.Parallel()
	dir := genTarget(t)
	type result struct {
		ids   []string
		elems any
	}
	get := func(limit string) result {
		cmd := exec.Command(binary, "analyze", "-sites", "-workers", "1", "-rules", "rules.yaml", "-max-memory", limit, "./...")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze -max-memory %s: %v", limit, err)
		}
		var rep struct {
			Findings []struct {
				ID string `json:"id"`
			} `json:"findings"`
			Stats map[string]any `json:"stats"`
		}
		if err := json.Unmarshal(out, &rep); err != nil {
			t.Fatal(err)
		}
		var r result
		for _, f := range rep.Findings {
			r.ids = append(r.ids, f.ID)
		}
		sort.Strings(r.ids)
		r.elems = rep.Stats["summary_elems"]
		return r
	}
	a, b := get("16MiB"), get("off")
	if len(a.ids) == 0 {
		t.Fatal("no findings: the check would prove nothing")
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("the measuring changed the result:\n16MiB: %d findings, summary_elems %v\noff: %d findings, summary_elems %v", len(a.ids), a.elems, len(b.ids), b.elems)
	}
}

// (f) models coverage says incomplete and incomplete_reasons at the top, as query and resolve do (ADR-120, #195): from the
// analysis of the code now and from an index, complete (false, []) or not.
func TestModelsCoverageSaysIncomplete(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)

	out, code := w.runCode("models", "coverage", "-rules", "rules.yaml", "./main")
	if code != 0 {
		t.Errorf("now, complete: exit code %d", code)
	}
	wantState(t, w.top(out), false, []string{})
	out, code = w.runCode("models", "coverage", "-rules", "rules.yaml", "-timeout", "1ns", "./main")
	if code != exitIncomplete {
		t.Errorf("now, timeout: exit code %d, want %d", code, exitIncomplete)
	}
	wantState(t, w.top(out), true, []string{report.ReasonTimeout})

	idx, _ := w.index("idx-ok")
	out, code = w.runCode("models", "coverage", "-index", idx)
	if code != 0 {
		t.Errorf("index, complete: exit code %d", code)
	}
	wantState(t, w.top(out), false, []string{})
	idx, _ = w.index("idx-timeout", "-timeout", "1ns")
	out, code = w.runCode("models", "coverage", "-index", idx)
	if code != 0 {
		t.Errorf("index, timeout: a saved index ends with 0 (ADR-120), got %d", code)
	}
	wantState(t, w.top(out), true, []string{report.ReasonTimeout})
}
