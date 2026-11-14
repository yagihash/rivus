// Package ssabuild turns a control flow graph with mutable variables into
// static single assignment form: it computes the dominator tree and the
// dominance frontiers, places the φ functions, and finds for every use the
// definition that reaches it.
//
// It knows nothing of any language or of go/ssa: this is the part that the
// architecture puts in the core, once, for every extractor (ADR-003). In
// Phase 1 the analysis still runs on go/ssa; this package is built and
// verified beside it, against go/ssa's own lifting and against an
// independent reaching definitions analysis (see internal/ssacheck).
package ssabuild

import (
	"fmt"
	"sort"
)

// Op is what an instruction does to a variable.
type Op uint8

const (
	// Use reads the variable.
	Use Op = iota
	// Def writes the variable.
	Def
	// Zero writes the zero value: the declaration of the variable. It
	// counts as a definition.
	Zero
)

// Instr is an instruction as far as variables are concerned. Instructions
// that do not touch a variable are not represented.
type Instr struct {
	Op  Op
	Var int
}

// Block is a basic block.
type Block struct {
	Succs  []int
	Instrs []Instr
}

// Func is a control flow graph. Roots lists the blocks where execution can
// begin: the entry block, and for a function with a deferred recover the
// block that runs after a panic. Blocks[Roots[0]] is the entry.
//
// A root has no predecessor: control never jumps back to it.
type Func struct {
	Blocks []Block
	Roots  []int
	NVars  int
}

// Validate checks what Build relies on.
func (f *Func) Validate() error {
	if len(f.Roots) == 0 {
		return fmt.Errorf("the function has no root block")
	}
	isRoot := map[int]bool{}
	for _, r := range f.Roots {
		if r < 0 || r >= len(f.Blocks) {
			return fmt.Errorf("root %d is not a block", r)
		}
		isRoot[r] = true
	}
	for i, b := range f.Blocks {
		for _, s := range b.Succs {
			if s < 0 || s >= len(f.Blocks) {
				return fmt.Errorf("block %d jumps to %d, which is not a block", i, s)
			}
			if isRoot[s] {
				return fmt.Errorf("block %d jumps to the root block %d", i, s)
			}
		}
		for _, in := range b.Instrs {
			if in.Var < 0 || in.Var >= f.NVars {
				return fmt.Errorf("block %d uses variable %d of %d", i, in.Var, f.NVars)
			}
		}
	}
	return nil
}

// DefKind says what a reaching definition is.
type DefKind uint8

const (
	// Undefined means nothing defines the variable on some path.
	Undefined DefKind = iota
	// ByInstr is a Def or Zero instruction.
	ByInstr
	// ByPhi is the φ function of the variable at the start of a block.
	ByPhi
)

// Definition names a definition of a variable.
type Definition struct {
	Kind  DefKind
	Block int
	// Instr is the index of the instruction in the block (ByInstr).
	Instr int
}

// PhiKey identifies a φ function: the variable and the block.
type PhiKey struct{ Block, Var int }

// UseKey identifies an instruction.
type UseKey struct{ Block, Instr int }

// Result is the SSA form.
type Result struct {
	// Phis lists, for each block, the variables that have a φ there.
	Phis [][]int
	// PhiArgs holds, for each φ, the definition that comes in from each
	// predecessor, in the order of Preds[block].
	PhiArgs map[PhiKey][]Definition
	// Reach gives the definition that reaches each use.
	Reach map[UseKey]Definition
	// Preds lists the predecessors of each block, in a fixed order.
	Preds [][]int
	// Idom is the immediate dominator of each block (-1 for a root).
	Idom []int
}

// Build computes the SSA form of the function. Only the φ functions that
// some use depends on are kept (pruned SSA), as go/ssa does.
func Build(f *Func) *Result {
	n := len(f.Blocks)
	b := &builder{f: f, n: n}
	b.preds()
	b.dominators()
	b.frontiers()
	b.placePhis()
	b.rename()
	b.pruneDeadPhis()
	return b.result()
}

