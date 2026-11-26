package sites

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// rustClosureTwiceIR is the IR of a macro that writes a closure twice at one place (`F/rust/macro-closure-twice`): two functions
// at the place, and two allocs of an environment of the one type. The type does not say which function an alloc is the value of;
// Instr.Func of the alloc does. Each closure is given to the router by its own function (`reg1`, `reg2`).
const rustClosureTwiceIR = `{"schema_version":"rivus.ir/1"}
{"kind":"func","func":{"id":"zoo::reg1","symbol":"zoo::reg1","file":"src/c.rs","pos":{"line":3,"col":1},"exported":true,"roots":[0],
 "locals":[{"class":"pointer"},{"class":"pointer"},{"class":"string"},{"class":"func"},{"class":"func"}],
 "blocks":[{"id":0,"instrs":[
  {"op":"alloc","dsts":[0],"type":"zoo_dep::Router","class":"struct","pos":{"line":4,"col":5}},
  {"op":"const","dsts":[2],"const":{"kind":"nil"},"pos":{"line":5,"col":5}},
  {"op":"alloc","dsts":[3],"type":"{closure@src/c.rs:5:20: 5:40}","class":"func","func":%q,"pos":{"line":5,"col":20}},
  {"op":"convert","operator":"convert","dsts":[4],"args":[3],"type":"fn(&Request, &str)","pos":{"line":5,"col":20}},
  {"op":"call","dsts":[1],"args":[0,2,4],"callee":{"kind":"static","func":"zoo_dep::Router::get","symbol":"zoo_dep::Router::get"},"pos":{"line":5,"col":5}}],
  "term":{"kind":"return"}}]}}
{"kind":"func","func":{"id":"zoo::reg2","symbol":"zoo::reg2","file":"src/c.rs","pos":{"line":13,"col":1},"exported":true,"roots":[0],
 "locals":[{"class":"pointer"},{"class":"pointer"},{"class":"string"},{"class":"func"},{"class":"func"}],
 "blocks":[{"id":0,"instrs":[
  {"op":"alloc","dsts":[0],"type":"zoo_dep::Router","class":"struct","pos":{"line":14,"col":5}},
  {"op":"const","dsts":[2],"const":{"kind":"nil"},"pos":{"line":15,"col":5}},
  {"op":"alloc","dsts":[3],"type":"{closure@src/c.rs:5:20: 5:40}","class":"func","func":%q,"pos":{"line":15,"col":20}},
  {"op":"convert","operator":"convert","dsts":[4],"args":[3],"type":"fn(&Request, &str)","pos":{"line":15,"col":20}},
  {"op":"call","dsts":[1],"args":[0,2,4],"callee":{"kind":"static","func":"zoo_dep::Router::get","symbol":"zoo_dep::Router::get"},"pos":{"line":15,"col":5}}],
  "term":{"kind":"return"}}]}}
{"kind":"func","func":{"id":"zoo::v::{closure#0}","symbol":"zoo::v::{closure#0}","file":"src/c.rs","pos":{"line":5,"col":20},
 "params":[{"class":"pointer","name":"req"},{"class":"string","name":"id"}],
 "locals":[{"class":"pointer","name":"req"},{"class":"string","name":"id"}],
 "blocks":[{"id":0,"instrs":[{"op":"param","dsts":[0],"index":0,"pos":{"line":5,"col":21}},{"op":"param","dsts":[1],"index":1,"pos":{"line":5,"col":26}}],"term":{"kind":"return"}}]}}
{"kind":"func","func":{"id":"zoo::v::{closure#1}","symbol":"zoo::v::{closure#1}","file":"src/c.rs","pos":{"line":5,"col":20},
 "params":[{"class":"pointer","name":"req"},{"class":"string","name":"id"}],
 "locals":[{"class":"pointer","name":"req"},{"class":"string","name":"id"}],
 "blocks":[{"id":0,"instrs":[{"op":"param","dsts":[0],"index":0,"pos":{"line":5,"col":21}},{"op":"param","dsts":[1],"index":1,"pos":{"line":5,"col":26}}],"term":{"kind":"return"}}]}}
`

// Two closures at one place (a macro): the alloc of the environment says its function, so each one is marked. Without it (an old
// extractor, or a function the program does not have) the type names two functions and none is marked, as a call that is not known.
func TestCallbackParamRustClosureTwiceAtOnePlace(t *testing.T) {
	rules := callbackRules(t, `
sources:
  - kind: callback_param
    symbol: zoo_dep::Router::get
    operand: 2
    from: 1
    labels: [user_input]
`)
	run := func(a, b string) (string, map[IRInstr]string) {
		prog, err := ir.Read(strings.NewReader(fmt.Sprintf(rustClosureTwiceIR, a, b)))
		if err != nil {
			t.Fatal(err)
		}
		res := Enumerate(ir.MemSource(prog), ir.NewStreamTexts(prog), rules)
		return " " + strings.Join(callbackSitesOf(res), " ") + " ", res.UnresolvedCallbacks
	}
	got, notes := run("zoo::v::{closure#0}", "zoo::v::{closure#1}")
	for _, want := range []string{" zoo::v::{closure#0}:1 ", " zoo::v::{closure#1}:1 "} {
		if !strings.Contains(got, want) {
			t.Errorf("%s is not marked: %q (notes %v)", want, got, notes)
		}
	}
	for _, c := range [][2]string{{"", ""}, {"zoo::gone::{closure#0}", "zoo::gone::{closure#1}"}} {
		got, notes := run(c[0], c[1])
		if strings.Contains(got, "{closure#") {
			t.Errorf("%v: a closure is marked although the type names two: %q", c, got)
		}
		if len(notes) == 0 {
			t.Errorf("%v: the callbacks that are not found are not noted", c)
		}
	}
}

// The reason of the note: two closures at one place that the ID does not tell apart say how many there are; a closure that the
// program does not have at all keeps the plain reason. With the ID, nothing is noted.
func TestCallbackParamRustClosureTwiceReason(t *testing.T) {
	rules := callbackRules(t, `
sources:
  - kind: callback_param
    symbol: zoo_dep::Router::get
    operand: 2
    from: 1
    labels: [user_input]
`)
	notes := func(src string, a, b string) []string {
		prog, err := ir.Read(strings.NewReader(fmt.Sprintf(src, a, b)))
		if err != nil {
			t.Fatal(err)
		}
		res := Enumerate(ir.MemSource(prog), ir.NewStreamTexts(prog), rules)
		var out []string
		for _, n := range res.UnresolvedCallbacks {
			out = append(out, n)
		}
		return out
	}
	const want = "one of 2 closures written at the same place (a macro)"
	got := notes(rustClosureTwiceIR, "", "")
	if len(got) != 2 {
		t.Fatalf("want 2 notes: %v", got)
	}
	for _, n := range got {
		if !strings.Contains(n, want) {
			t.Errorf("the reason is not %q: %q", want, n)
		}
	}
	if got := notes(rustClosureTwiceIR, "zoo::v::{closure#0}", "zoo::v::{closure#1}"); len(got) != 0 {
		t.Errorf("the closures are found by their IDs, but noted: %v", got)
	}
	// The type names a place where the program has no function: the reason stays as it was.
	none := strings.ReplaceAll(rustClosureTwiceIR, `"pos":{"line":5,"col":20},
 "params"`, `"pos":{"line":50,"col":20},
 "params"`)
	for _, n := range notes(none, "", "") {
		if strings.Contains(n, "closures written at the same place") || !strings.Contains(n, "not known") {
			t.Errorf("no function at the place: the reason changed: %q", n)
		}
	}
}
