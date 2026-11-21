package guard

import (
	"os"

	"example.com/cases/lib"
)

// Where the value that decides comes from, when it is not a plain copy of a
// constant: an operation on the way changes the value, so the tracing says
// the value the operation gives, or says it does not know, but never the
// value of the operand.

type rawCfg struct{ raw bool }

func renderEsc(escape bool, s string) string {
	if escape {
		return lib.Sanitize(s)
	}
	return s
}

// escape is !true = false.
func NegatedField() {
	var c rawCfg
	c.raw = true
	s := lib.Source()              // @source(negfield)
	lib.Sink(renderEsc(!c.raw, s)) // @sink(negfield)
}

func level(n int, s string) string {
	if n > 5 {
		return lib.Sanitize(s)
	}
	return s
}

// n is 9-9 = 0.
func ArithmeticOnField() {
	var c struct{ n int }
	c.n = 9
	s := lib.Source()         // @source(arith)
	lib.Sink(level(c.n-9, s)) // @sink(arith)
}

type onCfg struct {
	on    bool
	inner struct{ on bool }
}

func getOnCfg() onCfg { return onCfg{on: os.Getenv("X") != ""} }

func decide(on bool, s string) { lib.Sink(renderEsc(on, s)) } // @sink(whole)

// The whole struct is assigned from a function result.
func WholeFromResult() {
	c := getOnCfg()
	decide(c.on, lib.Source()) // @source(whole)
}

func decideP(on bool, s string) { lib.Sink(renderEsc(on, s)) } // @sink(wholeptr)

// The whole struct is copied from what a pointer points to.
func WholeFromPointer(p *onCfg) {
	var c onCfg
	c = *p
	decideP(c.on, lib.Source()) // @source(wholeptr)
}

func decideE(on bool, s string) { lib.Sink(renderEsc(on, s)) } // @sink(wholeelem)

// The whole struct is copied from an element of an array.
func WholeFromElement() {
	var cs [2]onCfg
	cs[1].on = true
	c := cs[1]
	decideE(c.on, lib.Source()) // @source(wholeelem)
}

func decideI(on bool, s string) { lib.Sink(renderEsc(on, s)) } // @sink(innerpart)

// A part of the field is set: the field is not the zero value.
func InnerFieldSet() {
	var c onCfg
	c.inner.on = true
	decideI(c.inner.on, lib.Source()) // @source(innerpart)
}

func decideZ(on bool, s string) { lib.Sink(renderEsc(on, s)) } // @sink(truezero)

// Nothing sets it: this is the zero value.
func TrulyZero() {
	var c onCfg
	decideZ(c.on, lib.Source()) // @source(truezero)
}

func decideT(on bool, s string) { lib.Sink(renderEsc(on, s)) } // @sink(throughptr)

// Set through a pointer to the field: still a copy of a constant.
func ThroughFieldPointer() {
	var c onCfg
	pc := &c.on
	*pc = true
	decideT(c.on, lib.Source()) // @source(throughptr)
}

func pass(b bool) bool { return b }

// The result of pass(true) is true; the other caller's false is not it.
func ThroughIdentity() {
	s := lib.Source()                  // @source(ident)
	lib.Sink(renderEsc(pass(true), s)) // @sink(ident)
}

func OtherIdentityCaller() bool { return pass(false) }

func setIt(p *bool) { *p = true }

// The address of the variable is passed on: it may have been changed.
func AddressTaken() {
	x := false
	setIt(&x)
	s := lib.Source()         // @source(addr)
	lib.Sink(renderEsc(x, s)) // @sink(addr)
}

// A comparison of a field with a constant.
func ComparedField() {
	var c struct{ n int }
	c.n = 3
	s := lib.Source()                // @source(cmp)
	lib.Sink(renderEsc(c.n == 3, s)) // @sink(cmp)
}

// A loop that steps the value: the values are not 1 and 8 only, so the
// tracing says it does not know.
func SteppedInLoop(k int) {
	a := 1
	for i := 0; i < k; i++ {
		b := a
		if i == 3 {
			b = 7
		}
		a = b + 1
	}
	s := lib.Source()     // @source(loopstep)
	lib.Sink(level(a, s)) // @sink(loopstep)
}
