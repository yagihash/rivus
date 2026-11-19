package ssabuild

import (
	"math/rand"
	"testing"
	"time"
)

func use(v int) Instr  { return Instr{Use, v} }
func def(v int) Instr  { return Instr{Def, v} }
func zero(v int) Instr { return Instr{Zero, v} }

func phiAt(r *Result, block, v int) bool {
	for _, x := range r.Phis[block] {
		if x == v {
			return true
		}
	}
	return false
}

// The diamond:   0 -> 1, 2 -> 3.
func diamond(b0, b1, b2, b3 []Instr) *Func {
	return &Func{
		Roots: []int{0}, NVars: 1,
		Blocks: []Block{{Succs: []int{1, 2}, Instrs: b0}, {Succs: []int{3}, Instrs: b1}, {Succs: []int{3}, Instrs: b2}, {Instrs: b3}},
	}
}

func TestStraightLineNeedsNoPhi(t *testing.T) {
	f := &Func{Roots: []int{0}, NVars: 1, Blocks: []Block{{Succs: []int{1}, Instrs: []Instr{zero(0), def(0)}}, {Instrs: []Instr{use(0)}}}}
	r := Build(f)
	if phiAt(r, 1, 0) || len(r.PhiArgs) != 0 {
		t.Fatalf("phis = %v", r.Phis)
	}
	if d := r.Reach[UseKey{1, 0}]; d.Kind != ByInstr || d.Block != 0 || d.Instr != 1 {
		t.Errorf("the use is reached by the second definition of block 0, got %+v", d)
	}
}

func TestDiamondPlacesAPhiAtTheJoin(t *testing.T) {
	f := diamond([]Instr{zero(0)}, []Instr{def(0)}, []Instr{def(0)}, []Instr{use(0)})
	r := Build(f)
	if !phiAt(r, 3, 0) {
		t.Fatalf("phis = %v", r.Phis)
	}
	args := r.PhiArgs[PhiKey{3, 0}]
	if len(args) != 2 || args[0].Block != 1 || args[1].Block != 2 {
		t.Errorf("each predecessor brings its own definition, got %+v", args)
	}
	if d := r.Reach[UseKey{3, 0}]; d.Kind != ByPhi {
		t.Errorf("the use is reached by the phi, got %+v", d)
	}
}

func TestOneSidedDefinitionBringsTheZeroInFromTheOtherSide(t *testing.T) {
	f := diamond([]Instr{zero(0)}, []Instr{def(0)}, nil, []Instr{use(0)})
	r := Build(f)
	args := r.PhiArgs[PhiKey{3, 0}]
	if len(args) != 2 || args[0].Block != 1 || args[1].Block != 0 {
		t.Fatalf("args = %+v", args)
	}
}

func TestDeadPhiIsPruned(t *testing.T) {
	// The variable is defined on both sides but never read afterwards.
	f := diamond([]Instr{zero(0)}, []Instr{def(0)}, []Instr{def(0)}, nil)
	if r := Build(f); phiAt(r, 3, 0) {
		t.Error("a phi that nothing uses must not be kept")
	}
	// A use inside one arm needs no phi either.
	g := diamond([]Instr{zero(0)}, []Instr{def(0), use(0)}, []Instr{def(0)}, nil)
	if r := Build(g); phiAt(r, 3, 0) {
		t.Error("the use is inside the arm: no phi at the join")
	}
}

func TestLoopPutsAPhiAtTheHeader(t *testing.T) {
	// 0 -> 1 (header) -> 2 (body) -> 1,  1 -> 3 (exit)
	f := &Func{Roots: []int{0}, NVars: 1, Blocks: []Block{
		{Succs: []int{1}, Instrs: []Instr{zero(0)}},
		{Succs: []int{2, 3}, Instrs: []Instr{use(0)}},
		{Succs: []int{1}, Instrs: []Instr{use(0), def(0)}},
		{},
	}}
	r := Build(f)
	if !phiAt(r, 1, 0) {
		t.Fatalf("the header joins the entry and the back edge: %v", r.Phis)
	}
	args := r.PhiArgs[PhiKey{1, 0}]
	if len(args) != 2 || args[0].Block != 0 || args[1].Block != 2 {
		t.Errorf("args = %+v", args)
	}
}

func TestSecondRootHasATreeOfItsOwn(t *testing.T) {
	// 0 is the entry, 1 runs after a panic (a recover block), both reach 2.
	f := &Func{Roots: []int{0, 1}, NVars: 1, Blocks: []Block{
		{Succs: []int{2}, Instrs: []Instr{zero(0), def(0)}},
		{Succs: []int{2}, Instrs: []Instr{def(0)}},
		{Instrs: []Instr{use(0)}},
	}}
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
	r := Build(f)
	if !phiAt(r, 2, 0) {
		t.Fatalf("phis = %v", r.Phis)
	}
	if r.Idom[2] != -1 || r.Idom[1] != -1 {
		t.Errorf("idom = %v", r.Idom)
	}
}

func TestUnreachableBlocksAreIgnored(t *testing.T) {
	f := &Func{Roots: []int{0}, NVars: 1, Blocks: []Block{
		{Instrs: []Instr{zero(0)}},
		{Succs: []int{0}, Instrs: []Instr{use(0), def(0)}}, // nobody jumps here
	}}
	r := Build(f)
	if _, ok := r.Reach[UseKey{1, 0}]; ok {
		t.Error("a use in a dead block has no reaching definition")
	}
}

