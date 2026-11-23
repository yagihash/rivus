package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// Every answer of "query" and "resolve" says, at the top of the output (not in stats), whether the analysis that it comes
// from was complete: incomplete and incomplete_reasons, as summary.* of "analyze" (ADR-120). Without them, an answer of
// an index that timed out reads "matched, and nothing reaches it" (#195).

// runCode runs rivus in the workspace and gives the raw output and the exit code.
func (w *workspace) runCode(args ...string) ([]byte, int) {
	w.t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = w.dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			w.t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	return out.Bytes(), code
}

// index makes the JSON index (the graph file and the index are in the workspace, not in the user's cache) and says the exit code.
func (w *workspace) index(name string, extra ...string) (string, int) {
	w.t.Helper()
	args := append([]string{"index", "-taint", "-rules", "rules.yaml", "-graph", name + ".db", "-o", name + ".json"}, extra...)
	_, code := w.runCode(append(args, "./main")...)
	return name + ".json", code
}

// top is the two items of the output, as they stand in the JSON (a missing or null item is not an array).
type top struct {
	Incomplete        *bool     `json:"incomplete"`
	IncompleteReasons *[]string `json:"incomplete_reasons"`
	Matched           []any     `json:"matched"`
	Findings          []any     `json:"findings"`
}

func (w *workspace) top(out []byte) top {
	w.t.Helper()
	var v top
	if err := json.Unmarshal(out, &v); err != nil {
		w.t.Fatalf("%v\n%s", err, out)
	}
	if v.Incomplete == nil || v.IncompleteReasons == nil || *v.IncompleteReasons == nil {
		w.t.Fatalf("incomplete and incomplete_reasons (an array) are always in the output: %s", out)
	}
	return v
}

const sinkSym = "example.com/tmp/lib.Sink"

func wantState(t *testing.T, v top, incomplete bool, reasons []string) {
	t.Helper()
	if *v.Incomplete != incomplete || !reflect.DeepEqual(*v.IncompleteReasons, reasons) {
		t.Errorf("incomplete %v %v, want %v %v", *v.Incomplete, *v.IncompleteReasons, incomplete, reasons)
	}
}

// (a) An index that timed out: the sink is matched, no finding was found, and the answer says that it is not complete.
func TestQueryIndexOfATimedOutAnalysis(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	idx, code := w.index("idx", "-timeout", "1ns")
	if code != exitIncomplete {
		t.Fatalf("index exit code %d", code)
	}
	out, code := w.runCode("query", "-index", idx, "-sink", sinkSym)
	if code != 0 {
		t.Errorf("a query of a saved index ends with 0 (ADR-120), got %d", code)
	}
	v := w.top(out)
	if len(v.Matched) == 0 || len(v.Findings) != 0 {
		t.Errorf("the case is: matched and no finding; got %d, %d", len(v.Matched), len(v.Findings))
	}
	wantState(t, v, true, []string{report.ReasonTimeout})
}

// (b) A complete index: false and [] (an empty array, not null).
func TestQueryIndexOfACompleteAnalysis(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	idx, code := w.index("idx")
	if code != 0 {
		t.Fatalf("index exit code %d", code)
	}
	out, code := w.runCode("query", "-index", idx, "-sink", sinkSym)
	if code != 0 {
		t.Errorf("exit code %d", code)
	}
	wantState(t, w.top(out), false, []string{})
	if !strings.Contains(string(out), `"incomplete_reasons": []`) && !strings.Contains(string(out), `"incomplete_reasons":[]`) {
		t.Errorf("incomplete_reasons is [] in the JSON: %s", out)
	}
}

// (c) A query that analyses the code now says it too, with the exit code of ADR-120.
func TestQueryOfTheSourceSaysIncomplete(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	out, code := w.runCode("query", "-rules", "rules.yaml", "-sink", sinkSym, "./main")
	if code != 0 {
		t.Errorf("exit code %d", code)
	}
	wantState(t, w.top(out), false, []string{})
	out, code = w.runCode("query", "-rules", "rules.yaml", "-timeout", "1ns", "-sink", sinkSym, "./main")
	if code != exitIncomplete {
		t.Errorf("exit code %d, want %d", code, exitIncomplete)
	}
	wantState(t, w.top(out), true, []string{report.ReasonTimeout})
}

// (d) resolve -index says it too.
func TestResolveIndexSaysIncomplete(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	bm, _ := w.bookmarks()
	for _, c := range []struct {
		name       string
		extra      []string
		incomplete bool
		reasons    []string
	}{
		{"idx-ok", nil, false, []string{}},
		{"idx-timeout", []string{"-timeout", "1ns"}, true, []string{report.ReasonTimeout}},
	} {
		idx, _ := w.index(c.name, c.extra...)
		out, code := w.runCode("resolve", "-bookmarks", bm, "-index", idx)
		if code != 0 {
			t.Errorf("%s: exit code %d", c.name, code)
		}
		wantState(t, w.top(out), c.incomplete, c.reasons)
	}
}

// models coverage -index has an entry for a saved index: its output is a coverage, not an answer about reaching, so it is
// not changed here (it is said in the pull request). This only keeps it reading the index.
func TestCoverageIndexStillReads(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	idx, _ := w.index("idx")
	if _, code := w.runCode("models", "coverage", "-index", idx); code != 0 {
		t.Errorf("exit code %d", code)
	}
}

// (e) An index of the older form (no summary.incomplete) takes the state from its stats; one with neither says "unknown".
func TestQueryIndexOfTheOlderForm(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	idx, _ := w.index("idx", "-timeout", "1ns")
	data, err := os.ReadFile(filepath.Join(w.dir, idx))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	write := func(name string, mutate func(m map[string]any)) string {
		c := map[string]any{}
		for k, v := range m {
			c[k] = v
		}
		s := map[string]any{}
		for k, v := range m["summary"].(map[string]any) {
			s[k] = v
		}
		c["summary"] = s
		mutate(c)
		b, _ := json.Marshal(c)
		if err := os.WriteFile(filepath.Join(w.dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
		return name
	}
	dropState := func(c map[string]any) {
		delete(c["summary"].(map[string]any), "incomplete")
		delete(c["summary"].(map[string]any), "incomplete_reasons")
	}
	old := write("old.json", dropState)
	wantState(t, w.top(first(w.runCode("query", "-index", old, "-sink", sinkSym))), true, []string{report.ReasonTimeout})

	none := write("none.json", func(c map[string]any) { dropState(c); delete(c, "stats") })
	wantState(t, w.top(first(w.runCode("query", "-index", none, "-sink", sinkSym))), true, []string{report.ReasonUnknown})
	wantState(t, w.top(first(w.runCode("resolve", "-bookmarks", mustBookmarks(t, w), "-index", none))), true, []string{report.ReasonUnknown})
}

func first(b []byte, _ int) []byte { return b }

func mustBookmarks(t *testing.T, w *workspace) string {
	p, _ := w.bookmarks()
	return p
}
