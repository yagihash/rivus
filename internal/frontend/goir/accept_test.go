package goir_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/accept"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/testutil"
)

// The extractor in use, read twice, gives the same result: the same functions, the same IR and candidates of each.
// A new extractor is put in as the second one.
var stdPatterns = []string{"bytes", "strings", "sort", "sync", "net/http", "encoding/json", "go/types", "text/template", "regexp", "bufio"}

func repoRoot() string { return filepath.Dir(filepath.Dir(testutil.Testdata())) }

func runBoth(t *testing.T, a, b accept.Extractor, dir string, patterns ...string) (*ir.Program, *ir.Program) {
	t.Helper()
	pa, err := a.Run(dir, patterns, accept.Options{})
	if err != nil {
		t.Fatalf("%s: %v", a.Name, err)
	}
	pb, err := b.Run(dir, patterns, accept.Options{})
	if err != nil {
		t.Fatalf("%s: %v", b.Name, err)
	}
	return pa, pb
}

// acceptOrder reads the code in the order it is by itself, and again in other orders of the load (the packages in the
// reverse order, the files into the file set in other orders: see gossa.loadOrderEnv). The results must be the same.
func acceptOrder(t *testing.T, dir string, patterns ...string) {
	t.Helper()
	base, err := accept.Current.Run(dir, patterns, accept.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range []string{"a", "b", "c"} {
		t.Setenv("RIVUS_INTERNAL_LOAD_ORDER", seed)
		q, err := accept.Current.Run(dir, patterns, accept.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if r := accept.Compare(base, q); !r.Empty() {
			t.Fatalf("the order of the load (seed %q) changes the result:\n%s", seed, r.Summary(20))
		}
	}
}

func TestAcceptOrderTestdata(t *testing.T) { acceptOrder(t, testutil.Testdata(), "./...") }

func TestAcceptOrderCurated(t *testing.T) {
	acceptOrder(t, filepath.Join(repoRoot(), "bench", "go-curated"), "./...")
}

func TestAcceptOrderZoo(t *testing.T) {
	acceptOrder(t, filepath.Join(repoRoot(), "bench", "zoo-go"), "./...")
}

func TestAcceptOrderStd(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the standard library")
	}
	acceptOrder(t, filepath.Join(runtime.GOROOT(), "src"), stdPatterns...)
}

func acceptSame(t *testing.T, dir string, patterns ...string) {
	t.Helper()
	pa, pb := runBoth(t, accept.Current, accept.Current, dir, patterns...)
	if r := accept.Compare(pa, pb); !r.Empty() {
		limit := 20
		if testing.Verbose() || os.Getenv("RIVUS_ACCEPT_ALL") != "" {
			limit = 0
		}
		t.Fatalf("the two results differ:\n%s", r.Summary(limit))
	}
}

func TestAcceptTestdata(t *testing.T) {
	acceptSame(t, testutil.Testdata(), "./...")
}

func TestAcceptCurated(t *testing.T) {
	acceptSame(t, filepath.Join(repoRoot(), "bench", "go-curated"), "./...")
}

func TestAcceptZoo(t *testing.T) {
	acceptSame(t, filepath.Join(repoRoot(), "bench", "zoo-go"), "./...")
}

// The standard library is a material that the zoo does not have: many functions, third-party-free, every
// feature of the language.
func TestAcceptStd(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the standard library")
	}
	acceptSame(t, filepath.Join(runtime.GOROOT(), "src"), stdPatterns...)
}

// The test must see a difference: a check that finds none when one is put in sees nothing.
func TestAcceptSeesWhatIsMissing(t *testing.T) {
	p, err := accept.Current.Run(testutil.Testdata(), []string{"./..."}, accept.Options{})
	if err != nil {
		t.Fatal(err)
	}
	same, err := accept.Current.Run(testutil.Testdata(), []string{"./..."}, accept.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r := accept.Compare(p, same); !r.Empty() {
		t.Fatalf("the same code, read twice, differs:\n%s", r.Summary(20))
	}
	clone := func() *ir.Program {
		q, err := accept.Current.Run(testutil.Testdata(), []string{"./..."}, accept.Options{})
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	callIn := func(q *ir.Program, withCandidates bool) (*ir.Func, int, int) {
		for _, f := range q.Funcs {
			for bi := range f.Blocks {
				for ii, in := range f.Blocks[bi].Instrs {
					if in.Callee != nil && (!withCandidates || len(in.Callee.Candidates) > 0) {
						return f, bi, ii
					}
				}
			}
		}
		t.Fatal("no such call in the testdata")
		return nil, 0, 0
	}
	cases := map[string]func(q *ir.Program){
		"an instruction removed": func(q *ir.Program) {
			f, bi, ii := callIn(q, false)
			is := f.Blocks[bi].Instrs
			f.Blocks[bi].Instrs = append(append([]ir.Instr(nil), is[:ii]...), is[ii+1:]...)
		},
		"an operator changed": func(q *ir.Program) {
			f, bi, ii := callIn(q, false)
			f.Blocks[bi].Instrs[ii].Callee.Symbol += "x"
		},
		"a function removed": func(q *ir.Program) { q.Funcs = q.Funcs[1:] },
		"a candidate removed": func(q *ir.Program) {
			f, bi, ii := callIn(q, true)
			c := f.Blocks[bi].Instrs[ii].Callee
			c.Candidates = c.Candidates[1:]
		},
		"a global removed": func(q *ir.Program) {
			if len(q.Globals) == 0 {
				t.Skip("no globals")
			}
			q.Globals = q.Globals[1:]
		},
	}
	for name, mutate := range cases {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			q := clone()
			mutate(q)
			if r := accept.Compare(p, q); r.Empty() {
				t.Fatalf("%s: no difference was found", name)
			}
		})
	}
}
