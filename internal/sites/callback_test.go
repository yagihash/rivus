package sites

import (
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/testutil"
	"github.com/yagihash/rivus/internal/valuecalls"
)

// enumerateGoResolved is enumerateGo after the calls of function values got their candidates (what the analysis runs on).
func enumerateGoResolved(t testing.TB, dir string, patterns []string, rules *model.Rules) *Result {
	t.Helper()
	p, err := goir.Load(dir, patterns, gossa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	prog, err := goir.Extract(p, goir.Options{})
	if err != nil {
		t.Fatal(err)
	}
	src := ir.MemSource(prog)
	valuecalls.Resolve(src)
	return Enumerate(src, goir.NewTexts(p), rules)
}

func callbackRules(t *testing.T, src string) *model.Rules {
	t.Helper()
	rs, err := model.Parse(strings.NewReader(src), "cb")
	if err != nil {
		t.Fatal(err)
	}
	return model.Merge(rs)
}

// paramsOf lists "function:parameter index" of the sites of the kind callback_param, sorted.
func callbackSitesOf(res *Result) []string {
	var out []string
	for _, s := range res.Sites {
		if s.Kind == model.KindCallbackParam {
			fn := s.Func[strings.LastIndex(s.Func, "/")+1:]
			out = append(out, fn+":"+itoa(s.IR.Param))
		}
	}
	sort.Strings(out)
	return out
}

func TestCallbackParamMarksTheParametersOfTheFunctionThatIsGiven(t *testing.T) {
	rules := callbackRules(t, `
sources:
  - kind: callback_param
    symbol: example.com/cases/cases/cbparam.Route
    operand: 1
    from: 1
    labels: [user_input]
  - kind: callback_param
    symbol: (*example.com/cases/cases/cbparam.Router).Get
    operand: 1
    from: 1
    labels: [user_input]
`)
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, rules)
	got := strings.Join(callbackSitesOf(res), " ")
	// the literal, the named function and the local that holds one: the parameters 1 and 2 (after the first); the
	// method: the parameter 1 (the receiver is not an operand of the rule)
	for _, want := range []string{"cbparam.Literal$1:1", "cbparam.Literal$1:2", "cbparam.named:1", "cbparam.named:2", "cbparam.ThroughLocal$1:1", "cbparam.OnMethod$1:1"} {
		if !strings.Contains(" "+got+" ", " "+want+" ") {
			t.Errorf("no site %s in: %s", want, got)
		}
	}
	for _, not := range []string{":0", "OnMethod$1:2"} {
		if strings.Contains(got, not) {
			t.Errorf("a site %q that the rule does not mark in: %s", not, got)
		}
	}
	// the helper that registers what it is given, and the variable of two functions, are said, not marked
	miss := map[string]string{}
	for k, v := range res.UnresolvedCallbacks {
		miss[string(k.Func)[strings.LastIndex(string(k.Func), "/")+1:]] = v
	}
	for _, fn := range []string{"cbparam.Registers", "cbparam.Chosen"} {
		if miss[fn] == "" {
			t.Errorf("%s: the call whose function is not known is not recorded: %v", fn, miss)
		}
	}
	for _, fn := range []string{"cbparam.Literal", "cbparam.Named", "cbparam.ThroughLocal", "cbparam.OnMethod"} {
		if miss[fn] != "" {
			t.Errorf("%s: recorded as unresolved: %s", fn, miss[fn])
		}
	}
}

func TestCallbackParamFromAndTo(t *testing.T) {
	rules := callbackRules(t, `
sources:
  - kind: callback_param
    symbol: example.com/cases/cases/cbparam.Route
    operand: 1
    from: 1
    to: 1
    labels: [user_input]
`)
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, rules)
	for _, s := range callbackSitesOf(res) {
		if strings.HasSuffix(s, ":2") || strings.HasSuffix(s, ":0") {
			t.Errorf("%s: outside from..to", s)
		}
	}
	got := callbackSitesOf(res)
	for _, want := range []string{"cbparam.Literal$1:1", "cbparam.named:1", "cbparam.ThroughLocal$1:1"} {
		if !hasSite(got, want) {
			t.Errorf("no site %s in %v", want, got)
		}
	}
}

