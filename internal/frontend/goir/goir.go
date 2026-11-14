// Package goir is the Go extractor: it turns a go/ssa program built with
// NaiveForm into the neutral IR (ARCHITECTURE 2.7.4).
//
// NaiveForm keeps local variables in memory (Alloc, Load, Store); the φ
// functions that go/ssa still makes for short-circuit operators are lowered
// to copies into a mutable local at the end of each predecessor. The SSA form
// is built later, by the core.
package goir

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/yagihash/rivus/internal/callgraph"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
)

// Options controls the extraction.
type Options struct {
	// All also extracts the functions of dependencies; by default only the
	// functions of the loaded packages.
	All bool
	// NoCandidates leaves out the call graph: the calls through interfaces
	// and function values then have no candidates.
	NoCandidates bool
	// Implementations lists the interface methods (rule symbols) whose
	// implementations go into the stream.
	Implementations []string
}

// MaxConstString is the longest string constant kept whole; a longer one is
// cut and marked truncated.
const MaxConstString = 1 << 16

// Extract converts the program. The program must have been loaded with
// gossa.Options{NaiveForm: true}.
func Extract(p *gossa.Program, opts Options) (*ir.Program, error) {
	out := &ir.Program{}
	globals := map[string]ir.Global{}
	var graph callgraph.Graph
	var lifted map[string]*ssa.Function
	if !opts.NoCandidates {
		if p.Lifted != nil {
			graph = callgraph.BuildGoOn(p.Lifted)
			lifted = map[string]*ssa.Function{}
			for fn := range ssautil.AllFunctions(p.Lifted) {
				lifted[gossa.FuncID(fn)] = fn
			}
		} else {
			graph = callgraph.BuildGo(p)
		}
	}
	for _, fn := range p.Funcs {
		if !opts.All && !p.Own[fn] {
			continue
		}
		out.Funcs = append(out.Funcs, convertFunc(p, fn, graph, lifted[gossa.FuncID(fn)], globals))
	}
	for _, g := range globals {
		out.Globals = append(out.Globals, g)
	}
	sort.Slice(out.Globals, func(i, j int) bool { return out.Globals[i].Name < out.Globals[j].Name })
	for _, sym := range opts.Implementations {
		impls, err := p.Implementations(sym)
		if err != nil {
			return nil, fmt.Errorf("implementations of %s: %w", sym, err)
		}
		rec := ir.Implementations{Method: sym}
		for _, i := range impls {
			rec.Impls = append(rec.Impls, ir.FuncID(i))
		}
		sort.Slice(rec.Impls, func(i, j int) bool { return rec.Impls[i] < rec.Impls[j] })
		out.Impls = append(out.Impls, rec)
	}
	return out, nil
}

// Func converts one function.
func Func(p *gossa.Program, fn *ssa.Function) *ir.Func {
	return convertFunc(p, fn, nil, nil, map[string]ir.Global{})
}

type conv struct {
	p      *gossa.Program
	fn     *ssa.Function
	out    *ir.Func
	locals map[ssa.Value]ir.LocalID
	tuples map[ssa.Value][]ir.LocalID
	fused  map[ssa.Value]bool // address instructions folded into a load or a store
	addr   map[ssa.Value]addrInfo
	graph  callgraph.Graph
	// liftedCall maps a call of this function to the same call in the
	// lifted build, which the call graph is about.
	liftedCall map[ssa.CallInstruction]ssa.CallInstruction
	global     map[string]ir.Global
	cur        *[]ir.Instr
}

// addrInfo is what a fused address computation contributes to the load or
// store that uses it.
type addrInfo struct {
	base  ir.LocalID
	field string
	index ir.LocalID
	isIdx bool
	pos   ir.Pos
}

