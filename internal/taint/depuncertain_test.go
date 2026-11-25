package taint

import (
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/testutil"
)

// The state of a function of a dependency holds no uncertainty (#212): collect leaves the states of the dependencies out
// of the output, so what they hold is never read. The result of the functions of the project is the same.
func TestStateOfADependencyFunctionHoldsNoUncertainty(t *testing.T) {
	run := func(dep func(fn *sg.Func) bool) (*Engine, *Result) {
		prog := loadIR(t, testutil.Testdata(), "./cases/uncertain/...")
		rules := testutil.Rules(t)
		rules.Expand(prog.gossa, 0)
		for _, fn := range prog.sg.Funcs {
			fn.Dep = dep(fn)
		}
		e := prog.engine(prog.sites(rules), rules, DefaultOptions())
		return e, e.Run()
	}
	// the functions that the case defines are the ones that make notes; the second run reads half of them as the
	// functions of a dependency (the odd ones by their place in the order of the ids)
	var ids []string
	e0, res0 := run(func(*sg.Func) bool { return false })
	for fn := range e0.states {
		if strings.Contains(fn.ID, "cases/uncertain") {
			ids = append(ids, fn.ID)
		}
	}
	sort.Strings(ids)
	odd := map[string]bool{}
	for i, id := range ids {
		odd[id] = i%2 == 1
	}
	if len(ids) < 4 || len(res0.Uncertainties) == 0 {
		t.Fatalf("the case does not have what the test needs: %d functions, %d uncertainties", len(ids), len(res0.Uncertainties))
	}
	e1, res1 := run(func(fn *sg.Func) bool { return odd[fn.ID] })
	held, deps := 0, 0
	for fn, s := range e1.states {
		if fn.Dep {
			deps++
			held += len(s.uncertain)
		}
	}
	if deps == 0 {
		t.Fatal("no function was read as a dependency")
	}
	if held != 0 {
		t.Errorf("the states of the dependency functions hold %d uncertainties", held)
	}
	// and the states of the project functions still hold theirs: the output is made of them
	own := 0
	for fn, s := range e1.states {
		if !fn.Dep {
			own += len(s.uncertain)
		}
	}
	if own == 0 || len(res1.Uncertainties) == 0 {
		t.Errorf("the project functions hold %d uncertainties, the result has %d", own, len(res1.Uncertainties))
	}
	// no uncertainty of the result is of a function of the dependency
	for _, u := range res1.Uncertainties {
		if odd[u.Func] {
			t.Errorf("the result has an uncertainty of the dependency function %s: %s", u.Func, u.Kind)
		}
	}
}
