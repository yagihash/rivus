package valuecalls

import "testing"

// A function item is not a candidate of Fn::call. When the walk finds one that flows to the receiver, the list of
// candidates is not the whole of what may be called, and the call is not narrowed (the closure that flows beside
// the function would be the only one left, and the function's flows are not in the list at all).
func TestNarrowIsNotMadeWhenAFunctionItemFlowsToTheReceiver(t *testing.T) {
	prog := resolved(t, "narrow_fn_item.jsonl")
	c := fnCall(t, prog, "lib::zz::home")
	mustKeep(t, c, "lib::zz::demo::{closure#0}", "lib::zz::demo::{closure#1}", "lib::zz::other::{closure#0}")
	if c.NarrowedFrom != 0 {
		t.Errorf("the call was narrowed from %d candidates", c.NarrowedFrom)
	}
}
