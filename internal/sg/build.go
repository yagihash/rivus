package sg

import (
	"fmt"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irssa"
	"github.com/yagihash/rivus/internal/valuecalls"
)

// Build makes the SSA graph of every function of the program.
func Build(src ir.Source) (*Program, error) { return build(src, false) }

// BuildConsuming is Build that lets the source go of the functions as it
// goes (the source is released afterwards): the graph keeps what it needs, and
// the program is not in memory twice.
func BuildConsuming(src ir.Source) (*Program, error) { return build(src, true) }

func build(src ir.Source, consume bool) (*Program, error) {
	valuecalls.Resolve(src)
	p := src.Header()
	out := &Program{IR: p, ByID: map[string]*Func{}, Globals: map[string]*Value{}, GlobalPos: map[string]ir.Pos{}}
	for _, g := range p.Globals {
		out.GlobalPos[g.Name] = g.Pos
	}
	for i, n := 0, src.Len(); i < n; i++ {
		h := src.Head(i)
		if h.Gone {
			continue
		}
		ref := ir.Ref{ID: h.ID, Seq: i}
		f, err := src.Func(ref)
		if err != nil {
			return nil, &ir.ReadError{Ref: ref, Err: err}
		}
		for bi := range f.Blocks {
			for _, e := range f.Blocks[bi].Succs {
				if e.Kind == ir.EdgeException {
					out.HasExceptions = true
				}
			}
		}
		fn, err := buildFunc(out, f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.ID, err)
		}
		out.Funcs = append(out.Funcs, fn)
		out.ByID[fn.ID] = fn
		if consume {
			src.Drop(ref)
		}
	}
	if consume {
		src.Release()
	}
	out.infos = nil
	out.link()
	out.linkEnvClosures()
	out.resolveValueCalls()
	return out, nil
}

// link resolves the functions that calls and function values name.
func (p *Program) link() {
	for _, fn := range p.Funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				for _, v := range in.Args {
					if v.Kind == KFunc && v.Fn == nil {
						v.Fn = p.ByID[string(v.FuncID)]
					}
				}
				if in.Callee == nil {
					continue
				}
				switch in.Callee.Kind {
				case ir.CalleeStatic:
					in.StaticID = in.Callee.Func
					in.Static = p.ByID[string(in.Callee.Func)]
				case ir.CalleeValue:
					// A closure made right there is a static call with the
					// captured values (as go/ssa has it). It may have been
					// copied into a variable on the way.
					fn := (*Value)(nil)
					if len(in.Args) > 0 {
						fn = in.Args[0]
						for i := 0; i < 8 && fn.Kind == KResult && fn.Instr != nil && fn.Instr.Op == ir.OpCopy && !fn.Instr.IR.View && len(fn.Instr.Args) == 1; i++ {
							fn = fn.Instr.Args[0]
						}
					}
					if fn != nil && fn.Kind == KFunc {
						// A function with nothing captured, held in a variable.
						in.StaticID = fn.FuncID
						in.Static = p.ByID[string(fn.FuncID)]
					} else if fn != nil && fn.Kind == KResult && fn.Instr != nil &&
						fn.Instr.Op == ir.OpMakeClosure && fn.Instr.IR != nil {
						mc := fn.Instr
						in.StaticID = mc.IR.Func
						in.Static = p.ByID[string(mc.IR.Func)]
						in.Bindings = mc.Args
					}
				}
				in.Candidates = in.Callee.Candidates
				for _, id := range in.Callee.Candidates {
					if f := p.ByID[string(id)]; f != nil {
						in.CandidateFns = append(in.CandidateFns, f)
					}
				}
			}
		}
	}
}

type valueKey struct{ block, instr, dst int }

type builder struct {
	prog *Program
	f    *ir.Func
	fn   *Func
	form *irssa.Form

	values map[valueKey]*Value
	alias  map[valueKey]irssa.Ref
	phis   map[[2]int]*Value // block, var
	zeros  map[ir.LocalID]*Value
	undef  *Value
}