func convertFunc(p *gossa.Program, fn *ssa.Function, graph callgraph.Graph, liftedFn *ssa.Function, globals map[string]ir.Global) *ir.Func {
	c := &conv{
		p:      p,
		fn:     fn,
		locals: map[ssa.Value]ir.LocalID{},
		tuples: map[ssa.Value][]ir.LocalID{},
		fused:  map[ssa.Value]bool{},
		addr:   map[ssa.Value]addrInfo{},
		graph:  graph,
		global: globals,
	}
	c.out = &ir.Func{
		ID:     ir.FuncID(gossa.FuncID(fn)),
		Pos:    c.pos(fn.Pos()),
		Blocks: make([]ir.Block, len(fn.Blocks)),
		Symbol: gossa.Symbol(fn),
		Recv:   fn.Signature.Recv() != nil,
		Dep:    !p.Own[fn],
	}
	if o := fn.Object(); o != nil {
		c.out.Exported = o.Exported()
	}
	res := fn.Signature.Results()
	for i := 0; i < res.Len(); i++ {
		c.out.Results = append(c.out.Results, ir.TypeID(typeString(res.At(i).Type())))
	}
	c.out.Roots = []ir.BlockID{0}
	if fn.Recover != nil {
		c.out.Roots = append(c.out.Roots, ir.BlockID(fn.Recover.Index))
	}

	c.matchCalls(liftedFn)
	c.findFused()

	// Parameters first, in block 0.
	var entry []ir.Instr
	c.cur = &entry
	for i, prm := range fn.Params {
		c.out.Params = append(c.out.Params, ir.Param{Name: prm.Name(), Type: ir.TypeID(typeString(prm.Type())), Class: classOf(prm.Type()), Display: displayString(prm.Type())})
		c.emit(ir.Instr{Op: ir.OpParam, Dsts: []ir.LocalID{c.local(prm)}, Index: i, Pos: c.pos(prm.Pos())})
	}
	for i, fv := range fn.FreeVars {
		c.out.Params = append(c.out.Params, ir.Param{Name: fv.Name(), Type: ir.TypeID(typeString(fv.Type())), Class: classOf(fv.Type()), Display: displayString(fv.Type()), Captured: true})
		c.emit(ir.Instr{Op: ir.OpParam, Dsts: []ir.LocalID{c.local(fv)}, Index: len(fn.Params) + i, Pos: c.pos(fv.Pos())})
	}

	for bi, b := range fn.Blocks {
		blk := &c.out.Blocks[bi]
		blk.ID = ir.BlockID(bi)
		if bi == 0 {
			blk.Instrs = entry
		}
		c.cur = &blk.Instrs
		for _, in := range b.Instrs {
			c.instr(in, blk, b)
		}
	}
	c.lowerPhis()
	return c.out
}

// findFused marks the address computations whose only use is as the address
// of a load or a store: they become field_load / field_store and
// index_load / index_store.
func (c *conv) findFused() {
	for _, b := range c.fn.Blocks {
		for _, in := range b.Instrs {
			switch in.(type) {
			case *ssa.FieldAddr, *ssa.IndexAddr:
			default:
				continue
			}
			v := in.(ssa.Value)
			ok := true
			refs := v.Referrers()
			if refs == nil {
				continue
			}
			n := 0
			for _, r := range *refs {
				switch r := r.(type) {
				case *ssa.UnOp:
					if r.Op != token.MUL {
						ok = false
					}
					n++
				case *ssa.Store:
					if r.Addr != v || r.Val == v {
						ok = false
					}
					n++
				case *ssa.DebugRef:
				default:
					ok = false
				}
			}
			// Only an address with one use is folded: with two (x.f += 1) the
			// field is touched once in the source, and addr_of keeps it so.
			if ok && n == 1 {
				c.fused[v] = true
			}
		}
	}
}

func (c *conv) emit(in ir.Instr) { *c.cur = append(*c.cur, in) }

func (c *conv) newLocal(name string, t types.Type) ir.LocalID {
	l := ir.Local{Name: name}
	if t != nil {
		l.Type = ir.TypeID(typeString(t))
		l.Class = classOf(t)
	}
	c.out.Locals = append(c.out.Locals, l)
	return ir.LocalID(len(c.out.Locals) - 1)
}

// local is the local that holds the value of an instruction, a parameter or
// a free variable.
func (c *conv) local(v ssa.Value) ir.LocalID {
	if id, ok := c.locals[v]; ok {
		return id
	}
	if x, ok := v.(*ssa.Extract); ok {
		// An element of a tuple is the local that the instruction wrote.
		id := c.tupleDsts(x.Tuple, tupleLen(x.Tuple))[x.Index]
		c.locals[v] = id
		return id
	}
	name := ""
	switch v := v.(type) {
	case *ssa.Parameter:
		name = v.Name()
	case *ssa.FreeVar:
		name = v.Name()
	case *ssa.Alloc:
		name = v.Comment
	}
	id := c.newLocal(name, v.Type())
	c.locals[v] = id
	return id
}

