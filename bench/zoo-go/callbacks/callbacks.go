//go:build zooexec

// Package callbacks calls the functions of the standard library whose built-in model has a row of calls (the
// function calls a function it is given), with a closure that records what it was called with, and says what
// came out. cmd/rivus TestZooCallbacksGo holds the rows to it: a row that says the closure is called with the
// receiver, and the closure was called with something else, is wrong; a row that says its result goes to the
// result, and the result has no trace of it, is wrong. The answer is from the run, not from the model.
//
// Each argument that carries data has a marker of its own, in a form that survives what the function does
// to it (a rune of a string is itself). What the closure returns is the marker RET.
package callbacks

import (
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Observed is what one function did.
type Observed struct {
	Symbol string `json:"symbol"`
	// Markers are the markers of the arguments that carry one, by position (recv, arg0 ...).
	Markers map[string]string `json:"markers"`
	// Calls is how many times the closure was called, Params what it was called with (the first call).
	Calls  int      `json:"calls"`
	Params []string `json:"params"`
	// Result is what the call returned, printed.
	Result string `json:"result"`
}

const (
	retRune = 'Ж'
	retStr  = "Ж"
)

type rec struct {
	calls  int
	params []string
}

func (r *rec) see(p ...string) {
	if r.calls == 0 {
		r.params = p
	}
	r.calls++
}

// Run calls every function.
func Run() []Observed {
	var out []Observed
	add := func(sym string, markers map[string]string, r *rec, result string) {
		out = append(out, Observed{Symbol: sym, Markers: markers, Calls: r.calls, Params: r.params, Result: result})
	}

	{
		var r rec
		var once sync.Once
		once.Do(func() { r.see() })
		add("(*sync.Once).Do", nil, &r, "")
	}
	{
		var r rec
		x := []string{"b", "a"}
		sort.Slice(x, func(i, j int) bool { r.see(itoa(i), itoa(j)); return x[i] < x[j] })
		add("sort.Slice", nil, &r, "")
	}
	{
		var r rec
		x := []string{"b", "a"}
		sort.SliceStable(x, func(i, j int) bool { r.see(itoa(i), itoa(j)); return x[i] < x[j] })
		add("sort.SliceStable", nil, &r, "")
	}
	{
		var r rec
		sort.Search(4, func(i int) bool { r.see(itoa(i)); return i >= 2 })
		add("sort.Search", nil, &r, "")
	}
	{
		var r rec
		x := []string{"ARG0-b", "ARG0-a"}
		slices.SortFunc(x, func(a, b string) int { r.see(a, b); return strings.Compare(a, b) })
		add("slices.SortFunc", map[string]string{"arg0": "ARG0"}, &r, "")
	}
	{
		var r rec
		x := []string{"ARG0-b", "ARG0-a"}
		slices.SortStableFunc(x, func(a, b string) int { r.see(a, b); return strings.Compare(a, b) })
		add("slices.SortStableFunc", map[string]string{"arg0": "ARG0"}, &r, "")
	}
	{
		var r rec
		x := []string{"ARG0-b", "ARG0-a"}
		n := slices.IndexFunc(x, func(a string) bool { r.see(a); return false })
		add("slices.IndexFunc", map[string]string{"arg0": "ARG0"}, &r, itoa(n))
	}
	{
		var r rec
		x := []string{"ARG0-b", "ARG0-a"}
		ok := slices.ContainsFunc(x, func(a string) bool { r.see(a); return false })
		add("slices.ContainsFunc", map[string]string{"arg0": "ARG0"}, &r, boolStr(ok))
	}
	{
		var r rec
		n := strings.IndexFunc("ÿ", func(c rune) bool { r.see(string(c)); return false })
		add("strings.IndexFunc", map[string]string{"arg0": "ÿ"}, &r, itoa(n))
	}
	{
		var r rec
		s := strings.Map(func(c rune) rune { r.see(string(c)); return retRune }, "ÿ")
		add("strings.Map", map[string]string{"arg1": "ÿ"}, &r, s)
	}
	{
		var r rec
		s := os.Expand("ÿ-${ÿ}", func(name string) string { r.see(name); return retStr })
		add("os.Expand", map[string]string{"arg0": "ÿ"}, &r, s)
	}
	{
		var r rec
		s := strings.TrimFunc("ÿx", func(c rune) bool { r.see(string(c)); return c == 'x' })
		add("strings.TrimFunc", map[string]string{"arg0": "ÿ"}, &r, s)
	}
	{
		var r rec
		s := strings.FieldsFunc("ÿx", func(c rune) bool { r.see(string(c)); return c == 'x' })
		add("strings.FieldsFunc", map[string]string{"arg0": "ÿ"}, &r, strings.Join(s, ","))
	}
	return out
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

var _ = retStr
