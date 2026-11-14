package irssa

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ssabuild"
)

// The test checks the SSA form of the IR against go/ssa's own lifting, the
// way internal/ssacheck checks ssabuild: the same packages are built twice,
// once lifted and once as NaiveForm, and the naive one goes through the
// extractor into the IR and then through this package. The φ functions of the
// promoted allocations must be where go/ssa put them, those of the locals that
// the extractor lowered from φ functions must be where the naive build had its
// own, and every use must be reached by the definitions that an independent
// reaching definitions analysis finds.

type phiKey struct {
	block   int
	file    string
	offset  int
	comment string
}

func (k phiKey) String() string {
	return fmt.Sprintf("block %d, %q at %s:%d", k.block, k.comment, k.file, k.offset)
}

func keyOf(p *gossa.Program, block int, pos token.Pos, comment string) phiKey {
	at := p.Position(pos)
	return phiKey{block, at.File, at.Offset, comment}
}

func liftedPhis(lp *gossa.Program, lifted *ssa.Function, np *gossa.Program, naive *ssa.Function) map[phiKey]int {
	out := map[phiKey]int{}
	for _, b := range lifted.Blocks {
		for _, in := range b.Instrs {
			if p, ok := in.(*ssa.Phi); ok {
				out[keyOf(lp, b.Index, p.Pos(), p.Comment)]++
			}
		}
	}
	for _, b := range naive.Blocks {
		for _, in := range b.Instrs {
			if p, ok := in.(*ssa.Phi); ok {
				out[keyOf(np, b.Index, p.Pos(), p.Comment)]--
			}
		}
	}
	for k, n := range out {
		if n == 0 {
			delete(out, k)
		}
	}
	return out
}

type stats struct {
	functions, promoted, phis, lowered, uses, instances int
}

func checkModule(t *testing.T, dir string, patterns []string) stats {
	t.Helper()
	lifted, err := gossa.Load(dir, patterns, gossa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	naive, err := gossa.Load(dir, patterns, gossa.Options{NaiveForm: true})
	if err != nil {
		t.Fatal(err)
	}
	liftedByID := map[string]*ssa.Function{}
	for _, fn := range lifted.Funcs {
		liftedByID[gossa.FuncID(fn)] = fn
	}
	var st stats
	for _, nfn := range naive.Funcs {
		if !naive.Own[nfn] {
			continue
		}
		id := gossa.FuncID(nfn)
		lfn := liftedByID[id]
		if lfn == nil {
			t.Errorf("%s: not in the lifted build", id)
			continue
		}
		if len(lfn.Blocks) != len(nfn.Blocks) {
			if lfn.Origin() != nil || nfn.Origin() != nil {
				st.instances++
				continue
			}
			t.Errorf("%s: %d blocks lifted, %d naive", id, len(lfn.Blocks), len(nfn.Blocks))
			continue
		}
		f := goir.Func(naive, nfn)
		form, err := Build(f)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		st.functions++
		st.promoted += len(form.Promoted)

		// 1. The φ functions of the promoted allocations are those that
		// lifting added.
		want := liftedPhis(lifted, lfn, naive, nfn)
		got := map[phiKey]int{}
		lowered := map[int]int{}
		for blk, phis := range form.Phis {
			for _, p := range phis {
				if form.Promoted[p.Var] {
					a := form.allocInstr(p.Var)
					got[phiKey{blk, a.Pos.File, a.Pos.Offset, f.Locals[p.Var].Name}]++
					st.phis++
				} else {
					lowered[blk]++
					st.lowered++
				}
			}
		}
		var diffs []string
		for k, n := range want {
			if got[k] != n {
				diffs = append(diffs, fmt.Sprintf("go/ssa has %d phi at %v, irssa %d", n, k, got[k]))
			}
		}
		for k, n := range got {
			if _, ok := want[k]; !ok {
				diffs = append(diffs, fmt.Sprintf("irssa has %d phi at %v, go/ssa none", n, k))
			}
		}
		// 2. The φ functions of the lowered locals are where the naive
		// build had its own (those that survive, as the naive ones are
		// never removed by lifting).
		naivePhis := map[int]int{}
		for _, b := range nfn.Blocks {
			for _, in := range b.Instrs {
				if _, ok := in.(*ssa.Phi); ok {
					naivePhis[b.Index]++
				}
			}
		}
		for blk, n := range naivePhis {
			if lowered[blk] > n {
				diffs = append(diffs, fmt.Sprintf("block %d: %d lowered phis, the naive build had %d", blk, lowered[blk], n))
			}
		}
		if len(diffs) > 0 {
			sort.Strings(diffs)
			t.Errorf("%s: phi placement differs:\n  %s", id, strings.Join(diffs, "\n  "))
			continue
		}

		// 3. Every use is reached by what an independent analysis finds.
		sb, res := form.SSABuild()
		oracle := ssabuild.ReachingDefinitions(sb)
		for k, wantSet := range oracle {
			in := sb.Blocks[k.Block].Instrs[k.Instr]
			st.uses++
			if gotSet := res.Expand(res.Reach[k], in.Var); !gotSet.Equal(wantSet) {
				t.Errorf("%s: a use in block %d of %%%d is reached by %+v, by the oracle %+v", id, k.Block, form.VarLocal(in.Var), gotSet, wantSet)
			}
		}
	}
	return st
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func TestAgainstGoSSAOnTheTestCases(t *testing.T) {
	st := checkModule(t, filepath.Join(repoRoot(), "testdata", "go"), []string{"./..."})
	t.Logf("%+v", st)
	if st.functions < 50 || st.phis < 5 {
		t.Errorf("too little was checked: %+v", st)
	}
}

func TestAgainstGoSSAOnRivusItself(t *testing.T) {
	st := checkModule(t, repoRoot(), []string{"./..."})
	t.Logf("%+v", st)
	if st.functions < 300 || st.phis < 100 {
		t.Errorf("too little was checked: %+v", st)
	}
}

func TestAgainstGoSSAOnAModule(t *testing.T) {
	dir := os.Getenv("RIVUS_TEST_MODULE")
	if dir == "" {
		t.Skip("RIVUS_TEST_MODULE is not set")
	}
	t.Logf("%+v", checkModule(t, dir, []string{"./..."}))
}
