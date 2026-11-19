package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
)

// The same query on the same report gives the same output, also when sites
// share a position (a field source and the write that follows it) (#10).
func TestQueryOutputIsDeterministic(t *testing.T) {
	pos := ir.Position{File: "a.go", Offset: 10, Line: 1, Column: 1}
	r := &Report{}
	for i := 0; i < 12; i++ {
		src := taint.SiteRef{ID: fmt.Sprintf("s1:%02d", i), Symbol: "src", Position: pos}
		snk := taint.SiteRef{ID: "k1:00", Symbol: "snk", Position: ir.Position{File: "b.go", Offset: 5}}
		r.Findings = append(r.Findings, &taint.Finding{ID: fmt.Sprintf("f%02d", i), Source: src, Sink: snk, Labels: []string{"x"}})
	}
	var first string
	for i := 0; i < 40; i++ {
		res, err := r.Run(Query{Sink: "snk"})
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(res)
		if i == 0 {
			first = string(b)
		} else if string(b) != first {
			t.Fatalf("run %d differs from the first", i)
		}
	}
}

// A matched site with no finding carries its fingerprint, so what query
// printed can be handed to resolve as it is (#11).
func TestQueryMatchedSiteWithoutFindingKeepsFingerprint(t *testing.T) {
	r := &Report{Sites: []*sites.Site{{
		ID: "k1:aa", Role: "sink", Symbol: "snk", Func: "f",
		Position:    ir.Position{File: "a.go", Offset: 1, Line: 1, Column: 1},
		Fingerprint: map[string]string{fingerprint.Content: "abc"},
	}}}
	res, err := r.Run(Query{Sink: "snk"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matched) != 1 || len(res.Matched[0].Fingerprint) == 0 {
		t.Fatalf("matched = %+v, want one site with a fingerprint", res.Matched)
	}
	b, _ := json.Marshal(res.Matched)
	path := filepath.Join(t.TempDir(), "bm.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBookmarks(path); err != nil {
		t.Fatalf("query output is not accepted as bookmarks: %v", err)
	}
}
