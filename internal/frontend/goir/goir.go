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
	"strings"

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
	// Emit, when it is set, is given each function as it is converted, and Extract does not keep it: the program it
	// returns has no Funcs. The functions come in the order of the conversion, not sorted by ID (the caller that
	// needs the order sorts what it keeps: irspill.Builder.SortByID).
	Emit func(*ir.Func)
}

// MaxConstString is the longest string constant kept whole; a longer one is
// cut and marked truncated.
const MaxConstString = 1 << 16

// Load reads the code the way the analysis does: the program of the naive form, and the table of the lifted call graph made while
// loading (Extract needs it for the candidates of a call). o.NaiveForm, o.WithLifted and o.OnLifted are set here.
func Load(dir string, patterns []string, o gossa.Options) (*gossa.Program, error) {
	o.NaiveForm, o.WithLifted, o.OnLifted = true, true, LiftedTable
	return gossa.Load(dir, patterns, o)
}

// Extract converts the program. It takes the bodies of the go/ssa functions
// it converts (releaseBody), so a program is converted once. The program must
// have been loaded by Load (or with gossa.Options{NaiveForm: true, WithLifted: true, OnLifted: LiftedTable}), except with NoCandidates.
func Extract(p *gossa.Program, opts Options) (*ir.Program, error) {
	out := &ir.Program{}
	globals := map[string]ir.Global{}
	seen := map[ir.FuncID]bool{}
	var lifted map[string][][]liftedCall
	if !opts.NoCandidates {
		// The candidates come from one place only: the table made while
		// loading (gossa.Options.OnLifted = LiftedTable, see Load). A call graph
		// made on the program of the naive form would say something else of the
		// closures that bind one method.
		t, ok := p.LiftedResult.(map[string][][]liftedCall)
		if !ok {
			return nil, fmt.Errorf("goir: the program was loaded without the table of the lifted call graph (use goir.Load, or gossa.Options.OnLifted = goir.LiftedTable)")
		}
		lifted = t
		p.LiftedResult = nil
	}
	// With All, a function of a dependency is converted only when something
	// the project does can reach it (a call, a candidate of a call through an
	// interface or a function value, a function named as a value): the rest
	// cannot change the analysis, and converting all of a dependency is most
	// of the memory.
	byID := map[ir.FuncID]*ssa.Function{}
	var work []*ssa.Function
	push := func(id ir.FuncID, fn *ssa.Function) {
		if !seen[id] {
			seen[id] = true
			work = append(work, fn)
		}
	}
	var impls []ir.Implementations
	for _, sym := range opts.Implementations {
		is, err := p.Implementations(sym)
		if err != nil {
			return nil, fmt.Errorf("implementations of %s: %w", sym, err)
		}
		rec := ir.Implementations{Method: sym}
		for _, i := range is {
			rec.Impls = append(rec.Impls, ir.FuncID(i))
		}
		sort.Slice(rec.Impls, func(i, j int) bool { return rec.Impls[i] < rec.Impls[j] })
		impls = append(impls, rec)
	}
	for _, fn := range p.Funcs {
		// Synthetic wrappers (thunks) of the same method are made once per
		// package that uses them, under one name: they are the same function
		// (the first, the biggest, stands for it).
		id := ir.FuncID(gossa.FuncID(fn))
		if _, ok := byID[id]; ok {
			continue
		}
		byID[id] = fn
		if p.Own[fn] || !opts.All {
			push(id, fn)
		}
	}
	if opts.All {
		for _, rec := range impls {
			for _, id := range rec.Impls {
				if fn := byID[id]; fn != nil {
					push(id, fn)
				}
			}
		}
	}
	for len(work) > 0 {
		fn := work[len(work)-1]
		work = work[:len(work)-1]
		if !opts.All && !p.Own[fn] {
			continue
		}
		f := convertFunc(p, fn, lifted[gossa.FuncID(fn)], globals)
		releaseBody(fn)
		if opts.Emit != nil {
			opts.Emit(f)
		} else {
			out.Funcs = append(out.Funcs, f)
		}
		if !opts.All {
			continue
		}
		visit := func(id ir.FuncID) {
			if id != "" && byID[id] != nil {
				push(id, byID[id])
			}
		}
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				visit(in.Func)
				if c := in.Callee; c != nil {
					visit(c.Func)
					for _, id := range c.Candidates {
						visit(id)
					}
				}
			}
		}
	}
	sort.Slice(out.Funcs, func(i, j int) bool { return out.Funcs[i].ID < out.Funcs[j].ID })
	for _, g := range globals {
		out.Globals = append(out.Globals, g)
	}
	sort.Slice(out.Globals, func(i, j int) bool { return out.Globals[i].Name < out.Globals[j].Name })
	out.Impls = impls
	out.TypeAliases = typeAliases(p)
	return out, nil
}

