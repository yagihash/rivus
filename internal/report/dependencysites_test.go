package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
)

// A run that read the body of a dependency says, every time, that sources and sinks were not searched for in it (ADR-099).
// A run without dependency functions says nothing.
func TestSummarySaysThatDependencySitesWereNotSearched(t *testing.T) {
	build := func(n int) *Report {
		return Build(nil, nil, &sites.Result{DependencyFunctions: n}, &taint.Result{}, &model.Rules{})
	}
	if s := build(0).Summary; s.DependencySites != nil {
		t.Errorf("no dependency function: %+v", s.DependencySites)
	}
	r := build(7)
	if d := r.Summary.DependencySites; d == nil || d.Searched || d.DependencyFunctions != 7 {
		t.Fatalf("7 dependency functions: %+v", d)
	}
	for _, detail := range []string{DetailSummary, DetailSnippet} {
		rd, err := r.WithDetail(detail)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(rd)
		if !strings.Contains(string(b), `"dependency_sites":{"searched":false,"dependency_functions":7}`) {
			t.Errorf("detail %s: %s", detail, b)
		}
	}
	if b, _ := json.Marshal(build(0)); strings.Contains(string(b), "dependency_sites") {
		t.Errorf("no dependency function, but the field is there")
	}
}
