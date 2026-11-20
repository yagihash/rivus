package sg

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// The resolution of the calls of function values (resolveValueCalls, funcsOf, callArgv): which
// functions reach the value through copies, φ functions and the parameters of the function. A Dart
// or Rust call of a closure is resolved here, and the tests of those need a toolchain (yagihash/rivus #26).

// fb writes the IR of one function: a local for every value, and the instructions in one block (or
// more, with branch).
type fb struct {
	f      *ir.Func
	locals int
}

func newFn(id string, nparams int) *fb {
	f := &ir.Func{ID: ir.FuncID(id), Symbol: id, Roots: []ir.BlockID{0}, Exported: true, Blocks: []ir.Block{{}}}
	b := &fb{f: f}
	for i := 0; i < nparams; i++ {
		f.Params = append(f.Params, ir.Param{Name: fmt.Sprintf("p%d", i), Class: ir.ClassFunc})
	}
	for i := 0; i < nparams; i++ {
		l := b.local()
		b.emit(0, ir.Instr{Op: ir.OpParam, Dsts: []ir.LocalID{l}, Index: i})
	}
	return b
}

func (b *fb) local() ir.LocalID {
	b.f.Locals = append(b.f.Locals, ir.Local{Class: ir.ClassFunc})
	b.locals++
	return ir.LocalID(len(b.f.Locals) - 1)
}

func (b *fb) emit(block int, in ir.Instr) {
	b.f.Blocks[block].Instrs = append(b.f.Blocks[block].Instrs, in)
}

func (b *fb) param(i int) ir.LocalID { return ir.LocalID(i) }

// closure makes a closure of the function that captures what it is given.
func (b *fb) closure(block int, fn string, captured ...ir.LocalID) ir.LocalID {
	d := b.local()
	b.emit(block, ir.Instr{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{d}, Func: ir.FuncID(fn), Args: captured})
	return d
}

func (b *fb) copyOf(block int, v ir.LocalID) ir.LocalID {
	d := b.local()
	b.emit(block, ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{d}, Args: []ir.LocalID{v}})
	return d
}

// callValue calls a function value with the arguments.
func (b *fb) callValue(block int, fv ir.LocalID, args ...ir.LocalID) {
	b.emit(block, ir.Instr{Op: ir.OpCall, Args: append([]ir.LocalID{fv}, args...), Callee: &ir.Callee{Kind: ir.CalleeValue}})
}

// callStatic calls a function; recv says that the first argument is the receiver of a method.
func (b *fb) callStatic(block int, fn string, recv bool, args ...ir.LocalID) {
	b.emit(block, ir.Instr{Op: ir.OpCall, Args: args, Callee: &ir.Callee{Kind: ir.CalleeStatic, Func: ir.FuncID(fn), Symbol: fn, Recv: recv}})
}

func (b *fb) done() *ir.Func {
	for i := range b.f.Blocks {
		if b.f.Blocks[i].Term.Kind == "" {
			b.f.Blocks[i].Term = ir.Term{Kind: ir.TermReturn}
		}
	}
	return b.f
}

// leaf is a function that does nothing, the target of a closure.
func leaf(id string, nparams int) *ir.Func { return newFn(id, nparams).done() }

// candidatesOf builds the program and gives, for each call of a function value that has candidates,
// the IDs of them, by the function that holds the call.
func candidatesOf(t *testing.T, funcs ...*ir.Func) map[string][]string {
	t.Helper()
	prog := &ir.Program{Funcs: funcs}
	if err := prog.Validate(); err != nil {
		t.Fatal(err)
	}
	g, err := Build(prog)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, fn := range g.Funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpCall || in.Callee.Kind != ir.CalleeValue {
					continue
				}
				var ids []string
				for _, c := range in.CandidateFns {
					ids = append(ids, c.ID)
				}
				if in.Static != nil {
					ids = append(ids, "static:"+in.Static.ID)
				}
				sort.Strings(ids)
				out[fn.ID] = ids
			}
		}
	}
	return out
}

func same(a, b []string) bool { return strings.Join(a, ",") == strings.Join(b, ",") }