type builder struct {
	f *Func
	n int

	preds_ [][]int
	idom   []int   // -1 for roots, -2 for unreachable
	kids   [][]int // dominator tree children
	order  []int   // reverse postorder of the reachable blocks
	df     [][]int

	hasPhi  []map[int]bool // block -> var -> has a φ
	phiArgs map[PhiKey][]Definition
	reach   map[UseKey]Definition
}

func (b *builder) preds() {
	b.preds_ = make([][]int, b.n)
	for i, blk := range b.f.Blocks {
		for _, s := range blk.Succs {
			b.preds_[s] = append(b.preds_[s], i)
		}
	}
}

// dominators computes immediate dominators with the algorithm of Cooper,
// Harvey and Kennedy. The roots hang below a virtual root, so that the
// recover block of go/ssa gets a tree of its own.
func (b *builder) dominators() {
	const virtual = -1
	// Reverse postorder over the reachable blocks, from the virtual root.
	visited := make([]bool, b.n)
	var post []int
	var visit func(v int)
	visit = func(v int) {
		visited[v] = true
		for _, s := range b.f.Blocks[v].Succs {
			if !visited[s] {
				visit(s)
			}
		}
		post = append(post, v)
	}
	for _, r := range b.f.Roots {
		if !visited[r] {
			visit(r)
		}
	}
	b.order = make([]int, len(post))
	num := make([]int, b.n) // reverse postorder number; roots come first
	for i := range num {
		num[i] = -1
	}
	for i, v := range post {
		b.order[len(post)-1-i] = v
	}
	for i, v := range b.order {
		num[v] = i
	}

	b.idom = make([]int, b.n)
	for i := range b.idom {
		b.idom[i] = -2
	}
	isRoot := map[int]bool{}
	for _, r := range b.f.Roots {
		isRoot[r] = true
		b.idom[r] = virtual
	}
	intersect := func(x, y int) int {
		for x != y {
			for x != virtual && (y == virtual || num[x] > num[y]) {
				x = b.idom[x]
			}
			for y != virtual && (x == virtual || num[y] > num[x]) {
				y = b.idom[y]
			}
			if x == virtual || y == virtual {
				return virtual
			}
		}
		return x
	}
	for changed := true; changed; {
		changed = false
		for _, v := range b.order {
			if isRoot[v] {
				continue
			}
			nd := -2
			for _, p := range b.preds_[v] {
				if b.idom[p] == -2 {
					continue
				}
				if nd == -2 {
					nd = p
				} else {
					nd = intersect(p, nd)
				}
			}
			if nd != b.idom[v] {
				b.idom[v] = nd
				changed = true
			}
		}
	}
	b.kids = make([][]int, b.n)
	for _, v := range b.order {
		if p := b.idom[v]; p >= 0 {
			b.kids[p] = append(b.kids[p], v)
		}
	}
}

// frontiers computes the dominance frontier of every block.
func (b *builder) frontiers() {
	b.df = make([][]int, b.n)
	seen := make([]map[int]bool, b.n)
	for _, v := range b.order {
		ps := b.preds_[v]
		if len(ps) < 2 {
			continue
		}
		// A join point: every block on the way up from a predecessor to
		// the immediate dominator of v has v in its frontier.
		for _, p := range ps {
			if b.idom[p] == -2 {
				continue
			}
			for r := p; r >= 0 && r != b.idom[v]; r = b.idom[r] {
				if seen[r] == nil {
					seen[r] = map[int]bool{}
				}
				if !seen[r][v] {
					seen[r][v] = true
					b.df[r] = append(b.df[r], v)
				}
			}
		}
	}
	for _, l := range b.df {
		sort.Ints(l)
	}
}

// placePhis puts a φ for each variable at the iterated dominance frontier of
// the blocks that define it.
func (b *builder) placePhis() {
	b.hasPhi = make([]map[int]bool, b.n)
	for v := 0; v < b.f.NVars; v++ {
		var work []int
		inWork := map[int]bool{}
		for i, blk := range b.f.Blocks {
			if b.idom[i] == -2 {
				continue
			}
			for _, in := range blk.Instrs {
				if in.Var == v && in.Op != Use && !inWork[i] {
					inWork[i] = true
					work = append(work, i)
				}
			}
		}
		for len(work) > 0 {
			u := work[len(work)-1]
			work = work[:len(work)-1]
			for _, d := range b.df[u] {
				if b.hasPhi[d] == nil {
					b.hasPhi[d] = map[int]bool{}
				}
				if !b.hasPhi[d][v] {
					b.hasPhi[d][v] = true
					if !inWork[d] {
						inWork[d] = true
						work = append(work, d)
					}
				}
			}
		}
	}
}

