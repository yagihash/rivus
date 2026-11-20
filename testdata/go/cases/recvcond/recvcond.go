// Package recvcond holds sinks whose condition is on the type of the receiver, or of an argument of a
// method: when_arg_type counts the arguments as args does, without the receiver (yagihash/rivus #56).
package recvcond

import "example.com/cases/lib"

type Out interface{ Put(s string) }

type Web struct{}

func (Web) Put(s string) {}

type Log struct{}

func (Log) Put(s string) {}

// --- the receiver is an interface; the sink is the call whose value is a Web ---

func ToWeb() {
	x := lib.Source() // @source(recv-web)
	Out(Web{}).Put(x) // @sink(recv-web)
}

func ToLog() {
	x := lib.Source()
	Out(Log{}).Put(x) // a Log is not what the rule is about
}

// --- an argument of a method: arg 0 is the first argument, not the receiver ---

type Box struct{}

func (Box) Fill(dst Out, s string) {}

func FillWeb() {
	x := lib.Source() // @source(arg-web)
	Box{}.Fill(Out(Web{}), x) // @sink(arg-web)
}

func FillLog() {
	x := lib.Source()
	Box{}.Fill(Out(Log{}), x)
}