func TestNoCallbackParamRuleChangesNothing(t *testing.T) {
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, callbackRules(t, "sources:\n  - symbol: a.B\n    labels: [x]\n"))
	if len(callbackSitesOf(res)) != 0 || len(res.UnresolvedCallbacks) != 0 {
		t.Errorf("no rule, and a site or a note: %v %v", callbackSitesOf(res), res.UnresolvedCallbacks)
	}
}

func callbackNotes(res *Result) map[string]string {
	miss := map[string]string{}
	for k, v := range res.UnresolvedCallbacks {
		miss[string(k.Func)[strings.LastIndex(string(k.Func), "/")+1:]] = v
	}
	return miss
}

func routeRules(t *testing.T) *model.Rules {
	return callbackRules(t, `
sources:
  - kind: callback_param
    symbol: example.com/cases/cases/cbparam.Route
    operand: 1
    from: 1
    labels: [user_input]
`)
}

func hasSite(sites []string, want string) bool {
	for _, s := range sites {
		if s == want {
			return true
		}
	}
	return false
}

// The function that a helper registers is the one its callers pass (docs/issues/149): two helpers deep, a literal and a named function.
func TestCallbackParamFollowsTheCallersOfAHelper(t *testing.T) {
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, routeRules(t))
	got := callbackSitesOf(res)
	for _, want := range []string{"cbparam.TwoLevels$1:1", "cbparam.TwoLevels$1:2", "cbparam.named2:1", "cbparam.named2:2"} {
		if !hasSite(got, want) {
			t.Errorf("no site %s in %v", want, got)
		}
	}
	if n := callbackNotes(res); n["cbparam.outer2"] != "" || n["cbparam.inner2"] != "" {
		t.Errorf("the helpers are noted although their callers pass functions: %v", n)
	}
}

// A helper that calls itself: the walk up the callers ends, and the function that is passed from outside is found, with no note.
func TestCallbackParamRecursiveHelper(t *testing.T) {
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, routeRules(t))
	if got := callbackSitesOf(res); !hasSite(got, "cbparam.Recursion$1:1") {
		t.Errorf("no site for the function that the recursive helper is given: %v", got)
	}
	if n := callbackNotes(res); n["cbparam.recursive"] != "" {
		t.Errorf("the recursive helper is noted: %s", n["cbparam.recursive"])
	}
}

// The helper that registers on one branch marks the function it is given; the helper that registers nothing does not.
func TestCallbackParamHelperThatDoesNotRegister(t *testing.T) {
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, routeRules(t))
	got := callbackSitesOf(res)
	if !hasSite(got, "cbparam.Maybe$1:1") {
		t.Errorf("no site for the function given to the helper that registers on a branch: %v", got)
	}
	for _, s := range got {
		if strings.HasPrefix(s, "cbparam.Skips") {
			t.Errorf("a site for the function given to a helper that registers nothing: %s", s)
		}
	}
}

// A caller that gives a call through a function value: the first operand of that call is the value, not the helper's first parameter.
func TestCallbackParamCallerThroughAFunctionValue(t *testing.T) {
	res := enumerateGoResolved(t, testutil.Testdata(), []string{"./cases/cbparam"}, routeRules(t))
	if got := callbackSitesOf(res); !hasSite(got, "cbparam.ViaValue$1:1") {
		t.Errorf("no site for the function given through a function value: %v", got)
	}
}

// A caller that gives the same function twice is one target; a caller that cannot be followed is said while the other is marked.
func TestCallbackParamOneCallerNotFollowed(t *testing.T) {
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, routeRules(t))
	got := callbackSitesOf(res)
	n := 0
	for _, s := range got {
		if s == "cbparam.Twice$1:1" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the function given twice: %d sites of it in %v", n, got)
	}
	if !hasSite(got, "cbparam.LiteralCaller$1:1") {
		t.Errorf("the caller that gives a literal is not marked: %v", got)
	}
	if n := callbackNotes(res); n["cbparam.chosenHelper"] == "" {
		t.Errorf("the caller of two functions is not said: %v", n)
	}
}

