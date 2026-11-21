package valuecalls

import "testing"

// The closed world is the crates that the program reaches from its entry, not every crate that is read with it: a
// library that a tool of the same workspace does not link is called from outside by its `pub` functions.

// The entry (tool::main) calls the library: the library's `pub` helper has the callers that are read, and the call is
// narrowed to the closure that is given.
func TestNarrowClosedWorldIsTheLinkedCrate(t *testing.T) {
	prog := resolved(t, "narrow_closed_linked.jsonl")
	c := fnCall(t, prog, "lib::zz::home")
	if got := ids(c); len(got) != 1 || got[0] != "lib::zz::demo::{closure#0}" {
		t.Errorf("want the closure that is given, got %v", got)
	}
}

// The entry does not reach the library: its `pub` helper may be called by a program that is not read.
func TestNarrowClosedWorldIsNotACrateThatIsNotLinked(t *testing.T) {
	prog := resolved(t, "narrow_closed_unlinked.jsonl")
	c := fnCall(t, prog, "lib::zz::home")
	mustKeep(t, c, "lib::zz::demo::{closure#0}", "lib::zz::demo::{closure#1}", "lib::zz::other::{closure#0}")
}