// rename walks the dominator tree and finds, for each use and for each φ
// argument, the definition on top of the stack of its variable.
func (b *builder) rename() {
	b.phiArgs = map[PhiKey][]Definition{}
	b.reach = map[UseKey]Definition{}
	stacks := make([][]Definition, b.f.NVars)
	top := func(v int) Definition {
		if s := stacks[v]; len(s) > 0 {
			return s[len(s)-1]
		}
		return Definition{Kind: Undefined}
	}
	var walk func(blk int)
	walk = func(blk int) {
		var pushed []int
		for v := 0; v < b.f.NVars; v++ {
			if b.hasPhi[blk][v] {
				stacks[v] = append(stacks[v], Definition{Kind: ByPhi, Block: blk})
				pushed = append(pushed, v)
			}
		}
		for i, in := range b.f.Blocks[blk].Instrs {
			switch in.Op {
			case Use:
				b.reach[UseKey{blk, i}] = top(in.Var)
			default:
				stacks[in.Var] = append(stacks[in.Var], Definition{Kind: ByInstr, Block: blk, Instr: i})
				pushed = append(pushed, in.Var)
			}
		}
		// The arguments of the φ functions of the successors.
		used := map[int]int{}
		for _, s := range b.f.Blocks[blk].Succs {
			// The slot of this predecessor; a block that is a predecessor
			// twice fills its slots in turn.
			slot := -1
			skip := used[s]
			for j, p := range b.preds_[s] {
				if p == blk {
					if skip == 0 {
						slot = j
						break
					}
					skip--
				}
			}
			used[s]++
			if slot < 0 {
				continue
			}
			for v := range b.hasPhi[s] {
				k := PhiKey{s, v}
				if b.phiArgs[k] == nil {
					b.phiArgs[k] = make([]Definition, len(b.preds_[s]))
				}
				b.phiArgs[k][slot] = top(v)
			}
		}
		for _, c := range b.kids[blk] {
			walk(c)
		}
		for i := len(pushed) - 1; i >= 0; i-- {
			v := pushed[i]
			stacks[v] = stacks[v][:len(stacks[v])-1]
		}
	}
	// The roots, and the blocks that more than one root leads to, are the
	// children of the virtual root: each starts a tree of the forest.
	for _, v := range b.order {
		if b.idom[v] == -1 {
			walk(v)
		}
	}
}

// pruneDeadPhis drops the φ functions that no use depends on, not even
// through other φ functions.
func (b *builder) pruneDeadPhis() {
	live := map[PhiKey]bool{}
	var mark func(d Definition, v int)
	mark = func(d Definition, v int) {
		if d.Kind != ByPhi {
			return
		}
		k := PhiKey{d.Block, v}
		if live[k] {
			return
		}
		live[k] = true
		for _, a := range b.phiArgs[k] {
			mark(a, v)
		}
	}
	for k, d := range b.reach {
		in := b.f.Blocks[k.Block].Instrs[k.Instr]
		mark(d, in.Var)
	}
	for blk, vars := range b.hasPhi {
		for v := range vars {
			if !live[PhiKey{blk, v}] {
				delete(vars, v)
				delete(b.phiArgs, PhiKey{blk, v})
			}
		}
	}
}

func (b *builder) result() *Result {
	r := &Result{
		Phis:    make([][]int, b.n),
		PhiArgs: b.phiArgs,
		Reach:   b.reach,
		Preds:   b.preds_,
		Idom:    b.idom,
	}
	for blk, vars := range b.hasPhi {
		for v := range vars {
			r.Phis[blk] = append(r.Phis[blk], v)
		}
		sort.Ints(r.Phis[blk])
	}
	return r
}
