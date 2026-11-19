package guard

import (
	"os"

	"example.com/cases/lib"
)

// render sanitizes or not, depending on a value the caller passes.
func render(s string, escape bool) string {
	if escape {
		return lib.Sanitize(s)
	}
	return s
}

func Dynamic(flag bool) {
	s := lib.Source() // @source(dyn)
	out := render(s, flag)
	lib.Sink(out) // @sink(dyn)
}

func Constant() {
	s := lib.Source() // @source(const)
	out := render(s, false)
	lib.Sink(out) // @sink(const)
}

func ConstantSafe() {
	s := lib.Source()
	out := render(s, true)
	lib.Sink(out) // @sink(none)
}

// early returns on one side only: the condition says nothing about the
// flow, so it is no conditional node.
func early(s string, skip bool) string {
	if skip {
		return ""
	}
	return s
}

func EarlyReturn(skip bool) {
	s := lib.Source()        // @source(early)
	lib.Sink(early(s, skip)) // @sink(early)
}

// A flow that depends on more conditions than can be told apart.
func deep(s string, a, b, c, d, e bool) string {
	if a {
		if b {
			if c {
				if d {
					if e {
						return s
					}
				}
			}
		}
	}
	return ""
}

func TooManyGuards(a, b, c, d, e bool) {
	s := lib.Source()                // @source(deep)
	lib.Sink(deep(s, a, b, c, d, e)) // @sink(deep)
}

// The same, in a function that sanitizes: here the guards would tell the
// outcomes apart, so losing them is worth saying.
func deepSanitized(s string, a, b, c, d, e bool) string {
	if a {
		if b {
			if c {
				if d {
					if e {
						return lib.Sanitize(s)
					}
				}
			}
		}
	}
	return s
}

func TooManyGuardsInASanitizer(a, b, c, d, e bool) {
	s := lib.Source()                         // @source(deepsan)
	lib.Sink(deepSanitized(s, a, b, c, d, e)) // @sink(deepsan)
}

// A condition inside the same function as the sink.
func SameFunction(ok bool) {
	s := lib.Source() // @source(same)
	if ok {
		lib.Sink(s) // @sink(same)
	}
}

// Where the value that decides comes from.

type options struct{ Escape bool }

var global options

func FromPackageVariable() {
	s := lib.Source()                  // @source(pv)
	lib.Sink(render(s, global.Escape)) // @sink(pv)
}

func FromEnvironment() {
	s := lib.Source()                               // @source(env)
	lib.Sink(render(s, os.Getenv("ESCAPE") == "1")) // @sink(env)
}

func wrap(s string, esc bool) string { return render(s, esc) }

func ThroughWrapper() {
	s := lib.Source()        // @source(wrap)
	lib.Sink(wrap(s, false)) // @sink(wrap)
}

func FromStructField() {
	s := lib.Source() // @source(sf)
	o := options{Escape: false}
	lib.Sink(render(s, o.Escape)) // @sink(sf)
}

// A conversion can change the value: the comparison is on the converted one.
func conv(s string, n int) string {
	if int8(n) == 0 {
		return s
	}
	return ""
}

func ConvertTruncates() {
	s := lib.Source()      // @source(convtrunc)
	lib.Sink(conv(s, 256)) // @sink(convtrunc)
}

func str(s string, r rune) string {
	if string(r) != "a" {
		return s
	}
	return ""
}

func ConvertToString() {
	s := lib.Source()    // @source(convstr)
	lib.Sink(str(s, 98)) // @sink(convstr)
}

// The same element reaches the sink under a guard on a short path and
// without one on a longer path.
func sinkIt(s string) { lib.Sink(s) } // @sink(twoguards)

func twoPaths(x string, flag bool) {
	if flag {
		sinkIt(x)
	}
	sinkIt(x + "!")
}

func TwoGuardsAtTheSameSink() {
	twoPaths(lib.Source(), false) // @source(twoguards)
}

// Guarded and unguarded paths into one cell.
func oneCell(x string, flag bool) {
	a := ""
	if flag {
		a = x
	}
	lib.Sink(a + x) // @sink(onecell)
}

func GuardedAndUnguardedInOneCell() {
	oneCell(lib.Source(), false) // @source(onecell)
}

// An object that is handed down through a recursion, and a field of it that
// decides: the tracing of where the field is set must end.
type config struct{ Escape bool }

func render2(s string, escape bool) string {
	if escape {
		return lib.Sanitize(s)
	}
	return s
}

func leafSink(cfg *config, s string) {
	lib.Sink(render2(s, cfg.Escape)) // @sink(recobj)
}

func walkTree(cfg *config, s string, n int) {
	leafSink(cfg, s)
	walkTree(cfg, s, n-1)
}

func RecursiveObject(n int) {
	cfg := &config{}
	walkTree(cfg, lib.Source(), n) // @source(recobj)
}