// releaseBody lets go of the body of a function that has been converted: its
// blocks and instructions, parameters, free variables and locals. The
// conversion of another function reads the signature, the name and the
// position of what it calls, never its body, and the lifted SSA (which the
// call graph is about) is another set of functions. The go/ssa program holds
// the bodies, with the functions that have not been converted yet, until the
// extraction ends.
//
// Two things stay, because go/ssa reads them for the name (fn.String, which
// is the FuncID of a function that another function calls or makes a closure
// of): the AnonFuncs of a function (the number of an anonymous function is its
// place in them), and the one free variable of a "$bound" method value (the
// receiver type is its type).
func releaseBody(fn *ssa.Function) {
	fn.Blocks, fn.Params, fn.Locals, fn.Recover = nil, nil, nil, nil
	if !(len(fn.FreeVars) == 1 && strings.HasSuffix(fn.Name(), "$bound")) {
		fn.FreeVars = nil
	}
}

// callKey is what pairs a call of a function with the same call of the same
// function in the lifted SSA.
type callKey struct {
	pos    token.Pos
	invoke bool
	name   string
}

// liftedCall is a call of the lifted SSA that goes through an interface or
// a function value, with the functions the call graph says it may reach.
type liftedCall struct {
	key   callKey
	cands []ir.FuncID
}

// LiftedTable is gossa.Options.OnLifted for the extraction: it makes the table
// of liftedTable while the program is loaded, so that the lifted program is
// let go of before the program of the naive form is built.
func LiftedTable(prog *ssa.Program) any { return liftedTable(prog) }

// liftedTable asks the call graph of the lifted SSA what each of its calls
// through an interface or a function value may reach, and keeps the answer
// (in the order of the calls) by function. The lifted build is not read after
// this.
func liftedTable(prog *ssa.Program) map[string][][]liftedCall {
	graph := callgraph.BuildGoOn(prog)
	table := map[string][][]liftedCall{}
	for fn := range ssautil.AllFunctions(prog) {
		var calls []liftedCall
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				ci, ok := in.(ssa.CallInstruction)
				if !ok {
					continue
				}
				k, ok := keyOf(ci)
				if !ok {
					continue
				}
				var ids []ir.FuncID
				for _, callee := range graph.Callees(ci) {
					ids = append(ids, ir.FuncID(gossa.FuncID(callee)))
				}
				calls = append(calls, liftedCall{k, ids})
			}
		}
		// Several functions can have one name (the closure that binds a
		// method, made once for each place that takes it): they are one
		// function in the IR, and what the call graph says of each of them is
		// joined (which one a map gave last is not an answer).
		if len(calls) > 0 {
			id := gossa.FuncID(fn)
			table[id] = append(table[id], calls)
		}
	}
	return table
}

func keyOf(ci ssa.CallInstruction) (callKey, bool) {
	cc := ci.Common()
	switch {
	case cc.IsInvoke():
		return callKey{ci.Pos(), true, cc.Method.FullName()}, true
	case cc.StaticCallee() != nil:
		return callKey{}, false
	}
	if _, ok := cc.Value.(*ssa.Builtin); ok {
		return callKey{}, false
	}
	return callKey{ci.Pos(), false, ""}, true
}