func TestValueCallThroughParameters(t *testing.T) {
	// apply(f, x) { f(x) }, called as apply(closure g, x)
	apply := newFn("apply", 2)
	apply.callValue(0, apply.param(0), apply.param(1))
	caller := newFn("caller", 1)
	cl := caller.closure(0, "g")
	caller.callStatic(0, "apply", false, cl, caller.param(0))

	got := candidatesOf(t, apply.done(), caller.done(), leaf("g", 1))
	if !same(got["apply"], []string{"g"}) {
		t.Errorf("apply: %v", got)
	}
}

func TestValueCallParameterIndexOfAMethodStartsAfterTheReceiver(t *testing.T) {
	// (r).run(f) { f() }: the parameters are the receiver and f; a static call passes the receiver first
	run := newFn("run", 2)
	run.callValue(0, run.param(1))
	run.f.Recv = true
	a := newFn("a", 1)
	cl := a.closure(0, "right")
	a.callStatic(0, "run", true, a.param(0), cl)
	// a call of the function that takes the closure as its first parameter (the receiver) must not count
	decoy := newFn("decoy", 1)
	decoyCl := decoy.closure(0, "wrong")
	decoy.callStatic(0, "run", true, decoyCl, decoy.param(0))

	got := candidatesOf(t, run.done(), a.done(), decoy.done(), leaf("right", 0), leaf("wrong", 0))
	if !same(got["run"], []string{"right"}) {
		t.Errorf("run: %v", got["run"])
	}
}

func TestValueCallArgumentsOfAValueCallDropTheValue(t *testing.T) {
	// the function is reached by a call of a function value: its arguments are the ones after the value
	use := newFn("use", 1)
	use.callValue(0, use.param(0))
	via := newFn("via", 1)
	cl := via.closure(0, "target")
	// a value call whose candidate is `use`: use(h) is called as h2(h) where h2 is a value of `use`
	h2 := via.closure(0, "use")
	via.callValue(0, h2, cl)

	got := candidatesOf(t, use.done(), via.done(), leaf("target", 0))
	if !same(got["use"], []string{"target"}) {
		t.Errorf("use: %v (the value must be dropped from the arguments)", got["use"])
	}
}

func TestValueCallThroughCopiesAndPhis(t *testing.T) {
	// two closures meet at a join through a variable
	f := newFn("f", 1)
	f.f.Blocks = append(f.f.Blocks, ir.Block{ID: 1}, ir.Block{ID: 2}, ir.Block{ID: 3})
	f.f.Blocks[0].Term = ir.Term{Kind: ir.TermBranch, Cond: f.param(0)}
	f.f.Blocks[0].Succs = []ir.Edge{{To: 1, Kind: ir.EdgeTrue}, {To: 2, Kind: ir.EdgeFalse}}
	f.f.Blocks[1].Succs = []ir.Edge{{To: 3, Kind: ir.EdgeNormal}}
	f.f.Blocks[1].Term = ir.Term{Kind: ir.TermJump}
	f.f.Blocks[2].Succs = []ir.Edge{{To: 3, Kind: ir.EdgeNormal}}
	f.f.Blocks[2].Term = ir.Term{Kind: ir.TermJump}
	v := f.local()
	c1 := f.closure(1, "one")
	f.emit(1, ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{v}, Args: []ir.LocalID{c1}})
	c2 := f.closure(2, "two")
	f.emit(2, ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{v}, Args: []ir.LocalID{c2}})
	f.callValue(3, v)
	// a copy of a copy
	g := newFn("g", 0)
	k := g.closure(0, "one")
	k2 := g.copyOf(0, g.copyOf(0, k))
	g.callValue(0, k2)

	got := candidatesOf(t, f.done(), g.done(), leaf("one", 0), leaf("two", 0))
	if !same(got["f"], []string{"one", "two"}) {
		t.Errorf("through a φ: %v", got["f"])
	}
	// a closure that is made and called in the function is a static call, not a candidate
	if !same(got["g"], []string{"static:one"}) {
		t.Errorf("through copies: %v", got["g"])
	}
}

