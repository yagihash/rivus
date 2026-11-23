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

// The rules apply to the function a call of a function value reaches: yagihash/rivus #54.
func TestFunctionValueSinks(t *testing.T) { checkCase(t, "funcsink") }

// A call through an interface of the project reaches the implementation that the rules name (ADR-068).
func TestSinkBehindOwnInterface(t *testing.T) { checkCase(t, "ownif") }
func TestGenerics(t *testing.T)               { checkCase(t, "generics") }

// when_arg_type counts the arguments of a method without the receiver, and recv: true is the receiver
// (yagihash/rivus #56).
func TestWhenArgTypeOfAMethod(t *testing.T) { checkCase(t, "recvcond") }

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

func TestTypeAliasInRules(t *testing.T) { checkCase(t, "typealias") }

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
	if len(p.Resolved) != 1 || p.Resolved[0].Value != "false" || p.Resolved[0].Certainty != CertaintyLiteral {
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

// A function with no body in the code, that has a sink rule and no rule of the
// flow, is not modelled by the sink: what goes in comes out, and the call is an
// unmodelled library call (CORE-K).
func TestSinkOnlyRuleDoesNotMakeTheCallAModel(t *testing.T) {
	checkCase(t, "sinkret")
	res, _ := runCase(t, "sinkret")
	got := map[string]bool{}
	for _, u := range res.Uncertainties {
		if u.Kind == KindUnmodeledLibraryCall {
			got[u.Symbol] = true
		}
	}
	for _, sym := range []string{"example.com/cases/lib.Open", "(*example.com/cases/lib.DB).Prepare"} {
		if !got[sym] {
			t.Errorf("no unmodeled_library_call for %s", sym)
		}
	}
}

// A flow that the model marks approximate is followed, and the finding through it carries one uncertainty of kind
// model_approximation with the reason of the model: it is not a path without any uncertainty (C-8). A flow that is
// not marked leaves none.
func TestApproximateFlowLeavesAnUncertaintyOnThePath(t *testing.T) {
	checkCase(t, "approx")
	res, _ := runCase(t, "approx")
	for _, f := range res.Findings {
		var n int
		for _, u := range f.Uncertainties {
			if u.Kind == KindModelApproximation {
				n++
				if !strings.Contains(u.Detail, "external program") || u.Symbol != "example.com/cases/lib.External" {
					t.Errorf("the uncertainty does not say why: %+v", u)
				}
			}
		}
		switch f.Source.Position.Line {
		case 8, 9, 10:
			if n != 1 || f.Confidence == ConfidenceClear {
				t.Errorf("the approximate flow: %d model_approximation, confidence %s", n, f.Confidence)
			}
		default:
			if n != 0 {
				t.Errorf("a flow that is not approximate has %d model_approximation: line %d", n, f.Source.Position.Line)
			}
		}
	}
}

// A function of a library that calls the function it is given (a model with calls) is a call of it: what it is
// called with is what the model says, what it returns goes where the model says, and nothing is assumed (no
// assumed_call) where the function is told. Where it is not (a parameter), the flows the row stands for are used,
// and the finding through them says assumed_call.
func TestCallbackRowsApplyTheFunctionThatWasGiven(t *testing.T) {
	checkCase(t, "callbacks")
	res, _ := runCase(t, "callbacks")
	for _, f := range res.Findings {
		n := 0
		for _, u := range f.Uncertainties {
			if u.Kind == KindAssumedCall {
				n++
			}
		}
		want := 0
		if f.Source.Position.Line == 53 {
			want = 1
		}
		if n != want {
			t.Errorf("line %d: %d assumed_call, want %d", f.Source.Position.Line, n, want)
		}
	}
}

// A function without a body and without a model, given a closure, is taken to call it: what it returns reaches
// the result, and the finding through it says assumed_call, naming the function (ADR-116).
func TestUnmodeledCallAssumesTheResultOfTheFunctionGiven(t *testing.T) {
	checkCase(t, "nomodelcb")
	res, _ := runCase(t, "nomodelcb")
	if len(res.Findings) != 1 {
		t.Fatalf("%d findings, want 1", len(res.Findings))
	}
	named := false
	for _, u := range res.Findings[0].Uncertainties {
		if u.Kind == KindAssumedCall && strings.Contains(u.Detail, "lib.Thunk") && strings.Contains(u.Detail, "Captured$1") {
			named = true
		}
	}
	if !named {
		t.Errorf("the finding has no assumed_call that names lib.Thunk and the closure: %+v", res.Findings[0].Uncertainties)
	}
}

// A call through an interface is a call to every implementation, unless the
// caller made the interface value right there: then only the implementation of
// its type is called.
func TestDispatchIsNarrowedByTheTypeTheCallerPassed(t *testing.T) { checkCase(t, "dispatch") }

// What is written to a writer is its content, a step below the writer: a field of the writer is not what was written (ADR-082).
func TestWriterContentIsNotTheWritersFields(t *testing.T) { checkCase(t, "writer") }

// A sanitizer of an interface method says the result is clean; the implementations of the project are read, with their result dropped (ADR-086).
func TestSanitizerOfAnInterfaceReadsTheImplementationsWithoutTheirResult(t *testing.T) {
	checkCase(t, "sanitizeown")
}

// gRPC: the metadata of a call and the messages of a stream are what a client
// sent; a connection to an address from outside is a request forgery.
func TestGRPCServerModels(t *testing.T) { checkCase(t, "grpcsrv") }

// The handlers of a gRPC service are found from the generated code (an
// interface with a mustEmbedUnimplemented method): what they are given as
// messages is input; a method of another interface is not.
func TestGRPCHandlersAreFoundFromTheGeneratedCode(t *testing.T) { checkCase(t, "grpcunary") }

// A program that starts a gRPC server, with no generated code to find the
// handlers in, is told so; one that has the generated code, or does not serve,
// is not.
func TestMissingGeneratedCodeIsReported(t *testing.T) {
	missing := func(name string) bool {
		prog := loadIR(t, testutil.Testdata(), "./cases/"+name+"/...")
		rules := testutil.Rules(t)
		for _, e := range rules.Expand(prog.gossa, 0) {
			if e.Missing {
				return true
			}
		}
		return false
	}
	if !missing("grpcnogen") {
		t.Error("a server without generated code was not reported")
	}
	if missing("grpcunary") {
		t.Error("a program with generated code was reported")
	}
	if missing("grpcsrv") {
		t.Error("a program with generated code was reported")
	}
}

// An object handed down a recursion: the tracing of its field ends at the
// limits instead of overflowing the stack, and says it was cut.
func TestProvenanceThroughARecursiveObjectIsBounded(t *testing.T) {
	p := provenanceOf(t, "@sink(recobj)")
	if _, ok := terminalKinds(p)[KindDepthLimit]; !ok {
		t.Errorf("the cut must be recorded as %s: %+v", KindDepthLimit, p.Unresolved)
	}
}

// A value is told only when it is a constant or a plain copy of one. An
// operation on the way gives the folded value or nothing, and a field that is
// assigned as a whole is not the zero value.
func TestProvenanceTellsNoValueThatIsNotKnown(t *testing.T) {
	type val struct{ value, certainty string }
	for _, tc := range []struct {
		marker string
		want   []val // nil: nothing resolved, and something said unresolved
	}{
		{"@sink(negfield)", []val{{"false", CertaintyLiteral}}},
		{"@sink(arith)", []val{{"0", CertaintyLiteral}}},
		{"@sink(whole)", nil},
		{"@sink(wholeptr)", nil},
		{"@sink(wholeelem)", nil},
		{"@sink(innerpart)", nil},
		{"@sink(truezero)", []val{{"false", CertaintyZeroValue}}},
		{"@sink(throughptr)", []val{{"true", CertaintyLiteral}}},
		{"@sink(ident)", []val{{"true", CertaintyLiteral}}},
		{"@sink(addr)", nil},
		{"@sink(cmp)", []val{{"true", CertaintyLiteral}}},
		{"@sink(readfirst)", []val{{"0", CertaintyZeroValue}}},
		{"@sink(onebranch)", []val{{"9", CertaintyLiteral}, {"0", CertaintyZeroValue}}},
		{"@sink(bothbranch)", []val{{"9", CertaintyLiteral}, {"0", CertaintyZeroValue}}}, // no single write dominates: the zero value stays a candidate
		{"@sink(loopstep)", []val{{"1", CertaintyLiteral}}},                              // the first value only, and the rest said unresolved
	} {
		p := provenanceOf(t, tc.marker)
		var got []val
		for _, r := range p.Resolved {
			got = append(got, val{r.Value, r.Certainty})
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s: resolved %v, want %v (%+v)", tc.marker, got, tc.want, p)
		}
		if (tc.want == nil || tc.marker == "@sink(loopstep)") && len(p.Unresolved) == 0 {
			t.Errorf("%s: nothing resolved and nothing unresolved: %+v", tc.marker, p)
		}
	}
}

// The finding at a sink behind an interface of the project says "one of N implementations" (ADR-068).
func TestSinkBehindOwnInterfaceKeepsTheDispatchUncertainty(t *testing.T) {
	res, _ := runCase(t, "ownif")
	found := 0
	for _, f := range res.Findings {
		two := strings.Contains(f.Sink.Snippet, "ownif_two")
		if !two && !strings.Contains(f.Sink.Snippet, "ownif_direct") {
			continue
		}
		found++
		var d *Uncertainty
		for _, u := range f.Uncertainties {
			if u.Kind == KindDynamicDispatch {
				d = u
			}
		}
		if d == nil {
			t.Errorf("%s: no dynamic_dispatch uncertainty: %+v", f.Sink.Snippet, f.Uncertainties)
			continue
		}
		if two && len(d.Candidates) < 2 {
			t.Errorf("candidates = %v, want both implementations", d.Candidates)
		}
	}
	if found != 2 {
		t.Errorf("found %d findings, want 2", found)
	}
}

// A built-in row and a project row of another category that read the same positions are one sink: the finding keeps the
// first category and lists both in categories (ownif: the rules have category test beside sql_injection).
func TestFindingListsTheCategoriesOfRowsWithTheSamePositions(t *testing.T) {
	res, _ := runCase(t, "ownif")
	n := 0
	for _, f := range res.Findings {
		if !strings.Contains(f.Sink.Snippet, "@sink(ownif_direct)") {
			continue
		}
		n++
		if len(f.Categories) != 2 || f.Categories[0] != f.Category || f.Categories[0] == f.Categories[1] {
			t.Errorf("category %q, categories %v: want the first one and both listed", f.Category, f.Categories)
		}
	}
	if n != 1 {
		t.Errorf("%d findings, want 1 (the rows read the same positions)", n)
	}
}
