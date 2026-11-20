package taint

import (
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/testutil"
)

// The limits that stop the analysis of a function are told apart (ADR-054): the work limit is the same
// on any machine, the two clocks are not, and the result says which.
func TestStoppedFunctionsSayWhichLimit(t *testing.T) {
	prog := loadIR(t, testutil.Testdata(), "./cases/inter/...")
	rules := testutil.Rules(t)
	st := prog.sites(rules)

	prefixes := func(opts Options) (map[string]int, Stats) {
		res := prog.engine(st, rules, opts).Run()
		got := map[string]int{}
		for _, u := range res.Uncertainties {
			if u.Kind == KindDepthLimit && u.Func != "" {
				got[strings.SplitN(u.Detail, ":", 2)[0]]++
			}
		}
		return got, res.Stats
	}

	// the work limit: the same on any machine, so the result does not depend on the load
	got, stats := prefixes(func() Options { o := DefaultOptions(); o.Budget = 1; return o }())
	if got["budget"] == 0 || got["func_timeout"] != 0 || got["run_timeout"] != 0 {
		t.Errorf("budget: %v", got)
	}
	if stats.BudgetExhausted == 0 || stats.FuncTimeouts != 0 || stats.RunTimeouts != 0 || stats.LoadDependent {
		t.Errorf("budget: stats %+v", stats)
	}

	// the time of one function: a clock
	got, stats = prefixes(func() Options { o := DefaultOptions(); o.FuncTimeout = time.Nanosecond; return o }())
	if got["func_timeout"] == 0 || got["budget"] != 0 {
		t.Errorf("func timeout: %v", got)
	}
	if stats.FuncTimeouts == 0 || !stats.LoadDependent {
		t.Errorf("func timeout: stats %+v", stats)
	}
}

// A run that converged has no unstable functions: the ones that changed in its last pass settled.
func TestUnstableIsEmptyWhenTheRunConverged(t *testing.T) {
	prog := loadIR(t, testutil.Testdata(), "./cases/inter/...")
	rules := testutil.Rules(t)
	st := prog.sites(rules)
	res := prog.engine(st, rules, DefaultOptions()).Run()
	if !res.Stats.Converged {
		t.Skip("the run did not converge")
	}
	if len(res.Stats.Unstable) != 0 || res.Stats.UnstableCount != 0 {
		t.Errorf("converged, and unstable is %v (%d)", res.Stats.Unstable, res.Stats.UnstableCount)
	}
}
