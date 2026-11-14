// Package irssa gives a function of the neutral IR its SSA form.
//
// The IR has mutable locals and no φ functions (ADR-003). Two things are
// variables here: the locals that more than one instruction writes (the
// extractor lowers the φ of a source language to such copies), and the
// allocations that are only loaded from and stored to, which are promoted
// from memory to registers (go/ssa calls it lifting). internal/ssabuild
// places the φ functions and finds the definition that reaches each use; this
// package connects it to the IR and answers the question the analyses ask:
// what is this operand?
//
// A Ref names a definition: an instruction (and which of its results), a φ,
// the zero value of a promoted allocation, or nothing. Refs are comparable and
// stable, so they serve as the identity of an SSA value.
package irssa

import (
	"fmt"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/ssabuild"
)

// RefKind says what a Ref names.
type RefKind uint8

const (
	// RefUndef: no definition reaches (or the local is never written).
	RefUndef RefKind = iota
	// RefInstr: result Dst of instruction Index in Block.
	RefInstr
	// RefPhi: the φ of the variable Var at the start of Block.
	RefPhi
	// RefZero: the zero value of the promoted allocation Var, declared by the
	// instruction Index in Block.
	RefZero
)

// Ref names a definition.
type Ref struct {
	Kind  RefKind
	Block int
	Index int
	Dst   int
	Var   ir.LocalID
}

func (r Ref) String() string {
	switch r.Kind {
	case RefInstr:
		return fmt.Sprintf("b%d.%d#%d", r.Block, r.Index, r.Dst)
	case RefPhi:
		return fmt.Sprintf("phi(b%d, %%%d)", r.Block, r.Var)
	case RefZero:
		return fmt.Sprintf("zero(%%%d)", r.Var)
	}
	return "undef"
}

// Phi is a φ function: for a variable, at the start of a block, the
// definition that comes in from each predecessor (in the order of Preds).
type Phi struct {
	Var  ir.LocalID
	Args []Ref
}

// Form is the SSA form of a function.
type Form struct {
	Func  *ir.Func
	Preds [][]int
	// Phis lists, for each block, its φ functions.
	Phis [][]Phi
	// Promoted holds the allocations that became variables: their loads and
	// stores are no longer memory operations.
	Promoted map[ir.LocalID]bool
	// Variables are the locals that more than one instruction writes.
	Variables map[ir.LocalID]bool

	sb       *ssabuild.Func
	res      *ssabuild.Result
	varOf    map[ir.LocalID]int
	localOf  []ir.LocalID
	useIdx   map[opKey]int
	sites    [][]site // per block: the ssabuild instruction -> the IR instruction
	def      map[ir.LocalID]defSite
	cache    map[opKey]Ref
	allocPos map[ir.LocalID]defSite
}

type opKey struct{ block, instr, arg int }

type siteKind uint8

const (
	siteUse siteKind = iota
	siteDef
	siteZero
	siteStore
)

type site struct {
	kind  siteKind
	instr int
	dst   int
}

type defSite struct{ block, instr, dst int }

// Build computes the SSA form.
func Build(f *ir.Func) (*Form, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	s := &Form{
		Func:      f,
		Promoted:  map[ir.LocalID]bool{},
		Variables: map[ir.LocalID]bool{},
		varOf:     map[ir.LocalID]int{},
		useIdx:    map[opKey]int{},
		def:       map[ir.LocalID]defSite{},
		cache:     map[opKey]Ref{},
		allocPos:  map[ir.LocalID]defSite{},
	}
	s.classify()
	s.graph()
	s.res = ssabuild.Build(s.sb)
	s.Preds = s.res.Preds
	s.phis()
	return s, nil
}

// SSABuild returns the graph that was given to ssabuild, and its result, for
// the tests that compare them with other analyses.
func (s *Form) SSABuild() (*ssabuild.Func, *ssabuild.Result) { return s.sb, s.res }

// VarLocal is the local that an ssabuild variable stands for.
func (s *Form) VarLocal(v int) ir.LocalID { return s.localOf[v] }

// operands lists the locals an instruction reads, in order.
func operands(in ir.Instr) []ir.LocalID { return in.Args }

func termOperands(t ir.Term) []ir.LocalID {
	switch t.Kind {
	case ir.TermBranch:
		return []ir.LocalID{t.Cond}
	case ir.TermReturn, ir.TermThrow:
		return t.Values
	}
	return nil
}

