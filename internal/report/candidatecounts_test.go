package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
)

// summary.value_candidates and summary.dispatch_candidates_applied (rivus.result/6) are always there, and carry what the engine counted.
func TestCandidateCountsInSummary(t *testing.T) {
	if SchemaVersion != "rivus.result/6" {
		t.Fatalf("schema version %s: the counts came with rivus.result/6", SchemaVersion)
	}
	empty := Build(nil, nil, &sites.Result{}, &taint.Result{}, &model.Rules{})
	data, err := json.Marshal(empty.Summary)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"value_candidates":{}`) || !strings.Contains(string(data), `"dispatch_candidates_applied":0`) {
		t.Errorf("not there without a count: %s", data)
	}
	r := Build(nil, nil, &sites.Result{}, &taint.Result{ValueCandidates: map[string]int{"21-32": 3, "33+": 2}, DispatchApplied: 41}, &model.Rules{})
	if r.Summary.ValueCandidates["33+"] != 2 || r.Summary.ValueCandidates["21-32"] != 3 || r.Summary.DispatchCandidatesApplied != 41 {
		t.Errorf("%+v %d", r.Summary.ValueCandidates, r.Summary.DispatchCandidatesApplied)
	}
}
