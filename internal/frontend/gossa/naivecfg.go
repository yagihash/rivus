package gossa

import (
	"go/token"

	"golang.org/x/tools/go/ssa"

	"github.com/yagihash/rivus/internal/ssabuild"
)

// NaiveCFG is a function built without lifting, as the graph of variables
// that ssabuild works on.
type NaiveCFG struct {
	Func *ssabuild.Func
	// Vars are the variables: the Allocs that go/ssa would lift.
	Vars []*ssa.Alloc
	// Instrs maps an instruction of Func back to the go/ssa instruction.
	Instrs map[ssabuild.UseKey]ssa.Instruction
}

// Liftable reports whether an Alloc is a variable that can be kept in a
// register: it is only loaded from and stored to. This is the criterion of
// go/ssa's lifting (liftAlloc), which the check against it needs to share.
func Liftable(alloc *ssa.Alloc) bool {
	if fn := alloc.Parent(); fn.Recover != nil {
		for _, r := range resultAllocs(fn) {
			if r == alloc {
				return false
			}
		}
	}
	for _, r := range *alloc.Referrers() {
		switch r := r.(type) {
		case *ssa.Store:
			if r.Val == ssa.Value(alloc) {
				return false // the address is used as a value
			}
		case *ssa.UnOp:
			if r.Op != token.MUL {
				return false
			}
		case *ssa.DebugRef:
		default:
			return false
		}
	}
	return true
}

// resultAllocs lists the Allocs that hold the named results of the function.
func resultAllocs(fn *ssa.Function) []*ssa.Alloc {
	var out []*ssa.Alloc
	// go/ssa keeps them in fn.results (unexported). A named result is an
	// Alloc whose name is the name of a result variable.
	res := fn.Signature.Results()
	for i := 0; i < res.Len(); i++ {
		name := res.At(i).Name()
		if name == "" || name == "_" {
			continue
		}
		for _, l := range fn.Locals {
			if l.Comment == name {
				out = append(out, l)
			}
		}
	}
	return out
}

// NaiveCFGOf converts a function built with NaiveForm. The block numbers are
// those of go/ssa.
func NaiveCFGOf(fn *ssa.Function) *NaiveCFG {
	c := &NaiveCFG{Instrs: map[ssabuild.UseKey]ssa.Instruction{}}
	varOf := map[*ssa.Alloc]int{}
	f := &ssabuild.Func{Blocks: make([]ssabuild.Block, len(fn.Blocks)), Roots: []int{0}}
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			if a, ok := in.(*ssa.Alloc); ok && Liftable(a) {
				varOf[a] = len(c.Vars)
				c.Vars = append(c.Vars, a)
			}
		}
	}
	f.NVars = len(c.Vars)
	if fn.Recover != nil {
		f.Roots = append(f.Roots, fn.Recover.Index)
	}
	for _, b := range fn.Blocks {
		blk := &f.Blocks[b.Index]
		for _, s := range b.Succs {
			blk.Succs = append(blk.Succs, s.Index)
		}
		for _, in := range b.Instrs {
			var ins ssabuild.Instr
			switch in := in.(type) {
			case *ssa.Alloc:
				v, ok := varOf[in]
				if !ok {
					continue
				}
				ins = ssabuild.Instr{Op: ssabuild.Zero, Var: v}
			case *ssa.Store:
				a, ok := in.Addr.(*ssa.Alloc)
				if !ok {
					continue
				}
				v, ok := varOf[a]
				if !ok {
					continue
				}
				ins = ssabuild.Instr{Op: ssabuild.Def, Var: v}
			case *ssa.UnOp:
				a, ok := in.X.(*ssa.Alloc)
				if !ok || in.Op != token.MUL {
					continue
				}
				v, ok := varOf[a]
				if !ok {
					continue
				}
				ins = ssabuild.Instr{Op: ssabuild.Use, Var: v}
			default:
				continue
			}
			c.Instrs[ssabuild.UseKey{Block: b.Index, Instr: len(blk.Instrs)}] = in
			blk.Instrs = append(blk.Instrs, ins)
		}
	}
	c.Func = f
	return c
}