// classify finds the variables: promoted allocations and multiply written
// locals, and the single definition of every other local.
func (s *Form) classify() {
	f := s.Func
	writes := map[ir.LocalID]int{}
	for bi, b := range f.Blocks {
		for ii, in := range b.Instrs {
			for k, d := range in.Dsts {
				writes[d]++
				s.def[d] = defSite{bi, ii, k}
			}
			if in.Op == ir.OpAlloc {
				s.allocPos[in.Dsts[0]] = defSite{bi, ii, 0}
			}
		}
	}
	// Allocations: a candidate is used only as the address of a load or a
	// store, and not stored itself.
	cand := map[ir.LocalID]bool{}
	for d := range s.allocPos {
		cand[d] = writes[d] == 1
	}
	reachable := s.reachableFromEntry()
	for bi, b := range f.Blocks {
		for _, in := range b.Instrs {
			for k, a := range in.Args {
				if !cand[a] {
					continue
				}
				switch {
				case in.Op == ir.OpLoad && k == 0:
					// With a recover root, a variable read where only the
					// recover path leads is the named result that a
					// deferred call may have changed: it stays in memory
					// (go/ssa does the same).
					if !reachable[bi] {
						cand[a] = false
					}
				case in.Op == ir.OpStore && k == 0:
				default:
					cand[a] = false
				}
			}
		}
		for _, a := range termOperands(b.Term) {
			cand[a] = false
		}
	}
	for d, ok := range cand {
		if ok {
			s.Promoted[d] = true
		}
	}
	for d, n := range writes {
		if n > 1 {
			s.Variables[d] = true
		}
	}
}

func (s *Form) reachableFromEntry() map[int]bool {
	seen := map[int]bool{}
	var walk func(b int)
	walk = func(b int) {
		if seen[b] {
			return
		}
		seen[b] = true
		for _, e := range s.Func.Blocks[b].Succs {
			walk(int(e.To))
		}
	}
	walk(0)
	if len(s.Func.Roots) <= 1 {
		// No recover root: everything that has a predecessor is reached.
		for i := range s.Func.Blocks {
			seen[i] = true
		}
	}
	return seen
}

func (s *Form) variable(l ir.LocalID) int {
	if v, ok := s.varOf[l]; ok {
		return v
	}
	v := len(s.localOf)
	s.varOf[l] = v
	s.localOf = append(s.localOf, l)
	return v
}

// graph builds the graph of variables that ssabuild works on.
func (s *Form) graph() {
	f := s.Func
	// Number the variables in a fixed order: promoted allocations by
	// position first, then the multiply written locals by local number.
	for bi := range f.Blocks {
		for _, in := range f.Blocks[bi].Instrs {
			if in.Op == ir.OpAlloc && s.Promoted[in.Dsts[0]] {
				s.variable(in.Dsts[0])
			}
		}
	}
	for l := range f.Locals {
		if s.Variables[ir.LocalID(l)] {
			s.variable(ir.LocalID(l))
		}
	}
	sb := &ssabuild.Func{Blocks: make([]ssabuild.Block, len(f.Blocks)), NVars: len(s.localOf)}
	for _, r := range f.Roots {
		sb.Roots = append(sb.Roots, int(r))
	}
	if len(sb.Roots) == 0 {
		sb.Roots = []int{0}
	}
	s.sites = make([][]site, len(f.Blocks))
	for bi, b := range f.Blocks {
		blk := &sb.Blocks[bi]
		for _, e := range b.Succs {
			blk.Succs = append(blk.Succs, int(e.To))
		}
		add := func(op ssabuild.Op, v int, st site) {
			blk.Instrs = append(blk.Instrs, ssabuild.Instr{Op: op, Var: v})
			s.sites[bi] = append(s.sites[bi], st)
		}
		use := func(l ir.LocalID, key opKey) {
			if v, ok := s.varOf[l]; ok {
				s.useIdx[key] = len(blk.Instrs)
				add(ssabuild.Use, v, site{kind: siteUse, instr: key.instr})
			}
		}
		for ii, in := range b.Instrs {
			switch {
			case in.Op == ir.OpLoad && s.Promoted[in.Args[0]]:
				use(in.Args[0], opKey{bi, ii, 0})
			case in.Op == ir.OpStore && s.Promoted[in.Args[0]]:
				use(in.Args[1], opKey{bi, ii, 1})
				add(ssabuild.Def, s.varOf[in.Args[0]], site{kind: siteStore, instr: ii})
			case in.Op == ir.OpAlloc && s.Promoted[in.Dsts[0]]:
				add(ssabuild.Zero, s.varOf[in.Dsts[0]], site{kind: siteZero, instr: ii})
			default:
				for k, a := range operands(in) {
					use(a, opKey{bi, ii, k})
				}
				for k, d := range in.Dsts {
					if v, ok := s.varOf[d]; ok {
						add(ssabuild.Def, v, site{kind: siteDef, instr: ii, dst: k})
					}
				}
			}
		}
		for k, a := range termOperands(b.Term) {
			use(a, opKey{bi, len(b.Instrs), k})
		}
	}
	s.sb = sb
}

