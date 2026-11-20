package taint

import "testing"

// A widened output (the whole of a parameter) joined with the parts of it does not keep them: the elements
// of a summary that is joined again and again must not go up and down.
func TestJoinAbsorbsPartsIntoTheWhole(t *testing.T) {
	mk := func(elems ...OutElem) *Summary {
		s := &Summary{Func: "f", Outputs: []Output{{Kind: OutResult, Index: 0, Elems: elems}}}
		s.Sig = s.signature()
		return s
	}
	whole := OutElem{E: ParamElem(0, "")}
	partA := OutElem{E: ParamElem(0, "/a")}
	partB := OutElem{E: ParamElem(0, "/b")}
	other := OutElem{E: ParamElem(1, "/a")}
	old := mk(whole, other)
	cur := mk(partA, partB, other)
	j := joinSummaries(old, cur)
	if n := len(j.Outputs[0].Elems); n != 2 {
		t.Fatalf("want the whole of parameter 0 and the part of parameter 1, got %d elements", n)
	}
	// joining again changes nothing: the signature is the same
	if again := joinSummaries(j, cur); again.Sig != j.Sig {
		t.Errorf("the join is not stable: %s then %s", j.Sig, again.Sig)
	}
	// a part that has other operations stays
	withMods := OutElem{E: ParamElem(0, "/c").WithMods("x")}
	if n := len(joinSummaries(old, mk(withMods)).Outputs[0].Elems); n != 3 {
		t.Errorf("a part with other operations is not absorbed: %d elements", n)
	}
}
