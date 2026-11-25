package givensource

import (
	"regexp"
	"slices"
)

func Src(s string) string { return s }

func SrcB(s string) bool { return false }

func Plain(s string) string { return s }

var re = regexp.MustCompile(".+")

// Direct gives the function that the rules declare a source to a function of the library that calls it.
func Direct() string {
	return re.ReplaceAllStringFunc("x", Src)
}

// Variable gives it through a variable.
func Variable() string {
	f := Src
	return re.ReplaceAllStringFunc("x", f)
}

// Closure writes it as a closure: the source is the call in it.
func Closure() string {
	return re.ReplaceAllStringFunc("x", func(s string) string { return Src(s) })
}

// NotSource gives a function that is no source.
func NotSource() string {
	return re.ReplaceAllStringFunc("x", Plain)
}

// NoResult gives it to a row that puts what the function returns nowhere.
func NoResult(xs []string) int {
	return slices.IndexFunc(xs, SrcB)
}
