// Package one_branch is a cell of the zoo.
//
// @cell(F/guard/one-branch)
// @shape: only one branch sanitizes (the case runs both branches)
package one_branch

import "zoo/mark"

func init() {
	mark.Case("F/guard/one-branch/vuln", vuln)
	mark.Case("F/guard/one-branch/fixed", fixed)
	mark.Case("F/guard/one-branch/decoy", decoy)
}

func branch(s string, ok bool) {
	if ok {
		s = mark.CleanCmd(s)
	}
	mark.SinkCmd(s) // @sink(guard-one-branch-vuln)
}

func vuln() { // @vuln
	v := mark.Src("guard-one-branch-vuln") // @source(guard-one-branch-vuln)
	branch(v, true)
	branch(v, false)
}

func branchFixed(s string, ok bool) {
	if ok {
		s = mark.CleanCmd(s)
	} else {
		s = mark.CleanCmd(s)
	}
	mark.SinkCmd(s) // @sink(none)
}

func fixed() { // @fixed
	v := mark.Src("guard-one-branch-fixed")
	branchFixed(v, true)
	branchFixed(v, false)
}

func branchDecoy(a, b string, ok bool) {
	if ok {
		a = mark.CleanCmd(a)
	}
	mark.SinkCmd(b) // @sink(none) @nosink(guard-one-branch-decoy)
	mark.SinkSQL(a) // @sink(guard-one-branch-decoy)
}

func decoy() { // @decoy
	v := mark.Src("guard-one-branch-decoy") // @source(guard-one-branch-decoy)
	branchDecoy(v, "constant", true)
	branchDecoy(v, "constant", false)
}
