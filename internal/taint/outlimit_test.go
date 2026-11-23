package taint

import (
	"reflect"
	"testing"

	"github.com/yagihash/rivus/internal/sg"
)

// parts makes n elements that stand for n different parts of parameter 0.
func parts(n int) []OutElem {
	var list []OutElem
	for i := 0; i < n; i++ {
		list = append(list, OutElem{E: ParamElem(0, "f"+itoa(i))})
	}
	return list
}

// An output with exactly maxOutElems elements is not widened and not counted; one more is (#186).
func TestWidenCountsOnlyWhatItMerged(t *testing.T) {
	out, widened := widen(parts(maxOutElems))
	if widened || len(out) != maxOutElems {
		t.Errorf("%d elements: widened=%v, %d elements left; want not widened, unchanged", maxOutElems, widened, len(out))
	}
	out, widened = widen(parts(maxOutElems + 1))
	if !widened || len(out) >= maxOutElems+1 {
		t.Errorf("%d elements: widened=%v, %d elements left; want widened", maxOutElems+1, widened, len(out))
	}
	// more than the limit, but nothing of the same parameter to merge: nothing changes, nothing is counted
	var distinct []OutElem
	for i := 0; i < maxOutElems+1; i++ {
		distinct = append(distinct, OutElem{E: ParamElem(i, "")})
	}
	out, widened = widen(distinct)
	if widened || len(out) != len(distinct) {
		t.Errorf("distinct parameters: widened=%v, %d left; want not widened", widened, len(out))
	}
}

// The functions whose summary has a widened output are listed in one record, sorted, and counted in the stats.
func TestOutLimitRecordListsFunctions(t *testing.T) {
	e := &Engine{summaries: map[*sg.Func]*Summary{
		{ID: "m.b"}: {WidenedOutputs: 2},
		{ID: "m.a"}: {WidenedOutputs: 1},
		{ID: "m.c"}: {},
	}}
	u, funcs, outputs := e.outLimitRecord()
	if u == nil || u.Kind != KindDepthLimit {
		t.Fatalf("record = %+v", u)
	}
	if want := []string{"m.a", "m.b"}; !reflect.DeepEqual(u.Candidates, want) {
		t.Errorf("candidates = %v, want %v", u.Candidates, want)
	}
	if len(u.Detail) < 10 || u.Detail[:10] != "out_limit:" {
		t.Errorf("detail = %q", u.Detail)
	}
	if funcs != 2 || outputs != 3 {
		t.Errorf("funcs, outputs = %d, %d; want 2, 3", funcs, outputs)
	}
	if u, _, _ := (&Engine{summaries: map[*sg.Func]*Summary{{ID: "m.c"}: {}}}).outLimitRecord(); u != nil {
		t.Errorf("no widened output, but a record: %+v", u)
	}
}

// An output that was widened stays marked when it is joined with a small one, even if the join has maxOutElems
// elements or fewer; and the order of old and cur does not matter (#186).
func TestJoinKeepsWidened(t *testing.T) {
	big := &Summary{Func: "m.f", Outputs: []Output{{Kind: OutParam, Index: 1, Elems: []OutElem{{E: ParamElem(0, "")}}, Widened: true}}, WidenedOutputs: 1}
	small := &Summary{Func: "m.f", Outputs: []Output{{Kind: OutParam, Index: 1, Elems: []OutElem{{E: ParamElem(2, "")}}}}}
	a, b := joinSummaries(big, small), joinSummaries(small, big)
	for _, j := range []*Summary{a, b} {
		if len(j.Outputs) != 1 || !j.Outputs[0].Widened || j.WidenedOutputs != 1 {
			t.Errorf("join = %+v, want one widened output", j)
		}
		if len(j.Outputs[0].Elems) > maxOutElems {
			t.Errorf("the test needs a join of %d elements or fewer", maxOutElems)
		}
	}
	if a.Sig != b.Sig || a.WidenedOutputs != b.WidenedOutputs {
		t.Errorf("the join depends on the order")
	}
}
