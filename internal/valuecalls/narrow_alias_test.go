package valuecalls

import "testing"

// *p = &evil where p is r1 or r2 (a φ of two pointers to cells), then r1(..): r1 is a or evil (b only reaches r2).
func TestNarrowKeepsAClosureWrittenThroughAPhiOfPointers(t *testing.T) {
	prog := resolved(t, "narrow_phi_alias_write.jsonl")
	c := fnCall(t, prog, "curated::zz::main")
	mustKeep(t, c, "curated::zz::demo::{closure#0}", "curated::zz::demo::{closure#2}")
}

// The control: p = &mut r1 with no φ. The write is through an alias that is followed: the same answer.
func TestNarrowKeepsAClosureWrittenThroughAPointer(t *testing.T) {
	prog := resolved(t, "narrow_alias_write_straight.jsonl")
	c := fnCall(t, prog, "curated::zz::main")
	mustKeep(t, c, "curated::zz::demo::{closure#0}", "curated::zz::demo::{closure#2}")
}