func buildFunc(p *Program, f *ir.Func) (*Func, error) {
	form, err := irssa.Build(f)
	if err != nil {
		return nil, err
	}
	b := &builder{
		prog: p, f: f, form: form,
		fn: &Func{
			ID: string(f.ID), IR: slimFunc(f), Prog: p, Symbol: f.Symbol, Recv: f.Recv, Dep: f.Dep,
			Exported: f.Exported, Entry: f.Entry, Init: f.Init, Pos: f.Pos,
		},
		values: map[valueKey]*Value{},
		alias:  map[valueKey]irssa.Ref{},
		phis:   map[[2]int]*Value{},
		zeros:  map[ir.LocalID]*Value{},
	}
	b.fn.Blocks = make([]*Block, len(f.Blocks))
	for i := range f.Blocks {
		b.fn.Blocks[i] = &Block{Func: b.fn, Index: i}
	}
	for i, blk := range f.Blocks {
		sb := b.fn.Blocks[i]
		for _, e := range blk.Succs {
			sb.Succs = append(sb.Succs, b.fn.Blocks[e.To])
			sb.Edges = append(sb.Edges, e)
		}
		for _, pi := range form.Preds[i] {
			sb.Preds = append(sb.Preds, b.fn.Blocks[pi])
		}
		if id := form.Idom(i); id >= 0 {
			sb.Idom = b.fn.Blocks[id]
		}
	}
	b.defineValues()
	b.buildInstrs()
	markEscapes(b.fn)
	return b.fn, nil
}

// markEscapes sets Instr.Escapes of the make_closure instructions that capture values: whether
// the closure is used in a way other than a call right here (the call, a go statement, a defer:
// the callee position of a call of a function value). The decision is by the uses of the value
// in the function, not by what the call resolution found.
func markEscapes(fn *Func) {
	var mcs []*Instr
	for _, blk := range fn.Blocks {
		for _, in := range blk.Instrs {
			if in.Op == ir.OpMakeClosure && len(in.Args) > 0 && len(in.Dsts) == 1 {
				mcs = append(mcs, in)
			}
		}
	}
	if len(mcs) == 0 {
		return
	}
	uses := map[*Value][]*Instr{}
	for _, blk := range fn.Blocks {
		for _, in := range blk.Instrs {
			for _, a := range in.Args {
				if a != nil {
					uses[a] = append(uses[a], in)
				}
			}
		}
	}
	for _, mc := range mcs {
		mc.Escapes = escapes(mc.Dsts[0], uses)
	}
}

func escapes(v *Value, uses map[*Value][]*Instr) bool {
	seen := map[*Value]bool{}
	work := []*Value{v}
	for len(work) > 0 {
		x := work[len(work)-1]
		work = work[:len(work)-1]
		if seen[x] {
			continue
		}
		seen[x] = true
		for _, u := range uses[x] {
			switch {
			case u.Op == ir.OpCall && u.Callee != nil && u.Callee.Kind == ir.CalleeValue &&
				len(u.Args) > 0 && u.Args[0] == x && !argAgain(u.Args[1:], x):
				// called right here
			case u.Op == OpPhi, u.Op == ir.OpCopy && u.IR != nil && !u.IR.View && len(u.Args) == 1:
				work = append(work, u.Dsts...)
			default:
				return true
			}
		}
	}
	return false
}

func argAgain(args []*Value, x *Value) bool {
	for _, a := range args {
		if a == x {
			return true
		}
	}
	return false
}

func (b *builder) local(l ir.LocalID) (name string, t ir.TypeID, c ir.TypeClass) {
	loc := b.f.Locals[l]
	return loc.Name, loc.Type, loc.Class
}