// val gives the local that holds an operand. A constant, a function, a
// global and a builtin are not instructions: they are materialised where they
// are used.
func (c *conv) val(v ssa.Value) ir.LocalID {
	switch v := v.(type) {
	case *ssa.Const:
		id := c.newLocal("", v.Type())
		c.emit(ir.Instr{Op: ir.OpConst, Dsts: []ir.LocalID{id}, Const: constOf(v), Pos: c.pos(v.Pos())})
		return id
	case *ssa.Function:
		id := c.newLocal("", v.Type())
		c.emit(ir.Instr{Op: ir.OpMakeClosure, Dsts: []ir.LocalID{id}, Func: ir.FuncID(gossa.FuncID(v))})
		return id
	case *ssa.Global:
		c.noteGlobal(v)
		id := c.newLocal("", v.Type())
		c.emit(ir.Instr{Op: ir.OpAddrOf, Dsts: []ir.LocalID{id}, Global: globalName(v)})
		return id
	case *ssa.Builtin:
		id := c.newLocal("", v.Type())
		c.emit(ir.Instr{Op: ir.OpUnsupported, Reason: "a builtin used as a value: " + v.Name()})
		return id
	}
	return c.local(v)
}

func (c *conv) vals(vs []ssa.Value) []ir.LocalID {
	out := make([]ir.LocalID, len(vs))
	for i, v := range vs {
		out[i] = c.val(v)
	}
	return out
}

func globalName(g *ssa.Global) string {
	if g.Pkg != nil {
		return g.Pkg.Pkg.Path() + "." + g.Name()
	}
	return g.Name()
}

func (c *conv) pos(p token.Pos) ir.Pos {
	if !p.IsValid() {
		return ir.Pos{}
	}
	gp := c.p.Position(p)
	return ir.Pos{File: gp.File, Offset: gp.Offset, Line: gp.Line, Col: gp.Column}
}

func (c *conv) dst(v ssa.Value) []ir.LocalID { return []ir.LocalID{c.local(v)} }

// tupleDsts gives the locals of a value that is a tuple: one per element,
// made on first use (an Extract may be met before the instruction).
func (c *conv) tupleDsts(v ssa.Value, n int) []ir.LocalID {
	if ids, ok := c.tuples[v]; ok {
		return ids
	}
	ids := make([]ir.LocalID, n)
	tup, _ := v.Type().(*types.Tuple)
	for i := range ids {
		var t types.Type
		if tup != nil && i < tup.Len() {
			t = tup.At(i).Type()
		}
		ids[i] = c.newLocal("", t)
	}
	c.tuples[v] = ids
	return ids
}

func tupleLen(v ssa.Value) int {
	if tup, ok := v.Type().(*types.Tuple); ok {
		return tup.Len()
	}
	return 1
}

