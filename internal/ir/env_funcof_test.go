package ir

import "testing"

// FuncOf: the ID that the alloc says comes first, if the program has the function; else the one at the place that the type
// names, which is one function (a macro that writes a closure twice at one place makes two: none).
func TestClosureIndexFuncOf(t *testing.T) {
	at := func(id string, line int) *Func {
		return &Func{ID: FuncID(id), File: "src/c.rs", Pos: Pos{File: "src/c.rs", Line: line, Col: 20}}
	}
	ix := NewClosureIndex([]*Func{at("a::{closure#0}", 5), at("a::{closure#1}", 5), at("b::{closure#0}", 9)})
	const two = "{closure@src/c.rs:5:20: 5:40}"
	const one = "{closure@src/c.rs:9:20: 9:40}"
	for _, c := range []struct {
		name, env string
		fn        FuncID
		want      FuncID
	}{
		{"the ID tells the two at one place apart", two, "a::{closure#1}", "a::{closure#1}"},
		{"the ID comes before the type", one, "a::{closure#0}", "a::{closure#0}"},
		{"no ID: the type, one function there", one, "", "b::{closure#0}"},
		{"no ID: the type, two functions there", two, "", ""},
		{"an ID of a function that is not there: the type, one function", one, "gone::{closure#0}", "b::{closure#0}"},
		{"an ID of a function that is not there: the type, two functions", two, "gone::{closure#0}", ""},
		{"not a type of an environment", "Foo", "", ""},
	} {
		if got := ix.FuncOf(c.env, c.fn); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	var none *ClosureIndex
	if got := none.FuncOf(one, "x::{closure#0}"); got != "x::{closure#0}" {
		t.Errorf("no index: the ID is taken as it is: %q", got)
	}
	if got := none.FuncOf(one, ""); got != "" {
		t.Errorf("no index, no ID: %q", got)
	}
}
