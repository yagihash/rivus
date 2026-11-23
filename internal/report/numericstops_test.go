package report

import (
	"encoding/json"
	"testing"

	"github.com/yagihash/rivus/internal/taint"
)

// summary.numeric_stops is always there: zeros and an empty list when nothing was dropped; the top 10 of the
// character reads by count, scope kept; the other stops are counts only.
func TestNumericStopsSummary(t *testing.T) {
	none, _ := json.Marshal(numericStopsOf(nil))
	if string(none) != `{"total":0,"char_reads":{"total":0,"functions":0,"top":[]},"other":{"total":0,"functions":0}}` {
		t.Errorf("empty: %s", none)
	}
	var in []taint.NumericStopCount
	for i := 0; i < 12; i++ {
		in = append(in, taint.NumericStopCount{Func: "f" + string(rune('a'+i)), Dependency: i == 11, Char: i + 1, Other: 2})
	}
	in = append(in, taint.NumericStopCount{Func: "z", Other: 5})
	got := numericStopsOf(in)
	if got.CharReads.Total != 78 || got.CharReads.Functions != 12 || len(got.CharReads.Top) != 10 {
		t.Fatalf("%+v", got)
	}
	if got.Other.Total != 29 || got.Other.Functions != 13 || got.Total != 107 {
		t.Errorf("other: %+v", got)
	}
	if g := got.CharReads.Top[0]; g.Function != "fl" || g.Scope != "dependency" || g.Count != 12 {
		t.Errorf("top: %+v", g)
	}
}