func (c *conv) instr(in ssa.Instruction, blk *ir.Block, b *ssa.BasicBlock) {
	pos := c.pos(in.Pos())
	switch in := in.(type) {
	case *ssa.Alloc:
		t := in.Type()
		if pt, ok := t.Underlying().(*types.Pointer); ok {
			t = pt.Elem()
		}
		c.emit(ir.Instr{Op: ir.OpAlloc, Dsts: c.dst(in), Type: ir.TypeID(typeString(t)), Class: classOf(t), Pos: pos})
	case *ssa.MakeSlice, *ssa.MakeMap, *ssa.MakeChan:
		v := in.(ssa.Value)
		c.emit(ir.Instr{Op: ir.OpAlloc, Dsts: c.dst(v), Type: ir.TypeID(typeString(v.Type())), Pos: pos})
	case *ssa.Phi:
		c.local(in) // lowered later
	case *ssa.Call:
		c.call(in, in, in.Common(), pos, false, false)
	case *ssa.Go:
		c.call(in, nil, in.Common(), pos, true, false)
	case *ssa.Defer:
		c.call(in, nil, in.Common(), pos, false, true)
	case *ssa.RunDefers:
		c.emit(ir.Instr{Op: ir.OpCall, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "go.rundefers"}, Pos: pos})
	case *ssa.BinOp:
		c.emit(ir.Instr{Op: ir.OpBinop, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X), c.val(in.Y)}, Operator: in.Op.String(), Pos: pos})
	case *ssa.UnOp:
		c.unop(in, pos)
	case *ssa.ChangeType, *ssa.Convert, *ssa.ChangeInterface, *ssa.MakeInterface, *ssa.SliceToArrayPointer, *ssa.MultiConvert:
		v := in.(ssa.Value)
		c.emit(ir.Instr{Op: ir.OpConvert, Dsts: c.dst(v), Args: []ir.LocalID{c.val(convOperand(in))}, Operator: convKind(in), Type: ir.TypeID(typeString(v.Type())), Pos: pos})
	case *ssa.Slice:
		c.emit(ir.Instr{Op: ir.OpCopy, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X)}, View: true, Pos: pos})
	case *ssa.FieldAddr:
		base := c.val(in.X)
		name := fieldName(in.X.Type(), in.Field)
		if !c.fused[in] {
			c.emit(ir.Instr{Op: ir.OpAddrOf, Dsts: c.dst(in), Args: []ir.LocalID{base}, Path: []ir.Step{{Kind: ir.StepField, Field: name}}, Pos: pos})
		} else {
			c.addr[in] = addrInfo{base: base, field: name, pos: pos}
		}
	case *ssa.Field:
		c.emit(ir.Instr{Op: ir.OpFieldLoad, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X)}, Field: fieldName(in.X.Type(), in.Field), Pos: pos})
	case *ssa.IndexAddr:
		base, idx := c.val(in.X), c.val(in.Index)
		if !c.fused[in] {
			c.emit(ir.Instr{Op: ir.OpAddrOf, Dsts: c.dst(in), Args: []ir.LocalID{base}, Path: []ir.Step{{Kind: ir.StepIndex}}, Pos: pos})
		} else {
			c.addr[in] = addrInfo{base: base, index: idx, isIdx: true, pos: pos}
		}
	case *ssa.Index:
		c.emit(ir.Instr{Op: ir.OpIndexLoad, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X), c.val(in.Index)}, Pos: pos})
	case *ssa.Lookup:
		dsts := c.dst(in)
		if in.CommaOk {
			dsts = c.tupleDsts(in, 2)
		}
		c.emit(ir.Instr{Op: ir.OpIndexLoad, Dsts: dsts, Args: []ir.LocalID{c.val(in.X), c.val(in.Index)}, Keyed: true, Pos: pos})
	case *ssa.MapUpdate:
		c.emit(ir.Instr{Op: ir.OpIndexStore, Args: []ir.LocalID{c.val(in.Map), c.val(in.Key), c.val(in.Value)}, Keyed: true, Pos: pos})
	case *ssa.Store:
		c.store(in, pos)
	case *ssa.TypeAssert:
		dsts := c.dst(in)
		if in.CommaOk {
			dsts = c.tupleDsts(in, 2)
		}
		c.emit(ir.Instr{Op: ir.OpTypeAssert, Dsts: dsts, Args: []ir.LocalID{c.val(in.X)}, Type: ir.TypeID(typeString(in.AssertedType)), Pos: pos})
	case *ssa.Extract:
		// An alias of the tuple element: see local.
	case *ssa.MakeClosure:
		fn := in.Fn.(*ssa.Function)
		c.emit(ir.Instr{Op: ir.OpMakeClosure, Dsts: c.dst(in), Args: c.vals(in.Bindings), Func: ir.FuncID(gossa.FuncID(fn)), Pos: pos})
	case *ssa.Send:
		c.emit(ir.Instr{Op: ir.OpCall, Args: []ir.LocalID{c.val(in.Chan), c.val(in.X)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "go/chan.send"}, Pos: pos})
	case *ssa.Range:
		c.emit(ir.Instr{Op: ir.OpCall, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "go/range.start"}, Pos: pos})
	case *ssa.Next:
		sym := "go/range.next"
		if in.IsString {
			sym = "go/range.next_string"
		}
		c.emit(ir.Instr{Op: ir.OpCall, Dsts: c.tupleDsts(in, 3), Args: []ir.LocalID{c.val(in.Iter)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: sym}, Pos: pos})
	case *ssa.Select:
		// A select is its communications, one after the other: the sends
		// and the receives. Which one happens is not data.
		n := 1
		if tup, ok := in.Type().(*types.Tuple); ok {
			n = tup.Len()
		}
		elems := c.tupleDsts(in, n)
		k := 0
		for _, st := range in.States {
			if st.Dir == types.SendOnly {
				c.emit(ir.Instr{Op: ir.OpCall, Args: []ir.LocalID{c.val(st.Chan), c.val(st.Send)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "go/chan.send"}, Pos: pos})
				continue
			}
			ok := c.newLocal("", types.Typ[types.Bool])
			c.emit(ir.Instr{Op: ir.OpCall, Dsts: []ir.LocalID{elems[2+k], ok}, Args: []ir.LocalID{c.val(st.Chan)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "go/chan.recv"}, Pos: pos})
			k++
		}
	case *ssa.DebugRef:
	case *ssa.If:
		blk.Term = ir.Term{Kind: ir.TermBranch, Cond: c.val(in.Cond), CondPos: pos, Pos: pos}
		blk.Succs = []ir.Edge{{To: ir.BlockID(b.Succs[0].Index), Kind: ir.EdgeTrue}, {To: ir.BlockID(b.Succs[1].Index), Kind: ir.EdgeFalse}}
	case *ssa.Jump:
		blk.Term = ir.Term{Kind: ir.TermJump, Pos: pos}
		blk.Succs = []ir.Edge{{To: ir.BlockID(b.Succs[0].Index), Kind: ir.EdgeNormal}}
	case *ssa.Return:
		blk.Term = ir.Term{Kind: ir.TermReturn, Values: c.vals(in.Results), Pos: pos}
	case *ssa.Panic:
		blk.Term = ir.Term{Kind: ir.TermThrow, Values: []ir.LocalID{c.val(in.X)}, Pos: pos}
	default:
		c.emit(ir.Instr{Op: ir.OpUnsupported, Reason: fmt.Sprintf("go/ssa instruction %T", in), Pos: pos})
	}
}