// Func converts one function.
func Func(p *gossa.Program, fn *ssa.Function) *ir.Func {
	return convertFunc(p, fn, nil, map[string]ir.Global{})
}

type conv struct {
	p      *gossa.Program
	fn     *ssa.Function
	out    *ir.Func
	locals map[ssa.Value]ir.LocalID
	tuples map[ssa.Value][]ir.LocalID
	fused  map[ssa.Value]bool // address instructions folded into a load or a store
	addr   map[ssa.Value]addrInfo
	// liftedCall maps a call of this function to what the call graph says of
	// the same call in the lifted SSA (one answer for each lifted function
	// of the name).
	liftedCall map[ssa.CallInstruction][][]ir.FuncID
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

func convertFunc(p *gossa.Program, fn *ssa.Function, liftedFns [][]liftedCall, globals map[string]ir.Global) *ir.Func {
	c := &conv{
		p:      p,
		fn:     fn,
		locals: map[ssa.Value]ir.LocalID{},
		tuples: map[ssa.Value][]ir.LocalID{},
		fused:  map[ssa.Value]bool{},
		addr:   map[ssa.Value]addrInfo{},
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
	c.out.Entry = isEntry(fn)
	res := fn.Signature.Results()
	for i := 0; i < res.Len(); i++ {
		c.out.Results = append(c.out.Results, ir.TypeID(typeString(res.At(i).Type())))
	}
	c.out.Roots = []ir.BlockID{0}
	if fn.Recover != nil {
		c.out.Roots = append(c.out.Roots, ir.BlockID(fn.Recover.Index))
	}

	c.matchCalls(liftedFns)
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
			blk.Instrs = append(make([]ir.Instr, 0, len(entry)+len(b.Instrs)), entry...)
		} else {
			// go/ssa's count is close to what the block becomes (a φ makes no
			// instruction here, and one instruction can make several), so the
			// block is not grown by doubling, which leaves about a quarter of it empty
			blk.Instrs = make([]ir.Instr, 0, len(b.Instrs))
		}
		c.cur = &blk.Instrs
		for _, in := range b.Instrs {
			c.instr(in, blk, b)
		}
	}
	c.lowerPhis()
	c.fitBlocks()
	return c.out
}

