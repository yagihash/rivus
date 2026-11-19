package taint

import (
	"testing"

	"github.com/yagihash/rivus/internal/testutil"
)

// A visit of a function that is cut short (its budget, or the time limit) does
// not take away what an earlier, complete visit found (#19).
func TestCutRevisitKeepsEarlierFindings(t *testing.T) {
	rules := testutil.Rules(t)
	l := loadIR(t, testutil.Testdata(), "./cases/...")
	e := l.engine(l.sites(rules), rules, DefaultOptions())
	res := e.Run()
	before := len(res.Findings)
	if before == 0 {
		t.Fatal("no findings: the test says nothing")
	}
	var withFindings []string
	for fn, s := range e.states {
		if len(s.findings) > 0 {
			withFindings = append(withFindings, fn.ID)
		}
	}
	if len(withFindings) == 0 {
		t.Fatal("no function holds findings")
	}
	// the second visit of each of them is cut short at once
	e.opts.Budget = 1
	for fn, s := range e.states {
		if len(s.findings) > 0 {
			e.analyzeFunc(fn, nil)
		}
	}
	after := e.collect(res.Stats)
	if len(after.Findings) != before {
		t.Errorf("%d findings before the cut revisits, %d after", before, len(after.Findings))
	}
}
