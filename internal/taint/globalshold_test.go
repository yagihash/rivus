package taint

import "testing"

// A package variable that the rules say holds nothing (os.Stderr) does not tie the code together (ADR-056).
// Inside the function that writes, the variable is like any other; what leaves the function through it is not followed.
func TestGlobalHoldsNothing(t *testing.T) { checkCase(t, "globalshold") }

// The variable that holds nothing is told once, with the functions that wrote to it, and counted in the stats.
func TestGlobalHoldsNothingIsRecorded(t *testing.T) {
	res, _ := runCase(t, "globalshold")
	var rec []*Uncertainty
	for _, u := range res.Uncertainties {
		if u.Kind == KindGlobalHoldsNothing {
			rec = append(rec, u)
		}
	}
	if len(rec) != 1 || rec[0].Symbol != "os.Stderr" || rec[0].AttributableTo != AttributableRivus {
		t.Fatalf("want one uncertainty for os.Stderr, attributable to rivus: %+v", rec)
	}
	if len(rec[0].Candidates) < 2 {
		t.Errorf("the functions that wrote to it are listed (LogIt, WriteAndRead): %v", rec[0].Candidates)
	}
	if d := res.Stats.DroppedGlobals; len(d) != 1 || d[0].Symbol != "os.Stderr" || d[0].Functions < 2 {
		t.Errorf("stats.dropped_globals: %+v", d)
	}
}