// phis converts the φ functions of ssabuild.
func (s *Form) phis() {
	s.Phis = make([][]Phi, len(s.Func.Blocks))
	for bi, vars := range s.res.Phis {
		for _, v := range vars {
			l := s.localOf[v]
			args := s.res.PhiArgs[ssabuild.PhiKey{Block: bi, Var: v}]
			p := Phi{Var: l}
			for _, d := range args {
				p.Args = append(p.Args, s.fromDef(d, l))
			}
			s.Phis[bi] = append(s.Phis[bi], p)
		}
	}
}

// fromDef turns a definition found by ssabuild into a Ref.
func (s *Form) fromDef(d ssabuild.Definition, v ir.LocalID) Ref {
	switch d.Kind {
	case ssabuild.ByPhi:
		return Ref{Kind: RefPhi, Block: d.Block, Var: v}
	case ssabuild.ByInstr:
		st := s.sites[d.Block][d.Instr]
		switch st.kind {
		case siteZero:
			return Ref{Kind: RefZero, Block: d.Block, Index: st.instr, Var: v}
		case siteStore:
			// The value of the variable is the value that was stored.
			return s.Operand(d.Block, st.instr, 1)
		case siteDef:
			return Ref{Kind: RefInstr, Block: d.Block, Index: st.instr, Dst: st.dst}
		}
	}
	return Ref{Kind: RefUndef}
}

// Operand is the definition of the k-th operand of an instruction. For the
// terminator of a block, pass instr = len(Instrs); the operands are then the
// condition of a branch, or the values of a return or a throw.
func (s *Form) Operand(block, instr, k int) Ref {
	key := opKey{block, instr, k}
	if r, ok := s.cache[key]; ok {
		return r
	}
	var l ir.LocalID
	b := s.Func.Blocks[block]
	if instr == len(b.Instrs) {
		l = termOperands(b.Term)[k]
	} else {
		l = b.Instrs[instr].Args[k]
	}
	r := s.resolve(l, key)
	s.cache[key] = r
	return r
}

// Value is the definition of a local at the place of an operand.
func (s *Form) resolve(l ir.LocalID, key opKey) Ref {
	if v, ok := s.varOf[l]; ok && !s.Promoted[l] {
		idx, ok := s.useIdx[key]
		if !ok {
			return Ref{Kind: RefUndef}
		}
		return s.fromDef(s.res.Reach[ssabuild.UseKey{Block: key.block, Instr: idx}], s.localOf[v])
	}
	d, ok := s.def[l]
	if !ok {
		return Ref{Kind: RefUndef}
	}
	def := s.Func.Blocks[d.block].Instrs[d.instr]
	if def.Op == ir.OpLoad && s.Promoted[def.Args[0]] {
		// A load of a promoted variable is the value the variable has there.
		lk := opKey{d.block, d.instr, 0}
		if r, ok := s.cache[lk]; ok {
			return r
		}
		idx := s.useIdx[lk]
		r := s.fromDef(s.res.Reach[ssabuild.UseKey{Block: d.block, Instr: idx}], def.Args[0])
		s.cache[lk] = r
		return r
	}
	return Ref{Kind: RefInstr, Block: d.block, Index: d.instr, Dst: d.dst}
}

// IsPromotedAccess reports whether an instruction is a load or a store of a
// promoted allocation, or the allocation itself: instructions that have no
// meaning in SSA form beyond the Refs they give.
func (s *Form) IsPromotedAccess(in ir.Instr) bool {
	switch in.Op {
	case ir.OpLoad, ir.OpStore:
		return s.Promoted[in.Args[0]]
	case ir.OpAlloc:
		return s.Promoted[in.Dsts[0]]
	}
	return false
}

// allocInstr is the instruction that allocates a promoted variable.
func (s *Form) allocInstr(l ir.LocalID) ir.Instr {
	d := s.allocPos[l]
	return s.Func.Blocks[d.block].Instrs[d.instr]
}

// AllocInstr is the instruction that allocates a promoted variable.
func (s *Form) AllocInstr(l ir.LocalID) ir.Instr { return s.allocInstr(l) }

// Idom is the immediate dominator of a block (-1 for a root or a block that
// is not reached).
func (s *Form) Idom(block int) int {
	if d := s.res.Idom[block]; d >= 0 {
		return d
	}
	return -1
}
