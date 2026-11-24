package valuecalls

import (
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irssa"
)

// A variable of a function that a closure takes and that is written after it is declared (a local function held in a
// variable that gets another one later, a `late` variable that is assigned after the closure that calls it was made) is
// held in a cell: the function that made the closure allocates the cell, and the closure gets the address of it and
// loads it where it uses the variable.
//
// cellValues gives the values that the cell may hold when the closure that was made at (mb, mi) reads it: the stores
// that reach the make_closure without another store to the cell between (a variable that was assigned a function and
// then another one holds only the second), and the stores that run after it (the closure may be called after them).
// Every store of the function would be too many: `var f = first; f(a); f = second; use(() => f(b))` calls only `second`
// in the closure.
//
// With after false (a load in the function that holds the cell), only the stores that reach the load count.
//
// ok is false when the cell may be written from somewhere that this walk does not see (the address goes to a call, is
// stored as a value, or is given to a closure that writes through it), or when the address is not an allocation of the
// function that made the closure (the closure that made this one got it from a third function): what is returned is
// then not all that the cell may hold, and the caller says that the callee is not known to be only these.
func (r *resolver) cellValues(mfn *ir.Func, mform *irssa.Form, start irssa.Ref, mb, mi int, after bool) (values []irssa.Ref, ok bool) {
	if !isAlloc(mfn, mform, start) {
		return nil, false
	}
	ok = true
	aliases := map[irssa.Ref]bool{start: true}
	type at struct{ b, i int }
	var stores []at
	for changed := true; changed; {
		changed = false
		for bi := range mfn.Blocks {
			for ii := range mfn.Blocks[bi].Instrs {
				in := &mfn.Blocks[bi].Instrs[ii]
				if (in.Op == ir.OpCopy || in.Op == ir.OpConvert) && len(in.Args) == 1 && !in.View && aliases[mform.Operand(bi, ii, 0)] {
					d := irssa.Ref{Kind: irssa.RefInstr, Block: bi, Index: ii}
					if !aliases[d] {
						aliases[d] = true
						changed = true
					}
				}
			}
		}
	}
	for bi := range mfn.Blocks {
		b := &mfn.Blocks[bi]
		for ii := range b.Instrs {
			in := &b.Instrs[ii]
			if mform.IsPromotedAccess(*in) {
				continue
			}
			for k := range in.Args {
				if !aliases[mform.Operand(bi, ii, k)] {
					continue
				}
				switch in.Op {
				case ir.OpLoad, ir.OpCopy, ir.OpConvert:
				case ir.OpStore:
					if k != 0 {
						ok = false
						continue
					}
					stores = append(stores, at{bi, ii})
				case ir.OpMakeClosure:
					if !r.closureOnlyReads(in, k) {
						ok = false
					}
				default:
					ok = false
				}
			}
		}
		for k := range b.Term.Values {
			if aliases[mform.Operand(bi, len(b.Instrs), k)] {
				ok = false
			}
		}
	}
	if len(stores) == 0 {
		return nil, ok
	}
	isStore := map[at]bool{}
	for _, s := range stores {
		isStore[s] = true
	}
	preds := make([][]int, len(mfn.Blocks))
	for bi := range mfn.Blocks {
		for _, e := range mfn.Blocks[bi].Succs {
			if int(e.To) >= 0 && int(e.To) < len(mfn.Blocks) {
				preds[e.To] = append(preds[e.To], bi)
			}
		}
	}
	chosen := map[at]bool{}
	// the stores that reach the make_closure: the nearest one on each path back
	seen := map[int]bool{}
	var back func(b, from int)
	back = func(b, from int) {
		for i := from; i >= 0; i-- {
			if isStore[at{b, i}] {
				chosen[at{b, i}] = true
				return
			}
		}
		if seen[b] {
			return
		}
		seen[b] = true
		for _, p := range preds[b] {
			back(p, len(mfn.Blocks[p].Instrs)-1)
		}
	}
	back(mb, mi-1)
	if !after {
		for _, s := range stores {
			if chosen[s] {
				values = append(values, mform.Operand(s.b, s.i, 1))
			}
		}
		return values, ok
	}
	// the stores that run after it
	for i := mi + 1; i < len(mfn.Blocks[mb].Instrs); i++ {
		if isStore[at{mb, i}] {
			chosen[at{mb, i}] = true
		}
	}
	reach := map[int]bool{}
	var fwd func(b int)
	fwd = func(b int) {
		for _, e := range mfn.Blocks[b].Succs {
			t := int(e.To)
			if t < 0 || t >= len(mfn.Blocks) || reach[t] {
				continue
			}
			reach[t] = true
			for i := range mfn.Blocks[t].Instrs {
				if isStore[at{t, i}] {
					chosen[at{t, i}] = true
				}
			}
			fwd(t)
		}
	}
	fwd(mb)
	for _, s := range stores {
		if chosen[s] {
			values = append(values, mform.Operand(s.b, s.i, 1))
		}
	}
	return values, ok
}

// isAlloc says whether the reference is, through copies, an allocation.
func isAlloc(f *ir.Func, form *irssa.Form, ref irssa.Ref) bool {
	for n := 0; n < 8; n++ {
		if ref.Kind != irssa.RefInstr {
			return false
		}
		in := &f.Blocks[ref.Block].Instrs[ref.Index]
		switch {
		case in.Op == ir.OpAlloc:
			return true
		case in.Op == ir.OpCopy && !in.View && len(in.Args) == 1:
			ref = form.Operand(ref.Block, ref.Index, 0)
		default:
			return false
		}
	}
	return false
}

// closureOnlyReads says whether the closure made by mk reads the cell it got as the capture k, and does nothing else
// with it (no store, no escape).
func (r *resolver) closureOnlyReads(mk *ir.Instr, k int) bool {
	g, _ := ir.Get(r.src, mk.Func)
	if g == nil {
		return false
	}
	gform := r.form(g)
	if gform == nil {
		return false
	}
	idx := declared(g) + k
	for bi := range g.Blocks {
		for ii := range g.Blocks[bi].Instrs {
			in := &g.Blocks[bi].Instrs[ii]
			if in.Op == ir.OpParam && in.Index == idx {
				_, _, ok := (*narrower)(nil).aliasUses(g, gform, irssa.Ref{Kind: irssa.RefInstr, Block: bi, Index: ii}, false)
				return ok
			}
		}
	}
	return false
}
