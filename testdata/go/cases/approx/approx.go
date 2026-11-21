package approx

import "example.com/cases/lib"

// A flow that the model marks `approximate` is followed, and the finding that goes through it carries one
// uncertainty of kind model_approximation with the reason of the model (C-8). A flow that is not marked
// leaves none.

func Guess() {
	src := lib.Source() // @source(approx_guess)
	out := lib.External(src)
	lib.Sink(out) // @sink(approx_guess)
}

func Sure() {
	src := lib.Source() // @source(approx_sure)
	out := lib.Exact(src)
	lib.Sink(out) // @sink(approx_sure)
}
