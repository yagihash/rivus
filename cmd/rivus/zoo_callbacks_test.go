package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// The rows of calls of the built-in model, called. A row says that a library function calls a function it is
// given, with what, and where its result goes. The check calls the function with a closure that records what it
// was given, and holds the row to it. The answer is the run (what the standard library does), not the model:
// a row that disagrees is a mistake of the model, and is corrected (not marked as a miss).

// cbObserved is what a harness wrote for one function (the same in the three languages).
type cbObserved struct {
	Symbol string `json:"symbol"`
	// Markers: the markers of the arguments that carry one, by position (recv, arg0 ...; as the model names them).
	Markers map[string]string `json:"markers"`
	// Calls: how many times the closure was called. Params: what it was called with, the first time (printed).
	Calls  int      `json:"calls"`
	Params []string `json:"params"`
	// Result: what the call returned, printed.
	Result string `json:"result"`
}

// cbRet is what the closures return, when they return something that can carry it (a string).
const cbRet = "Ж"

// uncheckedCalls are the rows that no harness calls, with the reason. A row of the model that is neither
// called nor here fails the test: it is a claim nobody holds to the run.
var uncheckedCalls = map[string]string{}

// checkCallbackRows holds the rows of calls of the rule sets (of one language: pick) to the observations.
func checkCallbackRows(t *testing.T, pick func(*model.RuleSet) bool, obs []cbObserved) {
	t.Helper()
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	bySym := map[string][]cbObserved{}
	for _, o := range obs {
		bySym[o.Symbol] = append(bySym[o.Symbol], o)
	}
	seen := map[string]bool{}
	rows := 0
	for _, rs := range sets {
		if !pick(rs) {
			continue
		}
		for _, p := range rs.Propagators {
			if len(p.Calls) == 0 {
				continue
			}
			if why, ok := uncheckedCalls[p.Symbol]; ok {
				_ = why
				continue
			}
			os := bySym[p.Symbol]
			if len(os) == 0 {
				t.Errorf("%s: a row of calls that no harness calls (call it, or write it in uncheckedCalls with the reason)", p.Symbol)
				continue
			}
			seen[p.Symbol] = true
			for i, row := range p.Calls {
				rows++
				for _, o := range os {
					if msg := cbCheck(row, o); msg != "" {
						t.Errorf("%s calls[%d] (%s): %s", p.Symbol, i, fmt.Sprintf("fn: %s with: %v ret: %q", row.Fn, row.With, row.Ret), msg)
					}
				}
			}
		}
	}
	for sym := range bySym {
		if !seen[sym] && uncheckedCalls[sym] == "" {
			t.Errorf("%s: called, but the model has no row of calls for it", sym)
		}
	}
	t.Logf("callbacks: %d rows of calls checked against the run (%d functions called)", rows, len(seen))
}

// cbCheck is what is wrong with the row, given what the run saw ("" when nothing is).
func cbCheck(row model.CallRow, o cbObserved) string {
	if o.Calls == 0 {
		return "the function did not call the closure"
	}
	markers := make([]string, 0, len(o.Markers))
	for _, m := range o.Markers {
		markers = append(markers, m)
	}
	sort.Strings(markers)
	contains := func(text, m string) bool { return strings.Contains(text, m) }
	// what the row says it is called with
	for k, sel := range row.With {
		m, ok := o.Markers[sel]
		if !ok {
			return fmt.Sprintf("with[%d] is %s, which the harness did not give a marker", k, sel)
		}
		if k >= len(o.Params) {
			return fmt.Sprintf("with[%d] is %s, and the closure was called with %d value(s)", k, sel, len(o.Params))
		}
		if !contains(o.Params[k], m) {
			return fmt.Sprintf("with[%d] is %s (marker %q), and the closure was called with %q", k, sel, m, o.Params[k])
		}
	}
	// what it is called with that the row does not say
	for k, p := range o.Params {
		for sel, m := range o.Markers {
			if k < len(row.With) && row.With[k] == sel {
				continue
			}
			if contains(p, m) {
				return fmt.Sprintf("the closure was called with the marker of %s as its parameter %d, and the row does not say so", sel, k)
			}
		}
	}
	// where the result goes
	toResult := strings.HasPrefix(row.Ret, "result")
	if toResult && !contains(o.Result, cbRet) {
		return fmt.Sprintf("ret is %s, and what the closure returned (%q) is not in what the call returned (%q)", row.Ret, cbRet, o.Result)
	}
	if row.Ret == "" && contains(o.Result, cbRet) {
		return fmt.Sprintf("the row drops what the closure returns, and it is in what the call returned (%q)", o.Result)
	}
	return ""
}

// TestZooCallbacksGo calls the functions of the standard library that the model has rows of calls for, with a
// closure that records what it was given (bench/zoo-go/callbacks), and holds the rows to it.
func TestZooCallbacksGo(t *testing.T) {
	dir := zooRoot(t)
	exe := filepath.Join(filepath.Dir(binary), "zoo-callbacks")
	if out, err := zooCommand(dir, nil, "go", "build", "-tags", "zooexec", "-o", exe, "./cmd/callbacks").CombinedOutput(); err != nil {
		t.Fatalf("building the harness: %v\n%s", err, out)
	}
	out, err := zooCommand(dir, nil, exe).Output()
	if err != nil {
		t.Fatalf("running it: %v", err)
	}
	var obs []cbObserved
	if err := json.Unmarshal(out, &obs); err != nil {
		t.Fatal(err)
	}
	checkCallbackRows(t, func(rs *model.RuleSet) bool { return rs.Name == "rivus/go/callbacks@1" }, obs)
}
