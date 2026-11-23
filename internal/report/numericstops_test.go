package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/taint"
)

// summary.numeric_stops is always there: zeros and an empty list when nothing was dropped, and the top 10 by count otherwise.
func TestNumericStopsSummary(t *testing.T) {
	none, _ := json.Marshal(numericStopsOf(nil))
	if string(none) != `{"total":0,"functions":0,"top":[]}` {
		t.Errorf("empty: %s", none)
	}
	var in []taint.NumericStopCount
	for i := 0; i < 12; i++ {
		in = append(in, taint.NumericStopCount{Func: "f" + string(rune('a'+i)), Dependency: i == 11, Count: i + 1})
	}
	got := numericStopsOf(in)
	if got.Total != 78 || got.Functions != 12 || len(got.Top) != 10 {
		t.Fatalf("%+v", got)
	}
	if got.Top[0].Function != "fl" || got.Top[0].Scope != "dependency" || got.Top[0].Count != 12 {
		t.Errorf("top: %+v", got.Top[0])
	}
	if strings.Contains(got.Top[9].Function, "fb") {
		t.Errorf("the 11th is listed: %+v", got.Top)
	}
}