// defineValues makes the Values of everything an instruction defines, before
// any operand is resolved (an operand may be defined later in the block
// order).
func (b *builder) defineValues() {
	declared := 0
	for _, p := range b.f.Params {
		if !p.Captured {
			declared++
		}
	}
	b.fn.Params = make([]*Value, declared)
	b.fn.FreeVars = make([]*Value, len(b.f.Params)-declared)
	for bi, blk := range b.f.Blocks {
		for ii, in := range blk.Instrs {
			if b.form.IsPromotedAccess(in) {
				continue
			}
			for k, d := range in.Dsts {
				name, typ, cls := b.local(d)
				v := &Value{Name: name, Type: typ, Class: cls, Func: b.fn, Pos: in.Pos, Index: k}
				key := valueKey{bi, ii, k}
				switch {
				case in.Op == ir.OpConst:
					v.Kind, v.Const = KConst, in.Const
				case in.Op == ir.OpParam:
					v.Param = in.Index
					if in.Index < declared {
						v.Kind = KParam
						b.fn.Params[in.Index] = v
					} else {
						v.Kind = KFreeVar
						b.fn.FreeVars[in.Index-declared] = v
					}
				case in.Op == ir.OpMakeClosure && len(in.Args) == 0:
					v.Kind, v.FuncID, v.Func = KFunc, in.Func, nil
				case in.Op == ir.OpAddrOf && in.Global != "":
					b.values[key] = b.prog.Global(in.Global)
					continue
				case in.Op == ir.OpCopy && !in.View && b.form.Variables[d]:
					// The copy that lowers a φ: the φ takes the copied value.
					b.alias[key] = b.form.Operand(bi, ii, 0)
					continue
				default:
					v.Kind = KResult
				}
				b.values[key] = v
			}
		}
	}
	// The φ functions.
	for bi, phis := range b.form.Phis {
		for _, ph := range phis {
			v := &Value{Kind: KPhi, Func: b.fn}
			if b.f.Locals[ph.Var].Name != "" {
				v.Name = b.f.Locals[ph.Var].Name
			}
			if b.form.Promoted[ph.Var] {
				v.Type, v.Class = b.allocElem(ph.Var)
			} else {
				_, v.Type, v.Class = b.local(ph.Var)
			}
			b.phis[[2]int{bi, int(ph.Var)}] = v
		}
	}
}

// allocElem is the type of what a promoted variable holds.
func (b *builder) allocElem(l ir.LocalID) (ir.TypeID, ir.TypeClass) {
	d := b.form.AllocInstr(l)
	return d.Type, d.Class
}

func (b *builder) valueOf(r irssa.Ref) *Value {
	switch r.Kind {
	case irssa.RefInstr:
		key := valueKey{r.Block, r.Index, r.Dst}
		if a, ok := b.alias[key]; ok {
			return b.valueOf(a)
		}
		if v, ok := b.values[key]; ok {
			return v
		}
	case irssa.RefPhi:
		if v, ok := b.phis[[2]int{r.Block, int(r.Var)}]; ok {
			return v
		}
	case irssa.RefZero:
		if v, ok := b.zeros[r.Var]; ok {
			return v
		}
		typ, cls := b.allocElem(r.Var)
		v := &Value{Kind: KConst, Type: typ, Class: cls, Func: b.fn}
		b.zeros[r.Var] = v
		return v
	}
	if b.undef == nil {
		b.undef = &Value{Kind: KUndef, Func: b.fn}
	}
	return b.undef
}