func TestValueCallBindingsOnlyFromTheFunctionThatMadeTheClosure(t *testing.T) {
	bindingsOf := func(g *Program, caller string) map[string]int {
		out := map[string]int{}
		for _, b := range g.ByID[caller].Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpCall && in.Callee.Kind == ir.CalleeValue {
					for f, v := range in.CandidateBindings {
						out[f.ID] = len(v)
					}
				}
			}
		}
		return out
	}
	// the closure made in the function that calls it brings what it captured (here it is made in the
	// branch of a φ, so that the call is not a static one)
	f := newFn("f", 2)
	f.f.Blocks = append(f.f.Blocks, ir.Block{ID: 1}, ir.Block{ID: 2}, ir.Block{ID: 3})
	f.f.Blocks[0].Term = ir.Term{Kind: ir.TermBranch, Cond: f.param(0)}
	f.f.Blocks[0].Succs = []ir.Edge{{To: 1, Kind: ir.EdgeTrue}, {To: 2, Kind: ir.EdgeFalse}}
	f.f.Blocks[1].Succs = []ir.Edge{{To: 3, Kind: ir.EdgeNormal}}
	f.f.Blocks[1].Term = ir.Term{Kind: ir.TermJump}
	f.f.Blocks[2].Succs = []ir.Edge{{To: 3, Kind: ir.EdgeNormal}}
	f.f.Blocks[2].Term = ir.Term{Kind: ir.TermJump}
	v := f.local()
	c1 := f.closure(1, "cap", f.param(1))
	f.emit(1, ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{v}, Args: []ir.LocalID{c1}})
	c2 := f.closure(2, "plain")
	f.emit(2, ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{v}, Args: []ir.LocalID{c2}})
	f.callValue(3, v)

	// a closure that arrives through a parameter: what it captured is not at hand
	callee := newFn("callee", 1)
	callee.callValue(0, callee.param(0))
	maker := newFn("maker", 1)
	mc := maker.closure(0, "cap", maker.param(0))
	maker.callStatic(0, "callee", false, mc)

	capture := leaf("cap", 1)
	capture.Params[0].Captured = true
	prog := &ir.Program{Funcs: []*ir.Func{f.done(), callee.done(), maker.done(), capture, leaf("plain", 0)}}
	if err := prog.Validate(); err != nil {
		t.Fatal(err)
	}
	g, err := Build(prog)
	if err != nil {
		t.Fatal(err)
	}
	if got := bindingsOf(g, "f"); got["cap"] != 1 || len(got) != 1 {
		t.Errorf("made in the function: bindings %v", got)
	}
	if got := bindingsOf(g, "callee"); len(got) != 0 {
		t.Errorf("through a parameter: bindings %v", got)
	}
}

func TestValueCallWithMoreThanEightCandidatesIsLeftUnresolved(t *testing.T) {
	build := func(n int) map[string][]string {
		apply := newFn("apply", 1)
		apply.callValue(0, apply.param(0))
		funcs := []*ir.Func{apply.done()}
		for i := 0; i < n; i++ {
			c := newFn(fmt.Sprintf("caller%d", i), 0)
			cl := c.closure(0, fmt.Sprintf("t%d", i))
			c.callStatic(0, "apply", false, cl)
			funcs = append(funcs, c.done(), leaf(fmt.Sprintf("t%d", i), 0))
		}
		return candidatesOf(t, funcs...)
	}
	if got := build(8); len(got["apply"]) != 8 {
		t.Errorf("eight candidates are kept: %v", got["apply"])
	}
	if got := build(9); len(got["apply"]) != 0 {
		t.Errorf("nine are too many to say: %v", got["apply"])
	}
}

func TestValueCallFollowsParametersTwoCallsUpNotThree(t *testing.T) {
	// l0(f) { f() }, l1(f) { l0(f) }, ..., the closure is made in the caller of the last one
	chain := func(n int) map[string][]string {
		funcs := []*ir.Func{}
		l0 := newFn("l0", 1)
		l0.callValue(0, l0.param(0))
		funcs = append(funcs, l0.done())
		for i := 1; i <= n; i++ {
			l := newFn(fmt.Sprintf("l%d", i), 1)
			l.callStatic(0, fmt.Sprintf("l%d", i-1), false, l.param(0))
			funcs = append(funcs, l.done())
		}
		top := newFn("top", 0)
		cl := top.closure(0, "target")
		top.callStatic(0, fmt.Sprintf("l%d", n), false, cl)
		funcs = append(funcs, top.done(), leaf("target", 0))
		return candidatesOf(t, funcs...)
	}
	// the depth counts the parameters passed on, and stops past three: the closure itself is one more
	if got := chain(2); !same(got["l0"], []string{"target"}) {
		t.Errorf("two functions between: %v", got["l0"])
	}
	if got := chain(3); len(got["l0"]) != 0 {
		t.Errorf("three between is past the limit: %v", got["l0"])
	}
}

