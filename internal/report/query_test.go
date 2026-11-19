package report

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
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