func (b *builder) buildInstrs() {
	for bi, blk := range b.f.Blocks {
		sb := b.fn.Blocks[bi]
		add := func(in *Instr) {
			in.Block = sb
			in.Index = len(sb.Instrs)
			sb.Instrs = append(sb.Instrs, in)
			for _, d := range in.Dsts {
				if d.Instr == nil && (d.Kind == KResult || d.Kind == KPhi) {
					d.Instr = in
				}
			}
		}
		// φ functions first, in a stable order.
		for _, ph := range b.form.Phis[bi] {
			v := b.phis[[2]int{bi, int(ph.Var)}]
			in := &Instr{Op: OpPhi, Dsts: []*Value{v}, IRBlock: bi, IRIndex: -1}
			v.Instr = in
			for _, a := range ph.Args {
				in.Args = append(in.Args, b.valueOf(a))
			}
			add(in)
		}
		for ii := range blk.Instrs {
			irin := &blk.Instrs[ii]
			if b.form.IsPromotedAccess(*irin) {
				continue
			}
			switch irin.Op {
			case ir.OpConst, ir.OpParam:
				continue
			case ir.OpMakeClosure:
				if len(irin.Args) == 0 {
					continue
				}
			case ir.OpAddrOf:
				if irin.Global != "" {
					continue
				}
			case ir.OpCopy:
				if !irin.View && len(irin.Dsts) == 1 && b.form.Variables[irin.Dsts[0]] {
					continue
				}
			}
			in := &Instr{Op: irin.Op, IR: b.prog.infoOf(irin), IRBlock: bi, IRIndex: ii, Pos: irin.Pos, AddrPos: irin.AddrPos, Callee: irin.Callee}
			for k := range irin.Dsts {
				in.Dsts = append(in.Dsts, b.values[valueKey{bi, ii, k}])
			}
			for k := range irin.Args {
				in.Args = append(in.Args, b.valueOf(b.form.Operand(bi, ii, k)))
			}
			add(in)
		}
		t := blk.Term
		term := &Instr{IRBlock: bi, IRIndex: len(blk.Instrs), Pos: t.Pos}
		switch t.Kind {
		case ir.TermJump:
			term.Op = OpJump
		case ir.TermBranch:
			term.Op = OpBranch
			term.Pos = t.CondPos
		case ir.TermReturn:
			term.Op = OpReturn
		case ir.TermThrow:
			term.Op = OpThrow
		default:
			term.Op = OpUnreachable
		}
		n := 0
		switch t.Kind {
		case ir.TermBranch:
			n = 1
		case ir.TermReturn, ir.TermThrow:
			n = len(t.Values)
		}
		for k := 0; k < n; k++ {
			term.Args = append(term.Args, b.valueOf(b.form.Operand(bi, len(blk.Instrs), k)))
		}
		add(term)
	}
}

// resolveValueCalls takes, for the calls of function values that nothing else
// resolved, the functions that internal/valuecalls found: those that flow to the
// value through copies, φ functions and the parameters of the function (the
// arguments its callers pass). The functions that have no body are candidates
// too (a rule may name them), but there is no summary to apply for them.
func (p *Program) resolveValueCalls() {
	for _, fn := range p.Funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpCall || in.Callee.Kind != ir.CalleeValue || in.Static != nil || len(in.Callee.Candidates) > 0 || len(in.Callee.ValueCands) == 0 {
					continue
				}
				for _, c := range in.Callee.ValueCands {
					in.Candidates = append(in.Candidates, c.ID)
					f := p.ByID[string(c.ID)]
					if f == nil {
						continue
					}
					in.CandidateFns = append(in.CandidateFns, f)
					if c.Bind != nil {
						if mc := fn.instrAt(c.Bind.Block, c.Bind.Instr); mc != nil && len(mc.Args) > 0 {
							if in.CandidateBindings == nil {
								in.CandidateBindings = map[*Func][]*Value{}
							}
							in.CandidateBindings[f] = mc.Args
						}
					}
				}
			}
		}
	}
}

// instrAt is the instruction that came from the instruction of the IR.
func (f *Func) instrAt(block, instr int) *Instr {
	if block < 0 || block >= len(f.Blocks) {
		return nil
	}
	for _, in := range f.Blocks[block].Instrs {
		if in.IRIndex == instr {
			return in
		}
	}
	return nil
}

// ExactFuncs are the functions a value is, when the function it is in
// decides it: a function, or a closure made there, perhaps copied or chosen by
// a φ. ok is false if it may be anything else too (a parameter, a result of a
// call, something read from memory): the callers cannot tell.
func ExactFuncs(v *Value) (ids map[string]bool, ok bool) {
	cs, ok := ExactClosures(v)
	if !ok {
		return nil, false
	}
	ids = map[string]bool{}
	for _, c := range cs {
		if c.Env != nil {
			return nil, false // a closure of Rust: not what ExactFuncs has told (its callers do not bind an environment)
		}
		ids[c.ID] = true
	}
	return ids, true
}

// Closure is a function a value is: Make is the instruction that made it, which has the values it
// captured (its Args: the bindings), or nil for a function that is not a closure. A closure of Rust
// is the alloc of its environment: Env is that value, which the closure takes as its first parameter,
// and the values it captured are its fields.
type Closure struct {
	ID   string
	Make *Instr
	Env  *Value
}

