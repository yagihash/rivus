package report

import (
	"encoding/json"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
)

// With nothing found, the arrays of the output are empty arrays, not null, at
// every detail level (#15).
func TestBuildEmptyArraysAreNotNull(t *testing.T) {
	// what the engine returns when nothing is found: nil slices
	rep := Build(nil, nil, &sites.Result{}, &taint.Result{}, &model.Rules{})
	for _, level := range []string{DetailSnippet, DetailSummary} {
		r, err := rep.WithDetail(level)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"findings", "uncertainties", "packages", "rules"} {
			if string(m[k]) != "[]" {
				t.Errorf("detail %q: %s = %s, want []", level, k, m[k])
			}
		}
	}
}
