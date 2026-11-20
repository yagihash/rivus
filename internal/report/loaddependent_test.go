package report

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/taint"
)

// The summary says that a result depends on the load of the machine when a clock stopped the
// analysis (ADR-054), and says nothing when only the work limit did.
func TestSummarySaysWhenTheResultDependsOnTheLoad(t *testing.T) {
	r := &Report{Stats: taint.Stats{BudgetExhausted: 3}}
	if s := summarize(r); s.LoadDependent || s.LoadDependentNote != "" {
		t.Errorf("the work limit alone: %+v", s)
	}
	r = &Report{Stats: taint.Stats{FuncTimeouts: 2, BudgetExhausted: 1, LoadDependent: true}}
	s := summarize(r)
	if !s.LoadDependent || !strings.Contains(s.LoadDependentNote, "depends on the load of the machine") || !strings.Contains(s.LoadDependentNote, "2 function(s)") {
		t.Errorf("a clock: %+v", s)
	}
}
