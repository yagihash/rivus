package ssacheck

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

	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ssabuild"
)

// The test builds the same packages twice. go/ssa lifts the local variables
// of the first into registers, placing φ nodes (pruned SSA). The second is
// built with NaiveForm: the variables stay in memory, which is the form the
// neutral IR has, and ssabuild builds the SSA. If ssabuild is right, it puts
// the φ functions where go/ssa put its own, and every use is reached by
// exactly the definitions that an independent reaching definitions analysis
// finds.

// phiKey names a φ by where it is and which variable it is for. The position
// is the file and offset: the two builds have file sets of their own, so the
// numbers of token.Pos do not compare.
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

func count(m map[phiKey]int, k phiKey, d int) { m[k] += d }

// liftedPhis returns the φ nodes that lifting added to the function: those of
// the lifted build minus those of the naive build, which are the ones that
// && and || produce.
func liftedPhis(lp *gossa.Program, lifted *ssa.Function, np *gossa.Program, naive *ssa.Function) map[phiKey]int {
	out := map[phiKey]int{}
	for _, b := range lifted.Blocks {
		for _, in := range b.Instrs {
			if p, ok := in.(*ssa.Phi); ok {
				count(out, keyOf(lp, b.Index, p.Pos(), p.Comment), 1)
			}
		}
	}
	for _, b := range naive.Blocks {
		for _, in := range b.Instrs {
			if p, ok := in.(*ssa.Phi); ok {
				count(out, keyOf(np, b.Index, p.Pos(), p.Comment), -1)
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
	functions, phis, uses int
	// instances are instantiated generic functions. go/ssa builds them on
	// demand, and the two builds do not always give the same body to the
	// same name, so they cannot be compared.
	instances int
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
			t.Errorf("%s: %d blocks lifted, %d naive: the builds are not comparable", id, len(lfn.Blocks), len(nfn.Blocks))
			continue
		}
		cfg := gossa.NaiveCFGOf(nfn)
		if err := cfg.Func.Validate(); err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		res := ssabuild.Build(cfg.Func)
		st.functions++

		// 1. The φ functions are where go/ssa put them.
		want := liftedPhis(lifted, lfn, naive, nfn)
		got := map[phiKey]int{}
		for blk, vars := range res.Phis {
			for _, v := range vars {
				a := cfg.Vars[v]
				count(got, keyOf(naive, blk, a.Pos(), a.Comment), 1)
				st.phis++
			}
		}
		var diffs []string
		for k, n := range want {
			if got[k] != n {
				diffs = append(diffs, fmt.Sprintf("go/ssa has %d phi at %v, ssabuild %d", n, k, got[k]))
			}
		}
		for k, n := range got {
			if _, ok := want[k]; !ok {
				diffs = append(diffs, fmt.Sprintf("ssabuild has %d phi at %v, go/ssa none", n, k))
			}
		}
		if len(diffs) > 0 {
			sort.Strings(diffs)
			t.Errorf("%s: phi placement differs:\n  %s", id, strings.Join(diffs, "\n  "))
			continue
		}

		// 2. Every use is reached by the definitions an independent
		// analysis finds.
		oracle := ssabuild.ReachingDefinitions(cfg.Func)
		for k, wantSet := range oracle {
			in := cfg.Func.Blocks[k.Block].Instrs[k.Instr]
			st.uses++
			if gotSet := res.Expand(res.Reach[k], in.Var); !gotSet.Equal(wantSet) {
				t.Errorf("%s: the use in block %d of %s is reached by %+v in SSA, by %+v in the reaching definitions analysis",
					id, k.Block, cfg.Vars[in.Var].Comment, gotSet, wantSet)
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
	t.Logf("%d functions, %d phis, %d uses checked (%d generic instances left out)", st.functions, st.phis, st.uses, st.instances)
	if st.functions < 50 || st.phis < 5 {
		t.Errorf("too little was checked: %+v", st)
	}
}

func TestAgainstGoSSAOnRivusItself(t *testing.T) {
	st := checkModule(t, repoRoot(), []string{"./..."})
	t.Logf("%d functions, %d phis, %d uses checked (%d generic instances left out)", st.functions, st.phis, st.uses, st.instances)
	if st.functions < 300 || st.phis < 100 {
		t.Errorf("too little was checked: %+v", st)
	}
}

// TestAgainstGoSSAOnAModule runs the check on a module of the user's choice,
// for example a large one: RIVUS_TEST_MODULE=/path/to/module.
func TestAgainstGoSSAOnAModule(t *testing.T) {
	dir := os.Getenv("RIVUS_TEST_MODULE")
	if dir == "" {
		t.Skip("RIVUS_TEST_MODULE is not set")
	}
	st := checkModule(t, dir, []string{"./..."})
	t.Logf("%d functions, %d phis, %d uses checked (%d generic instances left out)", st.functions, st.phis, st.uses, st.instances)
}