// noteGlobal records a package variable the function uses.
func (c *conv) noteGlobal(g *ssa.Global) {
	name := globalName(g)
	if _, ok := c.global[name]; !ok {
		c.global[name] = ir.Global{Name: name, Type: ir.TypeID(typeString(g.Type())), Pos: c.pos(g.Pos())}
	}
}

// matchCalls pairs the calls that go through an interface or a function value
// with the same calls of the same function in the lifted build, which the
// call graph is about. Lifting does not move or drop them, but it does drop
// bookkeeping calls, so they are paired by where they are and what they are
// (in order, when several are at the same place).
func (c *conv) matchCalls(lifted *ssa.Function) {
	if lifted == nil {
		return
	}
	type key struct {
		pos    token.Pos
		invoke bool
		name   string
	}
	keyOf := func(ci ssa.CallInstruction) (key, bool) {
		cc := ci.Common()
		switch {
		case cc.IsInvoke():
			return key{ci.Pos(), true, cc.Method.FullName()}, true
		case cc.StaticCallee() != nil:
			return key{}, false
		}
		if _, ok := cc.Value.(*ssa.Builtin); ok {
			return key{}, false
		}
		return key{ci.Pos(), false, ""}, true
	}
	pool := map[key][]ssa.CallInstruction{}
	for _, b := range lifted.Blocks {
		for _, in := range b.Instrs {
			if ci, ok := in.(ssa.CallInstruction); ok {
				if k, ok := keyOf(ci); ok {
					pool[k] = append(pool[k], ci)
				}
			}
		}
	}
	c.liftedCall = map[ssa.CallInstruction]ssa.CallInstruction{}
	for _, b := range c.fn.Blocks {
		for _, in := range b.Instrs {
			ci, ok := in.(ssa.CallInstruction)
			if !ok {
				continue
			}
			k, ok := keyOf(ci)
			if !ok {
				continue
			}
			if list := pool[k]; len(list) > 0 {
				c.liftedCall[ci] = list[0]
				pool[k] = list[1:]
			}
		}
	}
}
