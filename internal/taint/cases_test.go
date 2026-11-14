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
	rules.Expand(prog, 0)
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

func TestUncertain(t *testing.T) { checkCase(t, "uncertain") }

// kindsOn returns the uncertainty kinds on the path of the finding whose
// sink line contains the marker.
func kindsOn(t *testing.T, res *Result, marker string) map[string]string {
	t.Helper()
	for _, f := range res.Findings {
		if strings.Contains(f.Sink.Snippet, marker) {
			out := map[string]string{}
			for _, u := range f.Uncertainties {
				out[u.Kind] = u.AttributableTo
			}
			return out
		}
	}
	t.Fatalf("no finding for %q", marker)
	return nil
}

func TestPathUncertaintyKinds(t *testing.T) {
	res, _ := runCase(t, "uncertain")
	want := map[string]map[string]string{
		"@sink(chan)":      {KindConcurrency: AttributableMixed},
		"@sink(goroutine)": {KindConcurrency: AttributableMixed},
		"@sink(select)":    {KindConcurrency: AttributableMixed},
		"@sink(slice)":     {KindContainerImprecision: AttributableRivus},
		"@sink(global)":    {KindGlobalVar: AttributableTarget},
		"@sink(reflect)":   {KindReflection: AttributableTarget},
	}
	for marker, kinds := range want {
		got := kindsOn(t, res, marker)
		for k, attr := range kinds {
			if got[k] != attr {
				t.Errorf("%s: want %s (%s) on the path, got %v", marker, k, attr, got)
			}
		}
	}
	if got := kindsOn(t, res, "@sink(plain)"); len(got) != 0 {
		t.Errorf("a plain flow carries no uncertainty, got %v", got)
	}
}

func TestGlobalVarNoteListsTheWriters(t *testing.T) {
	res, _ := runCase(t, "uncertain")
	for _, f := range res.Findings {
		if !strings.Contains(f.Sink.Snippet, "@sink(global)") {
			continue
		}
		for _, u := range f.Uncertainties {
			if u.Kind == KindGlobalVar {
				if len(u.Candidates) != 1 || !strings.HasSuffix(u.Candidates[0], "uncertain.SetShared") {
					t.Errorf("writers = %v, want [..uncertain.SetShared]", u.Candidates)
				}
				return
			}
		}
	}
	t.Fatal("global_var note not found")
}

func TestImplementations(t *testing.T) { checkCase(t, "impl") }

func TestImplementationsStopAtTheLimit(t *testing.T) {
	prog := testutil.Load(t, "./cases/impl/...")
	rules := testutil.Rules(t)
	exp := rules.Expand(prog, 1) // the interface has two implementations
	var limited bool
	for _, e := range exp {
		if strings.HasSuffix(e.Symbol, "impl.Store).Save") {
			limited = e.Limited && e.Count == 2
		}
	}
	if !limited {
		t.Fatalf("expected a limited expansion with 2 implementations, got %+v", exp)
	}
	st := sites.Enumerate(prog, rules)
	res := New(prog, st, rules, callgraph.BuildGo(prog), DefaultOptions()).Run()
	// Only the call through the interface itself is still a sink.
	var lines []int
	for _, f := range res.Findings {
		lines = append(lines, f.Sink.Position.Line)
	}
	if len(lines) != 1 {
		t.Errorf("without the expansion only the interface call is a sink, got findings at %v", lines)
	}
}

func TestDispatchWithoutImplementationIsRivusToImprove(t *testing.T) {
	res, _ := runCase(t, "uncertain")
	for _, f := range res.Findings {
		if !strings.Contains(f.Sink.Snippet, "@sink(opaque)") {
			continue
		}
		for _, u := range f.Uncertainties {
			if u.Kind != KindDynamicDispatch {
				continue
			}
			if u.AttributableTo != AttributableRivus || len(u.Candidates) != 0 || !strings.Contains(u.Detail, "no implementation is known") {
				t.Errorf("a call without known implementation should be attributable to rivus: %+v", u)
			}
			return
		}
		t.Fatalf("no dynamic_dispatch on the path: %+v", f.Uncertainties)
	}
	t.Fatal("finding not found")
}

func TestDispatchWithOneCandidateStaysMixed(t *testing.T) {
	res, _ := runCase(t, "inter")
	for _, f := range res.Findings {
		if f.Sink.Snippet != "lib.Sink(s) // @sink(disp)" {
			continue
		}
		for _, u := range f.Uncertainties {
			if u.Kind == KindDynamicDispatch && u.AttributableTo == AttributableMixed && len(u.Candidates) == 1 {
				return
			}
		}
	}
	t.Fatal("a call with one candidate should stay mixed")
}