// fitBlocks gives each block the size of its instructions and no more: what
// append left over (the count of go/ssa is only close) is held for as long as
// the IR is, which is most of the memory of the extraction.
func (c *conv) fitBlocks() {
	for i := range c.out.Blocks {
		b := &c.out.Blocks[i]
		if len(b.Instrs) == 0 {
			b.Instrs = nil
		} else if cap(b.Instrs) != len(b.Instrs) {
			b.Instrs = append(make([]ir.Instr, 0, len(b.Instrs)), b.Instrs...)
		}
	}
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
		c.emit(ir.Instr{Op: ir.OpCall, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "rt.rundefers"}, Pos: pos})
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
		c.emit(ir.Instr{Op: ir.OpIndexLoad, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X), c.val(in.Index)}, CharRead: isStringType(in.X.Type()), Pos: pos})
	case *ssa.Lookup:
		dsts := c.dst(in)
		if in.CommaOk {
			dsts = c.tupleDsts(in, 2)
		}
		c.emit(ir.Instr{Op: ir.OpIndexLoad, Dsts: dsts, Args: []ir.LocalID{c.val(in.X), c.val(in.Index)}, Keyed: true, CharRead: isStringType(in.X.Type()), Pos: pos})
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
		// go/ssa gives a MakeClosure no position: the function literal's stands for it
		if pos.File == "" {
			pos = c.pos(fn.Pos())
		}
		c.emit(ir.Instr{Op: ir.OpMakeClosure, Dsts: c.dst(in), Args: c.vals(in.Bindings), Func: ir.FuncID(gossa.FuncID(fn)), Pos: pos})
	case *ssa.Send:
		c.emit(ir.Instr{Op: ir.OpCall, Args: []ir.LocalID{c.val(in.Chan), c.val(in.X)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "chan.send"}, Pos: pos})
	case *ssa.Range:
		c.emit(ir.Instr{Op: ir.OpCall, Dsts: c.dst(in), Args: []ir.LocalID{c.val(in.X)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "iter.start"}, Pos: pos})
	case *ssa.Next:
		sym := "iter.next"
		if in.IsString {
			sym = "iter.next_string"
		}
		c.emit(ir.Instr{Op: ir.OpCall, Dsts: c.tupleDsts(in, 3), Args: []ir.LocalID{c.val(in.Iter)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: sym}, CharRead: in.IsString, Pos: pos})
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
				c.emit(ir.Instr{Op: ir.OpCall, Args: []ir.LocalID{c.val(st.Chan), c.val(st.Send)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "chan.send"}, Pos: pos})
				continue
			}
			ok := c.newLocal("", types.Typ[types.Bool])
			c.emit(ir.Instr{Op: ir.OpCall, Dsts: []ir.LocalID{elems[2+k], ok}, Args: []ir.LocalID{c.val(st.Chan)}, Callee: &ir.Callee{Kind: ir.CalleeStatic, Symbol: "chan.recv"}, Pos: pos})
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
// with the same calls of the same function in the lifted SSA, which the
// call graph is about. Lifting does not move or drop them, but it does drop
// bookkeeping calls, so they are paired by where they are and what they are
// (in order, when several are at the same place).
func (c *conv) matchCalls(liftedFns [][]liftedCall) {
	if len(liftedFns) == 0 {
		return
	}
	pools := make([]map[callKey][][]ir.FuncID, len(liftedFns))
	for i, calls := range liftedFns {
		pool := map[callKey][][]ir.FuncID{}
		for _, lc := range calls {
			pool[lc.key] = append(pool[lc.key], lc.cands)
		}
		pools[i] = pool
	}
	c.liftedCall = map[ssa.CallInstruction][][]ir.FuncID{}
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
			for _, pool := range pools {
				if list := pool[k]; len(list) > 0 {
					c.liftedCall[ci] = append(c.liftedCall[ci], list[0])
					pool[k] = list[1:]
				}
			}
		}
	}
}

// isEntry reports whether the function is where a Go program starts: main of
// a main package, or the initialisation of a package.
func isEntry(fn *ssa.Function) bool {
	if fn.Signature.Recv() != nil || fn.Parent() != nil {
		return false
	}
	name := fn.Name()
	if name == "init" || strings.HasPrefix(name, "init#") {
		return true
	}
	return name == "main" && fn.Pkg != nil && fn.Pkg.Pkg.Name() == "main"
}

// typeAliases lists the type aliases the packages declare, so that a rule that
// names an alias and one that names its type both match (ADR-083).
func typeAliases(p *gossa.Program) []ir.TypeAlias {
	if p.SSA == nil {
		return nil
	}
	var out []ir.TypeAlias
	for _, pkg := range p.SSA.AllPackages() {
		if pkg.Pkg == nil {
			continue
		}
		sc := pkg.Pkg.Scope()
		for _, n := range sc.Names() {
			tn, ok := sc.Lookup(n).(*types.TypeName)
			if !ok || !tn.IsAlias() {
				continue
			}
			a, ok := tn.Type().(*types.Alias)
			if !ok || a.TypeArgs().Len() > 0 || a.TypeParams().Len() > 0 {
				continue
			}
			name, target := typeString(a), typeString(types.Unalias(a))
			if name != target {
				out = append(out, ir.TypeAlias{Name: ir.TypeID(name), Target: ir.TypeID(target)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// isStringType reports whether t is a string type: s[i] of it reads one byte of a text (a map is not).
// isByteOrRune reports whether the type is a byte or a rune (uint8 or int32): the element of a slice or an array of them is
// one element of a text (the bytes or the runes of a string, once converted), and one of the binary data as well (the
// extractor does not tell: what the core does with it is for -char-taint).
func isByteOrRune(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && (b.Kind() == types.Uint8 || b.Kind() == types.Int32)
}

func isStringType(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}
