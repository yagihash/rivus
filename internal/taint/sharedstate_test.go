package taint

import (
	"strings"
	"testing"
)

// The evidence of a flow through a package variable (Finding.SharedState): who wrote the element,
// how many write the variable, how many touch it. A flow that goes through none has none.
func TestSharedStateEvidence(t *testing.T) {
	res, _ := runCase(t, "uncertain")
	found := false
	for _, f := range res.Findings {
		if strings.Contains(f.Sink.Snippet, "@sink(plain)") && len(f.SharedState) != 0 {
			t.Errorf("a flow that goes through no variable has shared_state: %+v", f.SharedState)
		}
		if !strings.Contains(f.Sink.Snippet, "@sink(global)") {
			continue
		}
		found = true
		if len(f.SharedState) != 1 {
			t.Fatalf("shared_state = %+v, want one place", f.SharedState)
		}
		ss := f.SharedState[0]
		t.Logf("%+v", ss)
		if ss.Place.Kind != "global" || !strings.HasSuffix(ss.Place.Name, "shared") || ss.Writers.All != 1 || ss.Writers.Flow < 1 || ss.Writers.Flow > ss.Writers.All || ss.Readers < 1 {
			t.Errorf("shared_state = %+v", ss)
		}
	}
	if !found {
		t.Fatal("finding not found")
	}
}