// randomFunc builds a random graph. Every block is reachable from block 0 by
// construction (block i gets an edge from an earlier block); extra edges,
// including back edges, make loops and irreducible regions.
func randomFunc(rng *rand.Rand) *Func {
	n := 2 + rng.Intn(12)
	nv := 1 + rng.Intn(3)
	f := &Func{Roots: []int{0}, NVars: nv, Blocks: make([]Block, n)}
	for i := 1; i < n; i++ {
		p := rng.Intn(i)
		f.Blocks[p].Succs = append(f.Blocks[p].Succs, i)
	}
	for k := rng.Intn(n); k > 0; k-- {
		a, b := rng.Intn(n), 1+rng.Intn(n-1) // nothing jumps back to the entry
		if len(f.Blocks[a].Succs) < 2 {
			f.Blocks[a].Succs = append(f.Blocks[a].Succs, b)
		}
	}
	if rng.Intn(4) == 0 {
		// A second root, like the recover block of go/ssa: nothing jumps
		// to it, and it leads into the function.
		f.Blocks = append(f.Blocks, Block{Succs: []int{1 + rng.Intn(n-1)}})
		f.Roots = append(f.Roots, n)
		n++
	}
	for i := range f.Blocks {
		for k := rng.Intn(5); k > 0; k-- {
			op := Op(rng.Intn(3))
			if op == Zero && rng.Intn(2) == 0 {
				op = Def
			}
			f.Blocks[i].Instrs = append(f.Blocks[i].Instrs, Instr{op, rng.Intn(nv)})
		}
	}
	// Half of the functions declare the variables in the entry block, as
	// go/ssa does with the Alloc; the others leave paths without a definition.
	if rng.Intn(2) == 0 {
		var decl []Instr
		for v := 0; v < nv; v++ {
			decl = append(decl, zero(v))
		}
		f.Blocks[0].Instrs = append(decl, f.Blocks[0].Instrs...)
	}
	return f
}

// dominates reports whether block a dominates block b in the tree of r.
func dominates(r *Result, a, b int) bool {
	for x := b; x >= 0; x = r.Idom[x] {
		if x == a {
			return true
		}
	}
	return false
}

func TestRandomGraphsAgreeWithReachingDefinitions(t *testing.T) {
	rng := rand.New(rand.NewSource(20260101))
	phis := 0
	for iter := 0; iter < 5000; iter++ {
		f := randomFunc(rng)
		if err := f.Validate(); err != nil {
			t.Fatalf("iteration %d: the generator made an invalid graph: %v", iter, err)
		}
		r := Build(f)
		oracle := ReachingDefinitions(f)

		// The definition SSA gives for a use, followed through its phis,
		// is exactly the set of definitions that reach the use.
		for k, want := range oracle {
			in := f.Blocks[k.Block].Instrs[k.Instr]
			got := r.Expand(r.Reach[k], in.Var)
			if !got.Equal(want) {
				t.Fatalf("iteration %d, use %+v of v%d: SSA says %+v, reaching definitions say %+v\ngraph: %+v", iter, k, in.Var, got, want, f)
			}
		}
		if len(oracle) != len(r.Reach) {
			t.Fatalf("iteration %d: %d uses against %d", iter, len(r.Reach), len(oracle))
		}

		// Structure: a phi has one argument per predecessor, and every
		// definition dominates what it reaches.
		for blk, vars := range r.Phis {
			for _, v := range vars {
				phis++
				if got := len(r.PhiArgs[PhiKey{blk, v}]); got != len(r.Preds[blk]) {
					t.Fatalf("iteration %d: phi of v%d at %d has %d arguments for %d predecessors", iter, v, blk, got, len(r.Preds[blk]))
				}
			}
		}
		for k, d := range r.Reach {
			switch d.Kind {
			case ByInstr:
				if d.Block == k.Block {
					if d.Instr >= k.Instr {
						t.Fatalf("iteration %d: the definition comes after the use", iter)
					}
				} else if !dominates(r, d.Block, k.Block) {
					t.Fatalf("iteration %d: the definition in %d does not dominate the use in %d", iter, d.Block, k.Block)
				}
			case ByPhi:
				if !dominates(r, d.Block, k.Block) {
					t.Fatalf("iteration %d: the phi in %d does not dominate the use in %d", iter, d.Block, k.Block)
				}
			}
		}
	}
	if phis < 500 {
		t.Errorf("only %d phis in 5000 graphs: the generator does not exercise the interesting cases", phis)
	}
}

// A long chain of blocks, each with a variable of its own: the time grows with the size of the function,
// not with the blocks times the variables (yagihash/rivus #25). The bound is generous: the quadratic
// build took seconds on 20,000 blocks, this one milliseconds.
func TestLongChainOfBlocksIsNotQuadratic(t *testing.T) {
	const n = 30000
	f := &Func{Roots: []int{0}, NVars: n}
	for i := 0; i < n; i++ {
		b := Block{Instrs: []Instr{zero(i), def(i), use(i)}}
		if i+1 < n {
			b.Succs = []int{i + 1}
		}
		f.Blocks = append(f.Blocks, b)
	}
	start := time.Now()
	r := Build(f)
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("Build took %v for %d blocks and variables", d, n)
	}
	if len(r.Reach) != n {
		t.Errorf("%d uses reached, want %d", len(r.Reach), n)
	}
}