// ExactClosures is ExactFuncs with the closures' make instructions, so that a caller can bind
// the values they captured. The order is the order they were found in, without repeats.
func ExactClosures(v *Value) (out []Closure, ok bool) {
	seen := map[*Value]bool{}
	have := map[Closure]bool{}
	add := func(c Closure) {
		if !have[c] {
			have[c] = true
			out = append(out, c)
		}
	}
	var walk func(v *Value, depth int) bool
	walk = func(v *Value, depth int) bool {
		if v == nil || depth > 8 {
			return false
		}
		if seen[v] {
			return true
		}
		seen[v] = true
		switch v.Kind {
		case KFunc:
			add(Closure{ID: string(v.FuncID)})
			return true
		case KPhi:
			if v.Instr == nil {
				return false
			}
			for _, a := range v.Instr.Args {
				if !walk(a, depth+1) {
					return false
				}
			}
			return true
		case KResult:
			in := v.Instr
			if in == nil || in.IR == nil {
				return false
			}
			switch in.Op {
			case ir.OpMakeClosure:
				add(Closure{ID: string(in.IR.Func), Make: in})
				return true
			case ir.OpAlloc:
				if v.Func != nil && v.Func.Prog != nil && len(in.Dsts) == 1 {
					if f := v.Func.Prog.EnvClosures[in]; f != nil {
						add(Closure{ID: f.ID, Env: in.Dsts[0]})
						return true
					}
				}
			case ir.OpCopy:
				if !in.IR.View && len(in.Args) == 1 {
					return walk(in.Args[0], depth+1)
				}
			case ir.OpConvert:
				// a function of another function type (valuecalls does the same)
				if len(in.Args) == 1 && len(in.Dsts) == 1 && in.Dsts[0].Class == ir.ClassFunc && in.Args[0].Class == ir.ClassFunc {
					return walk(in.Args[0], depth+1)
				}
			}
		}
		return false
	}
	if !walk(v, 0) {
		return nil, false
	}
	return out, true
}

// slimFunc is the function without its blocks and locals.
func slimFunc(f *ir.Func) *ir.Func {
	c := *f
	c.Blocks = nil
	c.Locals = nil
	return &c
}

// infoKey is what InstrInfo says, as a key that can be compared (the path as a string).
type infoKey struct {
	operator, field, global, path, env, reason string
	fn                                         ir.FuncID
	view, folded, keyed, charRead, goStmt      bool
}

// infoOf is the InstrInfo of an instruction. Most instructions say the same (a call says nothing, a field
// load says its field), so the equal ones are one value: the analysis only reads it (nothing writes a field
// of Instr.IR or an element of its Path), and the live heap of a large program is smaller by the copies.
// The table is the one of a build; it is dropped when the build ends.
func (p *Program) infoOf(in *ir.Instr) *InstrInfo {
	k := infoKey{operator: in.Operator, field: in.Field, global: in.Global, reason: in.Reason, fn: in.Func,
		view: in.View, folded: in.Folded, keyed: in.Keyed, charRead: in.CharRead, goStmt: in.Go}
	if in.Op == ir.OpAlloc && strings.HasPrefix(string(in.Type), "{closure@") {
		k.env = string(in.Type)
	}
	if len(in.Path) > 0 {
		var sb strings.Builder
		for _, st := range in.Path {
			sb.WriteString(string(st.Kind))
			sb.WriteByte(0)
			sb.WriteString(st.Field)
			sb.WriteByte(1)
		}
		k.path = sb.String()
	}
	if info, ok := p.infos[k]; ok {
		return info
	}
	info := &InstrInfo{Operator: in.Operator, Field: in.Field, Global: in.Global, Path: in.Path, Func: in.Func,
		View: in.View, Folded: in.Folded, Keyed: in.Keyed, CharRead: in.CharRead, Go: in.Go, Reason: in.Reason, Env: k.env}
	if p.infos == nil {
		p.infos = map[infoKey]*InstrInfo{}
	}
	p.infos[k] = info
	return info
}
