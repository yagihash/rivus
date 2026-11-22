package sg

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// SetValueCandidates (-resolve-func-values): the candidates are the functions that were given, up to ir.ValueCandidateMost; more
// are not kept, and the limit is said as it is for a call that valuecalls resolved. The candidates are replaced, not added to.
func TestSetValueCandidates(t *testing.T) {
	f := &Func{ID: "a", Symbol: "pkg.a"}
	p := &Program{ByID: map[string]*Func{"a": f}}
	in := &Instr{Callee: &ir.Callee{Kind: ir.CalleeValue}}

	p.SetValueCandidates(in, []string{"a", "(*T).m"})
	c := in.Callee
	if len(c.ValueCands) != 2 || !c.ValueSupplemented || c.ValueLimit != "" || len(in.CandidateFns) != 1 || len(in.Candidates) != 2 {
		t.Fatalf("two candidates: %+v, fns %d", c, len(in.CandidateFns))
	}
	if c.ValueCands[0].Symbol != "pkg.a" || c.ValueCands[0].Param != -1 || !c.ValueCands[1].Recv || c.ValueCands[1].Symbol != "(*T).m" {
		t.Errorf("candidates: %+v", c.ValueCands)
	}
	if !strings.HasPrefix(c.ValueSupplementedDetail(), "func_values_supplemented:") {
		t.Errorf("detail: %q", c.ValueSupplementedDetail())
	}

	var many []string
	for i := 0; i <= ir.ValueCandidateMost; i++ {
		many = append(many, fmt.Sprintf("f%d", i))
	}
	p.SetValueCandidates(in, many)
	if len(c.ValueCands) != 0 || len(in.CandidateFns) != 0 || c.ValueLimit != ir.ValueCandidateLimit || c.ValueFound != len(many) || c.ValueSupplemented {
		t.Errorf("more than the limit: %+v", c)
	}
	if !strings.HasPrefix(c.ValueLimitDetail(), "candidate_limit:") {
		t.Errorf("detail: %q", c.ValueLimitDetail())
	}
}
