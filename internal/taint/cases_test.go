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
	prog := loadIR(t, testutil.Testdata(), "./cases/"+name+"/...")
	rules := testutil.Rules(t)
	rules.Expand(prog.gossa, 0)
	st := prog.sites(rules)
	eng := prog.engine(st, rules, DefaultOptions())
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
	prog := loadIR(t, testutil.Testdata(), "./cases/inter/...")
	rules := testutil.Rules(t)
	st := prog.sites(rules)
	opts := DefaultOptions()
	opts.Budget = 1 // far too small for any real function
	res := prog.engine(st, rules, opts).Run()

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
	prog := loadIR(t, testutil.Testdata(), "./cases/inter/...")
	rules := testutil.Rules(t)
	st := prog.sites(rules)
	opts := DefaultOptions()
	opts.Timeout = time.Nanosecond
	res := prog.engine(st, rules, opts).Run()
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

func TestFunctionValues(t *testing.T) { checkCase(t, "funcvalue") }

func TestImplementationsStopAtTheLimit(t *testing.T) {
	prog := loadIR(t, testutil.Testdata(), "./cases/impl/...")
	rules := testutil.Rules(t)
	exp := rules.Expand(prog.gossa, 1) // the interface has two implementations
	var limited bool
	for _, e := range exp {
		if strings.HasSuffix(e.Symbol, "impl.Store).Save") {
			limited = e.Limited && e.Count == 2
		}
	}
	if !limited {
		t.Fatalf("expected a limited expansion with 2 implementations, got %+v", exp)
	}
	st := prog.sites(rules)
	res := prog.engine(st, rules, DefaultOptions()).Run()
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

func TestEntryPoints(t *testing.T) { checkCase(t, "entry") }

func TestGuard(t *testing.T) { checkCase(t, "guard") }

func findingWithSink(t *testing.T, res *Result, marker string) *Finding {
	t.Helper()
	for _, f := range res.Findings {
		if strings.Contains(f.Sink.Snippet, marker) {
			return f
		}
	}
	t.Fatalf("no finding for %q", marker)
	return nil
}

func TestConditionalNodeForAValueDependentSanitizer(t *testing.T) {
	res, _ := runCase(t, "guard")
	f := findingWithSink(t, res, "@sink(dyn)")
	if len(f.ConditionalNodes) != 1 {
		t.Fatalf("want 1 conditional node, got %+v", f.ConditionalNodes)
	}
	n := f.ConditionalNodes[0]
	if n.Kind != CondSanitization || n.BranchCondition != "escape" || !strings.HasSuffix(n.Function, "guard.render") {
		t.Errorf("node = %+v", n)
	}
	if n.DerivedFrom != "parameter 1 (escape bool)" {
		t.Errorf("derived_from = %q", n.DerivedFrom)
	}
	if got := n.Outcomes["escape == true"]; got != "sanitized by example.com/cases/lib.Sanitize" {
		t.Errorf("true outcome = %q", got)
	}
	if got := n.Outcomes["escape == false"]; got != "tainted (passthrough)" {
		t.Errorf("false outcome = %q", got)
	}
	if n.CallSiteArgument != "render(s, flag)" || n.Argument != "flag" || n.Resolved {
		t.Errorf("call site: %q argument %q resolved %v", n.CallSiteArgument, n.Argument, n.Resolved)
	}
	if !strings.Contains(n.FunctionSource, "func render(") {
		t.Errorf("function source = %q", n.FunctionSource)
	}
}

func TestConditionalNodeWithAConstantArgumentIsResolved(t *testing.T) {
	res, _ := runCase(t, "guard")
	f := findingWithSink(t, res, "@sink(const)")
	if len(f.ConditionalNodes) != 1 || !f.ConditionalNodes[0].Resolved || f.ConditionalNodes[0].Argument != "false" {
		t.Fatalf("nodes = %+v", f.ConditionalNodes)
	}
}

func TestEarlyReturnIsNotAConditionalNode(t *testing.T) {
	res, _ := runCase(t, "guard")
	if f := findingWithSink(t, res, "@sink(early)"); len(f.ConditionalNodes) != 0 {
		t.Errorf("a condition with a flow on one side only says nothing: %+v", f.ConditionalNodes)
	}
}

func guardLimitFunctions(res *Result) map[string]*Uncertainty {
	out := map[string]*Uncertainty{}
	for _, u := range res.Uncertainties {
		if u.Kind == KindDepthLimit && strings.HasPrefix(u.Detail, "guard_limit") {
			out[u.Func] = u
		}
	}
	return out
}

func TestTooManyGuardsAreDroppedAndSaidWhereTheyWouldMatter(t *testing.T) {
	res, _ := runCase(t, "guard")
	for _, marker := range []string{"@sink(deep)", "@sink(deepsan)"} {
		if f := findingWithSink(t, res, marker); len(f.ConditionalNodes) != 0 {
			t.Errorf("%s: guards beyond the limit must be dropped: %+v", marker, f.ConditionalNodes)
		}
	}
	limited := guardLimitFunctions(res)
	var sanitizing *Uncertainty
	for fn, u := range limited {
		if strings.HasSuffix(fn, "guard.deepSanitized") {
			sanitizing = u
		}
		if strings.HasSuffix(fn, "guard.deep") {
			t.Errorf("a function that sanitizes nothing loses nothing by dropping guards: %+v", u)
		}
	}
	if sanitizing == nil {
		t.Fatalf("the function that sanitizes and exceeded the guard limit must be recorded: %v", limited)
	}
	if sanitizing.AttributableTo != AttributableRivus {
		t.Errorf("guard_limit is rivus' to improve: %+v", sanitizing)
	}
}

func provenanceOf(t *testing.T, marker string) *Provenance {
	t.Helper()
	res, _ := runCase(t, "guard")
	f := findingWithSink(t, res, marker)
	if len(f.ConditionalNodes) != 1 || f.ConditionalNodes[0].Provenance == nil {
		t.Fatalf("%s: nodes = %+v", marker, f.ConditionalNodes)
	}
	return f.ConditionalNodes[0].Provenance
}

func terminalKinds(p *Provenance) map[string]string {
	out := map[string]string{}
	for _, u := range p.Unresolved {
		out[u.TerminalKind] = u.AttributableTo
	}
	return out
}

func TestProvenanceStopsAtAPackageVariable(t *testing.T) {
	p := provenanceOf(t, "@sink(pv)")
	if terminalKinds(p)[KindGlobalVar] != AttributableTarget || len(p.Resolved) != 0 {
		t.Errorf("provenance = %+v", p)
	}
}

func TestProvenanceStopsAtConfiguration(t *testing.T) {
	p := provenanceOf(t, "@sink(env)")
	if terminalKinds(p)[KindConfigRead] != AttributableTarget {
		t.Errorf("provenance = %+v", p)
	}
	for _, u := range p.Unresolved {
		if u.TerminalKind == KindConfigRead && u.Detail == "" {
			t.Errorf("the configuration read must say which function: %+v", u)
		}
	}
	// The "1" compared with is not where the value comes from.
	if len(p.Resolved) != 0 {
		t.Errorf("a compared constant was reported as the value: %+v", p.Resolved)
	}
}

func TestProvenanceFollowsAParameterToItsCallers(t *testing.T) {
	p := provenanceOf(t, "@sink(wrap)")
	if len(p.Resolved) != 1 || p.Resolved[0].Value != "false" || p.Resolved[0].Certainty != "literal" || len(p.Unresolved) != 0 {
		t.Fatalf("provenance = %+v", p)
	}
	if p.Resolved[0].CallSite.Line == 0 {
		t.Error("the resolved value must say where it was passed")
	}
	if len(p.Argument) == 0 {
		t.Error("the argument must be named")
	}
}

func TestProvenanceFollowsAFieldToWhereItIsSet(t *testing.T) {
	p := provenanceOf(t, "@sink(sf)")
	if len(p.Resolved) != 1 || p.Resolved[0].Value != "false" || p.Resolved[0].Certainty != CertaintyZeroValue {
		t.Errorf("provenance = %+v", p)
	}
}

func TestProvenanceOfAnExportedFunctionsParameter(t *testing.T) {
	// Dynamic is exported and nothing in the code calls it: its argument
	// comes from outside.
	p := provenanceOf(t, "@sink(dyn)")
	if terminalKinds(p)[KindExportedAPIParam] != AttributableTarget {
		t.Errorf("provenance = %+v", p)
	}
}

func TestDBSources(t *testing.T) { checkCase(t, "dbsrc") }

// A call marked as a boundary of the data store is an uncertainty of its own
// kind, at the place of the call, even though nothing flows through it.
func TestStoredDataBoundaryIsRecorded(t *testing.T) {
	prog := loadIR(t, testutil.Testdata(), "./cases/dbsrc")
	rules := testutil.Rules(t)
	st := prog.sites(rules)
	res := prog.engine(st, rules, DefaultOptions()).Run()
	for _, u := range res.Uncertainties {
		if u.Kind == KindStoredDataBoundary {
			if u.AttributableTo != AttributableTarget || u.Symbol != "example.com/cases/lib.StoreRead" {
				t.Errorf("unexpected boundary note: %+v", u)
			}
			return
		}
	}
	t.Fatal("no stored_data_boundary uncertainty was recorded")
}

// A call through an interface is a call to every implementation, unless the
// caller made the interface value right there: then only the implementation of
// its type is called.
func TestDispatchIsNarrowedByTheTypeTheCallerPassed(t *testing.T) { checkCase(t, "dispatch") }

// gRPC: the metadata of a call and the messages of a stream are what a client
// sent; a connection to an address from outside is a request forgery.
func TestGRPCServerModels(t *testing.T) { checkCase(t, "grpcsrv") }

// The handlers of a gRPC service are found from the generated code (an
// interface with a mustEmbedUnimplemented method): what they are given as
// messages is input; a method of another interface is not.
func TestGRPCHandlersAreFoundFromTheGeneratedCode(t *testing.T) { checkCase(t, "grpcunary") }