// The forms of the callers are built once for each function.
func TestCallbackFormCacheBuildsOnce(t *testing.T) {
	res := enumerateGo(t, testutil.Testdata(), []string{"./cases/cbparam"}, routeRules(t))
	if res.callbackFormBuilds == 0 {
		t.Skip("no caller was read")
	}
	if res.callbackFormBuilds > res.callbackFormReads {
		t.Errorf("builds %d > reads %d", res.callbackFormBuilds, res.callbackFormReads)
	}
	if res.callbackFormReads > 0 && res.callbackFormBuilds == res.callbackFormReads {
		t.Errorf("every read built a form (%d): the cache is not used", res.callbackFormReads)
	}
}

// rustClosureIR is the IR of Rust as the extractor writes it for a closure that captures nothing and is given to a method: the
// closure is not a make_closure but an alloc of the type of its environment, converted to a function pointer; the function of the
// closure is the one at the place that the type names (internal/ir/env.go).
const rustClosureIR = `{"schema_version":"rivus.ir/1"}
{"kind":"func","func":{"id":"zoo::reg","symbol":"zoo::reg","file":"src/c.rs","pos":{"line":3,"col":1},"exported":true,"roots":[0],
 "locals":[{"class":"pointer"},{"class":"pointer"},{"class":"string"},{"class":"func"},{"class":"func"}],
 "blocks":[{"id":0,"instrs":[
  {"op":"alloc","dsts":[0],"type":"zoo_dep::Router","class":"struct","pos":{"line":4,"col":5}},
  {"op":"const","dsts":[2],"const":{"kind":"nil"},"pos":{"line":5,"col":5}},
  {"op":"alloc","dsts":[3],"type":"{closure@src/c.rs:5:20: 5:40}","class":"func","pos":{"line":5,"col":20}},
  {"op":"convert","operator":"convert","dsts":[4],"args":[3],"type":"fn(&Request, &str)","pos":{"line":5,"col":20}},
  {"op":"call","dsts":[1],"args":[0,2,4],"callee":{"kind":"static","func":"zoo_dep::Router::get","symbol":"zoo_dep::Router::get"},"pos":{"line":5,"col":5}}],
  "term":{"kind":"return"}}]}}
{"kind":"func","func":{"id":"zoo::reg::{closure#0}","symbol":"zoo::reg::{closure#0}","file":"src/c.rs","pos":{"line":5,"col":20},
 "params":[{"class":"pointer","name":"req"},{"class":"string","name":"id"}],
 "locals":[{"class":"pointer","name":"req"},{"class":"string","name":"id"}],
 "blocks":[{"id":0,"instrs":[{"op":"param","dsts":[0],"index":0,"pos":{"line":5,"col":21}},{"op":"param","dsts":[1],"index":1,"pos":{"line":5,"col":26}}],"term":{"kind":"return"}}]}}
`

// A closure that is given to a method with a callback_param row: its parameters after the first are a source, as those of a named function are.
func TestCallbackParamRustClosure(t *testing.T) {
	prog, err := ir.Read(strings.NewReader(rustClosureIR))
	if err != nil {
		t.Fatal(err)
	}
	rules := callbackRules(t, `
sources:
  - kind: callback_param
    symbol: zoo_dep::Router::get
    operand: 2
    from: 1
    labels: [user_input]
`)
	res := Enumerate(ir.MemSource(prog), ir.NewStreamTexts(prog), rules)
	got := strings.Join(callbackSitesOf(res), " ")
	if !strings.Contains(" "+got+" ", " zoo::reg::{closure#0}:1 ") {
		t.Errorf("the closure that is given to the router is not marked: %q (notes %v)", got, callbackNotes(res))
	}
	if strings.Contains(got, ":0") {
		t.Errorf("the request is marked: %q", got)
	}
}
