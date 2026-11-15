package sg

import (
	"fmt"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irssa"
)

// Build makes the SSA graph of every function of the program.
func Build(p *ir.Program) (*Program, error) {
	out := &Program{IR: p, ByID: map[string]*Func{}, Globals: map[string]*Value{}, GlobalPos: map[string]ir.Pos{}}
	for _, g := range p.Globals {
		out.GlobalPos[g.Name] = g.Pos
	}
	for _, f := range p.Funcs {
		fn, err := buildFunc(out, f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.ID, err)
		}
		out.Funcs = append(out.Funcs, fn)
		out.ByID[fn.ID] = fn
	}
	out.link()
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
			ID: string(f.ID), IR: f, Prog: p, Symbol: f.Symbol, Recv: f.Recv, Dep: f.Dep,
			Exported: f.Exported, Pos: f.Pos, Form: form,
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
	return b.fn, nil
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
			in := &Instr{Op: irin.Op, IR: irin, IRBlock: bi, IRIndex: ii, Pos: irin.Pos, AddrPos: irin.AddrPos, Callee: irin.Callee}
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
