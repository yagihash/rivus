package taint

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/callgraph"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/testutil"
)

// The test cases are Go packages under testdata/go/cases. Their expectations
// are written into the source as annotations:
//
//	x := lib.Source() // @source(tag)
//	lib.Sink(x)       // @sink(tag)
//	lib.Sink("c")     // @sink(none)
//
// Every source line and every sink line sharing a tag must be connected by a
// finding, and no other finding may exist. "@sink(none)" documents a sink
// that must stay quiet; such a line cannot be the sink of any finding.

var annot = regexp.MustCompile(`//\s*@(source|sink)\(([A-Za-z0-9_-]+)\)`)

type line struct {
	file string
	n    int
}

type expectation struct {
	sources map[string][]line // tag -> lines
	sinks   map[string][]line
}

func readExpectations(t *testing.T, dir string) expectation {
	t.Helper()
	exp := expectation{sources: map[string][]line{}, sinks: map[string][]line{}}
	root := filepath.Join(testutil.Testdata(), dir)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		rel, _ := filepath.Rel(testutil.Testdata(), path)
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			if m := annot.FindStringSubmatch(sc.Text()); m != nil {
				l := line{filepath.ToSlash(rel), n}
				if m[1] == "source" {
					exp.sources[m[2]] = append(exp.sources[m[2]], l)
				} else {
					exp.sinks[m[2]] = append(exp.sinks[m[2]], l)
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return exp
}

func (e expectation) pairs() map[string]bool {
	out := map[string]bool{}
	for tag, srcs := range e.sources {
		if tag == "none" {
			continue
		}
		for _, s := range srcs {
			for _, k := range e.sinks[tag] {
				out[pairKey(s, k)] = true
			}
		}
	}
	return out
}

func pairKey(src, sink line) string {
	return fmt.Sprintf("%s:%d -> %s:%d", src.file, src.n, sink.file, sink.n)
}

// runCase analyses testdata/go/cases/<name>/... and returns the result.
func runCase(t testing.TB, name string) (*Result, *sites.Result) {
	t.Helper()
	prog := testutil.Load(t, "./cases/"+name+"/...")
	rules := testutil.Rules(t)
	st := sites.Enumerate(prog, rules)
	eng := New(prog, st, rules, callgraph.BuildGo(prog), DefaultOptions())
	return eng.Run(), st
}

func checkCase(t *testing.T, name string) {
	t.Helper()
	res, _ := runCase(t, name)
	exp := readExpectations(t, "cases/"+name)
	want := exp.pairs()
	got := map[string]bool{}
	for _, f := range res.Findings {
		got[pairKey(line{f.Source.Position.File, f.Source.Position.Line}, line{f.Sink.Position.File, f.Sink.Position.Line})] = true
	}
	if len(want) == 0 {
		t.Fatalf("case %s has no expectations", name)
	}
	t.Logf("%s: %d expected pairs, %d findings, %d uncertainties", name, len(want), len(got), len(res.Uncertainties))
	var missing, extra []string
	for k := range want {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	for k := range got {
		if !want[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	for _, k := range missing {
		t.Errorf("missing finding: %s", k)
	}
	for _, k := range extra {
		t.Errorf("unexpected finding: %s", k)
	}
	if !res.Stats.Converged {
		t.Errorf("analysis did not converge: %+v", res.Stats)
	}
}

func TestIntra(t *testing.T) { checkCase(t, "intra") }

func TestInter(t *testing.T) { checkCase(t, "inter") }

func TestHeap(t *testing.T) { checkCase(t, "heap") }

func TestUnmodeledCallIsRecordedOnThePath(t *testing.T) {
	res, _ := runCase(t, "heap")
	for _, f := range res.Findings {
		if f.Source.Snippet == "" || f.Source.Position.Line == 0 {
			continue
		}
		if f.Sink.Snippet != "lib.Sink(strings.ToValidUTF8(s, \"?\")) // @sink(unmodeled)" {
			continue
		}
		for _, u := range f.Uncertainties {
			if u.Kind == KindUnmodeledLibraryCall {
				if u.AttributableTo != AttributableRivus {
					t.Errorf("unmodeled_library_call must be attributable to rivus, got %q", u.AttributableTo)
				}
				if u.Symbol != "strings.ToValidUTF8" {
					t.Errorf("symbol = %q", u.Symbol)
				}
				return
			}
		}
		t.Fatalf("finding through an unmodeled call has no such uncertainty: %+v", f.Uncertainties)
	}
	t.Fatal("finding not found")
}

func TestDispatchIsRecordedWithCandidates(t *testing.T) {
	res, _ := runCase(t, "inter")
	for _, f := range res.Findings {
		if f.Sink.Snippet != "lib.Sink(s) // @sink(disp)" {
			continue
		}
		for _, u := range f.Uncertainties {
			if u.Kind == KindDynamicDispatch {
				if len(u.Candidates) != 1 || u.AttributableTo != AttributableMixed {
					t.Errorf("unexpected uncertainty: %+v", u)
				}
				return
			}
		}
		t.Fatalf("no dynamic_dispatch uncertainty: %+v", f.Uncertainties)
	}
	t.Fatal("dispatch finding not found")
}

func TestBudgetCutsAFunctionShortAndRecordsIt(t *testing.T) {
	prog := testutil.Load(t, "./cases/inter/...")
	rules := testutil.Rules(t)
	st := sites.Enumerate(prog, rules)
	opts := DefaultOptions()
	opts.Budget = 3 // far too small for any real function
	res := New(prog, st, rules, callgraph.BuildGo(prog), opts).Run()

	var degraded int
	for _, u := range res.Uncertainties {
		if u.Kind == KindDepthLimit {
			degraded++
			if u.AttributableTo != AttributableRivus {
				t.Errorf("depth_limit must be attributable to rivus: %+v", u)
			}
			if u.Func == "" {
				t.Error("the degraded function is not named")
			}
		}
	}
	if degraded == 0 {
		t.Fatal("no depth_limit recorded although the budget was exhausted")
	}
	if !res.Stats.Converged {
		t.Error("a degraded run must still terminate normally")
	}
}

func TestTimeoutStopsTheRunAndSaysSo(t *testing.T) {
	prog := testutil.Load(t, "./cases/inter/...")
	rules := testutil.Rules(t)
	st := sites.Enumerate(prog, rules)
	opts := DefaultOptions()
	opts.Timeout = time.Nanosecond
	res := New(prog, st, rules, callgraph.BuildGo(prog), opts).Run()
	if !res.Stats.TimedOut {
		t.Fatal("the run should have timed out")
	}
	var recorded bool
	for _, u := range res.Uncertainties {
		if u.Kind == KindDepthLimit && u.AttributableTo == AttributableRivus {
			recorded = true
		}
	}
	if !recorded {
		t.Error("a timed out run must record depth_limit")
	}
}