func TestValueCallBackThroughRecursionTerminates(t *testing.T) {
	// r(f) { r(f); f() }: the parameter comes back to itself
	r := newFn("r", 1)
	r.callStatic(0, "r", false, r.param(0))
	r.callValue(0, r.param(0))
	top := newFn("top", 0)
	cl := top.closure(0, "target")
	top.callStatic(0, "r", false, cl)

	got := candidatesOf(t, r.done(), top.done(), leaf("target", 0))
	if !same(got["r"], []string{"target"}) {
		t.Errorf("r: %v", got["r"])
	}
}

func TestValueLess(t *testing.T) {
	a := &Value{Kind: KConst, Name: "a"}
	b := &Value{Kind: KResult, Name: "b"}
	if a.Less(a) {
		t.Error("a value is not less than itself")
	}
	if a.Less(b) == b.Less(a) {
		t.Error("two different values are ordered one way")
	}
}

// link() sees through up to eight copies to the closure that is called: it is a static call. Past
// that the value-call resolution finds the same function (a candidate, with no static call).
func TestValueCallOfAClosureSeenThroughEightCopies(t *testing.T) {
	through := func(n int) []string {
		g := newFn("g", 0)
		v := g.closure(0, "target")
		for i := 0; i < n; i++ {
			v = g.copyOf(0, v)
		}
		g.callValue(0, v)
		return candidatesOf(t, g.done(), leaf("target", 0))["g"]
	}
	if got := through(8); !same(got, []string{"static:target"}) {
		t.Errorf("eight copies: %v", got)
	}
	if got := through(9); !same(got, []string{"target"}) {
		t.Errorf("nine copies: %v", got)
	}
}

func TestValueCallCandidatesKnowTheParameterTheyCameThrough(t *testing.T) {
	// apply(f, x) { f(x) }: the callers pass g, and a function that has no body; run(c, f) { if c { f = local }; f() }
	apply := newFn("apply", 2)
	apply.callValue(0, apply.param(0), apply.param(1))
	c1 := newFn("c1", 1)
	c1.callStatic(0, "apply", false, c1.closure(0, "g"), c1.param(0))
	c2 := newFn("c2", 1)
	c2.callStatic(0, "apply", false, c2.closure(0, "ext.Sink"), c2.param(0))
	// a function that picks one itself, and the one it is given
	pick := newFn("pick", 1)
	v := pick.local()
	pick.emit(0, ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{v}, Args: []ir.LocalID{pick.closure(0, "g")}})
	pick.emit(0, ir.Instr{Op: ir.OpCopy, Dsts: []ir.LocalID{v}, Args: []ir.LocalID{pick.param(0)}})
	pick.callValue(0, v)
	c3 := newFn("c3", 0)
	c3.callStatic(0, "pick", false, c3.closure(0, "h"))

	prog := &ir.Program{Funcs: []*ir.Func{apply.done(), c1.done(), c2.done(), pick.done(), c3.done(), leaf("g", 1), leaf("h", 0)}}
	if err := prog.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(prog); err != nil {
		t.Fatal(err)
	}
	cands := func(fn string) []ir.ValueCand {
		for _, f := range prog.Funcs {
			if string(f.ID) != fn {
				continue
			}
			for _, b := range f.Blocks {
				for _, in := range b.Instrs {
					if in.Op == ir.OpCall && in.Callee.Kind == ir.CalleeValue {
						return in.Callee.ValueCands
					}
				}
			}
		}
		return nil
	}
	got := cands("apply")
	if len(got) != 2 || got[0].ID != "ext.Sink" || got[1].ID != "g" || got[0].Param != 0 || got[1].Param != 0 {
		t.Errorf("apply: %+v (both come through parameter 0, the one without a body too)", got)
	}
	if got[0].Symbol != "ext.Sink" {
		t.Errorf("a function without a body is known by its ID: %+v", got[0])
	}
	// g is chosen in the function itself (every caller reaches it); h only comes from the caller
	for _, c := range cands("pick") {
		if want := map[ir.FuncID]int{"g": -1, "h": 0}[c.ID]; c.Param != want {
			t.Errorf("pick: %+v, want parameter %d", c, want)
		}
	}
}
