package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model/builtin"
)

// The pure rows of the built-in model, called (R-03, bench/zoo-go/pure).

// knownWrongPure are the parts (symbol and "value" or "error") that the built-in model says are pure and
// that depend on the marker when they are called: a pure row is wrong for them (R-03). The list is a ratchet:
// a row that is no longer pure (or no longer depends) must be taken out of it, and a new one must be fixed
// or written here with its reference.
var knownWrongPure = []string{}

type zooPart struct {
	Symbol string `json:"symbol"`
	Part   string `json:"part"`
	Text   string `json:"text"`
}

// TestZooPureGo calls the pure rows of the model whose arguments are strings or bytes, and are safe to
// call, with the marker of both variants. A part whose result differs depends on the marker: the row is
// wrong. It is the run that decides (not the model, not the analysis); fixing the rows is R-03.
func TestZooPureGo(t *testing.T) {
	dir := zooRoot(t)
	exe := filepath.Join(filepath.Dir(binary), "zoo-pure")
	if out, err := zooCommand(dir, nil, "go", "build", "-tags", "zooexec", "-o", exe, "./cmd/pure").CombinedOutput(); err != nil {
		t.Fatalf("building the harness: %v\n%s", err, out)
	}
	run := func(variant string) []zooPart {
		cmd := zooCommand(dir, []string{"ZOO_VARIANT=" + variant}, exe)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("running it (variant %s): %v", variant, err)
		}
		var parts []zooPart
		if err := json.Unmarshal(out, &parts); err != nil {
			t.Fatal(err)
		}
		return parts
	}
	v0, v0b, v1 := run("0"), run("0"), run("1")
	if len(v0) != len(v1) || len(v0) != len(v0b) {
		t.Fatalf("the runs have a different number of parts: %d %d", len(v0), len(v1))
	}
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	pure := map[string]bool{}
	for _, rs := range sets {
		for _, p := range rs.Propagators {
			if p.Pure {
				pure[p.Symbol] = true
			}
		}
	}
	var wrong []string
	checked, pureChecked := 0, 0
	calledPure := map[string]bool{}
	for i := range v0 {
		if v0[i].Text != v0b[i].Text {
			t.Errorf("%s %s is not the same in two runs of variant 0 (the call is not deterministic: do not use it here)", v0[i].Symbol, v0[i].Part)
			continue
		}
		checked++
		if !pure[v0[i].Symbol] {
			continue
		}
		pureChecked++
		calledPure[v0[i].Symbol] = true
		if v0[i].Text != v1[i].Text {
			wrong = append(wrong, v0[i].Symbol+" "+v0[i].Part)
		}
	}
	sort.Strings(wrong)
	have := map[string]bool{}
	for _, w := range wrong {
		have[w] = true
	}
	want := map[string]bool{}
	for _, w := range knownWrongPure {
		want[w] = true
		if !have[w] {
			t.Errorf("%q is not wrong now (the row is not pure any more, or the result does not depend on the marker): take it out of knownWrongPure", w)
		}
	}
	for _, w := range wrong {
		if !want[w] {
			t.Errorf("%q: a pure row whose result depends on the marker (R-03). Fix the row, or add it to knownWrongPure", w)
		}
	}
	// The rows that are pure and were not called: the list is kept (bench/zoo-go/pure/unchecked.txt), with
	// the reason of each. A pure row of Go that is neither called here nor in the list fails: it has to be
	// called (a part in bench/zoo-go/pure) or written there with the reason.
	var unchecked []string
	goPure := 0
	for sym := range pure {
		if strings.ContainsAny(sym, "#:") {
			continue // Dart (package:...#) and Rust (a::b)
		}
		goPure++
		if !calledPure[sym] {
			unchecked = append(unchecked, sym)
		}
	}
	sort.Strings(unchecked)
	raw, err := os.ReadFile(filepath.Join(dir, "pure", "unchecked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line = strings.TrimSpace(line); line != "" {
			listed[line] = true
		}
	}
	gotU := map[string]bool{}
	for _, u := range unchecked {
		gotU[u] = true
		if !listed[u] {
			t.Errorf("%s is a pure row of Go that the run did not call and bench/zoo-go/pure/unchecked.txt does not list: call it in the harness, or list it there with the reason", u)
		}
	}
	for u := range listed {
		if !gotU[u] {
			t.Errorf("%s is in bench/zoo-go/pure/unchecked.txt but is called now, or is not pure any more: take it out", u)
		}
	}
	t.Logf("zoo-go pure Go rows: %d, called %d, not called %d (bench/zoo-go/pure/unchecked.txt)", goPure, len(calledPure), len(unchecked))
	t.Logf("zoo-go pure: %d parts called, %d of them of pure rows (the model has %d pure rows), %d wrong (R-03)", checked, pureChecked, len(pure), len(wrong))
}
