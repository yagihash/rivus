package intra

import (
	"fmt"
	"strings"

	"example.com/cases/lib"
)

func Direct() {
	s := lib.Source() // @source(direct)
	lib.Sink(s)       // @sink(direct)
}

func Concat() {
	s := lib.Source() // @source(concat)
	t := "prefix-" + s + "-suffix"
	lib.Sink(t) // @sink(concat)
}

func Sprintf() {
	s := lib.Source() // @source(sprintf)
	t := fmt.Sprintf("v=%s", s)
	lib.Sink(t) // @sink(sprintf)
}

func Reassign() {
	s := lib.Source()
	s = "constant"
	lib.Sink(s) // @sink(none)
}

func Sanitized() {
	s := lib.Source()
	c := lib.Sanitize(s)
	lib.Sink(c) // @sink(none)
}

func BranchOneSide(cond bool) {
	s := lib.Source() // @source(branch)
	var t string
	if cond {
		t = s
	} else {
		t = "x"
	}
	lib.Sink(t) // @sink(branch)
}

func SanitizedOnOneBranch(cond bool) {
	s := lib.Source() // @source(onesan)
	if cond {
		s = lib.Sanitize(s)
	}
	lib.Sink(s) // @sink(onesan)
}

func SanitizedOnBothBranches(cond bool) {
	s := lib.Source()
	if cond {
		s = lib.Sanitize(s)
	} else {
		s = lib.Sanitize(s)
	}
	lib.Sink(s) // @sink(none)
}

func Loop() {
	s := lib.Source() // @source(loop)
	acc := ""
	for i := 0; i < 3; i++ {
		acc += s
	}
	lib.Sink(acc) // @sink(loop)
}

func Literal() {
	lib.Sink("constant") // @sink(none)
}

func StringOps() {
	s := lib.Source() // @source(strops)
	u := strings.ToUpper(strings.TrimSpace(s))
	lib.Sink(u) // @sink(strops)
}

func Bytes() {
	b := lib.SourceBytes() // @source(bytes)
	lib.SinkBytes(b)       // @sink(bytes)
}

func BytesToString() {
	b := lib.SourceBytes() // @source(b2s)
	lib.Sink(string(b))    // @sink(b2s)
}

func StringToBytes() {
	s := lib.Source()         // @source(s2b)
	lib.SinkBytes([]byte(s)) // @sink(s2b)
}

func ToInterface() {
	s := lib.Source() // @source(any)
	lib.SinkAny(s)    // @sink(any)
}

func ComparisonCarriesNothing() {
	s := lib.Source()
	ok := s == "x"
	lib.SinkAny(ok) // @sink(none)
}

func LengthCarriesNothing() {
	s := lib.Source()
	lib.SinkAny(len(s)) // @sink(none)
}

func Append() {
	s := lib.Source() // @source(append)
	parts := []string{}
	parts = append(parts, s)
	lib.Sink(parts[0]) // @sink(append)
}

func Join() {
	s := lib.Source() // @source(join)
	lib.Sink(strings.Join([]string{"a", s}, ",")) // @sink(join)
}

func Switch(n int) {
	s := lib.Source() // @source(switch)
	var t string
	switch n {
	case 1:
		t = s
	case 2:
		t = "two"
	default:
		t = "other"
	}
	lib.Sink(t) // @sink(switch)
}

func Defer() {
	s := lib.Source() // @source(defer)
	defer lib.Sink(s) // @sink(defer)
}
