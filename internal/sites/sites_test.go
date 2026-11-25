package sites

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/testutil"
)

func enumerate(t *testing.T, pattern string) *Result {
	t.Helper()
	return enumerateGo(t, testutil.Testdata(), []string{pattern}, testutil.Rules(t))
}

// enumerateGo reads Go code into the neutral IR and enumerates its sites.
func enumerateGo(t testing.TB, dir string, patterns []string, rules *model.Rules) *Result {
	t.Helper()
	p, err := gossa.Load(dir, patterns, gossa.Options{NaiveForm: true})
	if err != nil {
		t.Fatal(err)
	}
	prog, err := goir.Extract(p, goir.Options{NoCandidates: true})
	if err != nil {
		t.Fatal(err)
	}
	return Enumerate(ir.MemSource(prog), goir.NewTexts(p), rules)
}

func find(res *Result, fn, role, symbol string, ordinal int) *Site {
	n := 0
	for _, s := range res.Sites {
		if strings.HasSuffix(s.Func, fn) && s.Role == role && s.Symbol == symbol {
			n++
			if n == ordinal {
				return s
			}
		}
	}
	return nil
}

func TestEnumerateRolesAndLiteral(t *testing.T) {
	res := enumerate(t, "./cases/sites")

	lit := find(res, "sites.Literal", RoleSink, "example.com/cases/lib.Sink", 1)
	if lit == nil || lit.LiteralOnly == nil || !*lit.LiteralOnly {
		t.Fatalf("literal sink should be marked literal_only: %+v", lit)
	}
	dyn := find(res, "sites.Dynamic", RoleSink, "example.com/cases/lib.Sink", 1)
	if dyn == nil || dyn.LiteralOnly == nil || *dyn.LiteralOnly {
		t.Fatalf("dynamic sink must not be literal_only: %+v", dyn)
	}
	if src := find(res, "sites.Dynamic", RoleSource, "example.com/cases/lib.Source", 1); src == nil {
		t.Fatal("source site missing")
	} else if src.LiteralOnly != nil {
		t.Error("literal_only applies to sinks only")
	}

	// exec.Command(name string, arg ...string): variadic constants count.
	e1 := find(res, "sites.Exec", RoleSink, "os/exec.Command", 1)
	e2 := find(res, "sites.Exec", RoleSink, "os/exec.Command", 2)
	e3 := find(res, "sites.Exec", RoleSink, "os/exec.Command", 3)
	if e1 == nil || e2 == nil || e3 == nil {
		t.Fatal("exec sinks missing")
	}
	if !*e1.LiteralOnly {
		t.Error("exec.Command(\"ls\", \"-l\") is literal only")
	}
	if *e2.LiteralOnly {
		t.Error("exec.Command(name, ...) is not literal only")
	}
	if *e3.LiteralOnly {
		t.Error("exec.Command(\"ls\", name) is not literal only")
	}

	// A parameter of type *http.Request is a source.
	req := find(res, "sites.Handler", RoleSource, "*net/http.Request", 1)
	if req == nil || req.Kind != "param_type" {
		t.Fatalf("request parameter source missing: %+v", req)
	}
}

func TestSnippetAndPosition(t *testing.T) {
	res := enumerate(t, "./cases/sites")
	s := find(res, "sites.Dynamic", RoleSink, "example.com/cases/lib.Sink", 1)
	if s.Position.File != "cases/sites/sites.go" || s.Position.Line != 16 {
		t.Errorf("position = %+v", s.Position)
	}
	if !strings.Contains(s.Snippet, "lib.Sink(s)") {
		t.Errorf("snippet = %q", s.Snippet)
	}
}

func TestIDsAreUniqueAndFingerprinted(t *testing.T) {
	res := enumerate(t, "./cases/sites")
	seen := map[string]bool{}
	for _, s := range res.Sites {
		if seen[s.ID] {
			t.Errorf("duplicate ID %s", s.ID)
		}
		seen[s.ID] = true
		if !strings.HasPrefix(s.ID, fingerprint.IDPrefix) {
			t.Errorf("ID %q lacks the prefix", s.ID)
		}
		for _, k := range []string{fingerprint.Structural, fingerprint.Content, fingerprint.Expr, fingerprint.Body} {
			if s.Fingerprint[k] == "" {
				t.Errorf("%s: fingerprint %s missing", s.ID, k)
			}
		}
	}
	// Two sinks to the same symbol in one function get different IDs.
	a := find(res, "sites.TwoSinks", RoleSink, "example.com/cases/lib.Sink", 1)
	b := find(res, "sites.TwoSinks", RoleSink, "example.com/cases/lib.Sink", 2)
	if a == nil || b == nil || a.ID == b.ID {
		t.Fatalf("ordinals do not distinguish the sites: %+v %+v", a, b)
	}
}

func TestEnumerateIsDeterministic(t *testing.T) {
	a := enumerate(t, "./cases/sites")
	b := enumerate(t, "./cases/sites")
	if len(a.Sites) != len(b.Sites) {
		t.Fatal("site count differs")
	}
	for i := range a.Sites {
		if a.Sites[i].ID != b.Sites[i].ID || a.Sites[i].Fingerprint[fingerprint.Content] != b.Sites[i].Fingerprint[fingerprint.Content] {
			t.Fatalf("site %d differs between runs", i)
		}
	}
}

// A call of a function value is a site for the function it may reach (yagihash/rivus #54), with
// or without the call graph of the extractor: the functions that flow to the value are found in the IR.
func TestEnumerateCallsOfFunctionValues(t *testing.T) {
	res := enumerate(t, "./cases/funcsink")
	for _, fn := range []string{"funcsink.Alias", "funcsink.Phi", "funcsink.runRule"} {
		if s := find(res, fn, RoleSink, "example.com/cases/lib.Sink", 1); s == nil {
			t.Errorf("%s: no sink site for the function the value holds", fn)
		} else if s.Callee != "example.com/cases/lib.Sink" {
			t.Errorf("%s: callee %q", fn, s.Callee)
		}
	}
	if s := find(res, "funcsink.PhiNone", RoleSink, "example.com/cases/lib.Sink", 1); s != nil {
		t.Errorf("a value that cannot hold a sink is not a site: %+v", s)
	}
	// the sites are the same when the call graph is there
	p, err := goir.Load(testutil.Testdata(), []string{"./cases/funcsink"}, gossa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	prog, err := goir.Extract(p, goir.Options{})
	if err != nil {
		t.Fatal(err)
	}
	with := Enumerate(ir.MemSource(prog), goir.NewTexts(p), testutil.Rules(t))
	for _, fn := range []string{"funcsink.Alias", "funcsink.Phi", "funcsink.runRule"} {
		a, b := find(res, fn, RoleSink, "example.com/cases/lib.Sink", 1), find(with, fn, RoleSink, "example.com/cases/lib.Sink", 1)
		if a == nil || b == nil || a.ID != b.ID {
			t.Errorf("%s: %+v / %+v", fn, a, b)
		}
	}
}

func givenSinkResult(t *testing.T) *Result {
	t.Helper()
	rules := callbackRules(t, `
sinks:
  - symbol: example.com/cases/cases/givensink.Sink
    args: [0]
    category: command_injection
propagators:
  - symbol: slices.IndexFunc
    pure: true
    calls: [{fn: arg1, with: [arg0]}]
  - symbol: slices.ContainsFunc
    pure: true
    calls: [{fn: arg1, with: [arg0]}]
`)
	return enumerateGo(t, testutil.Testdata(), []string{"./cases/givensink"}, rules)
}

func TestSinkOfFunctionGivenToACallRow(t *testing.T) {
	res := givenSinkResult(t)
	const sink = "example.com/cases/cases/givensink.Sink"
	for _, fn := range []string{"givensink.Direct", "givensink.Variable"} {
		s := find(res, fn, RoleSink, sink, 1)
		if s == nil || s.GivenWith == nil || len(s.GivenWith) != 1 || s.GivenWith[0] != 0 {
			t.Fatalf("%s: the call that gives the sink has no site that reads its operand 0: %+v", fn, s)
		}
		if s.Callee != sink {
			t.Errorf("%s: callee = %q", fn, s.Callee)
		}
	}
	if s := find(res, "givensink.Closure$1", RoleSink, sink, 1); s == nil || s.GivenWith != nil {
		t.Errorf("the call in the closure is a sink of its own: %+v", s)
	}
	if s := find(res, "givensink.Closure", RoleSink, sink, 1); s != nil {
		t.Errorf("a closure gives no sink to the call of the library: %+v", s)
	}
	if s := find(res, "givensink.NotSink", RoleSink, sink, 1); s != nil {
		t.Errorf("a function that is no sink has no site: %+v", s)
	}
}

func TestSinkOfFunctionGivenIsCountedApart(t *testing.T) {
	res := givenSinkResult(t)
	const sink = "example.com/cases/cases/givensink.Sink"
	var direct, given *Site
	for _, s := range res.Sites {
		if strings.HasSuffix(s.Func, "givensink.Mixed") && s.Role == RoleSink && s.Symbol == sink {
			if s.GivenWith != nil {
				given = s
			} else {
				direct = s
			}
		}
	}
	if direct == nil || given == nil || direct.ID == given.ID {
		t.Fatalf("Mixed has the sink of the call and the sink of the call that gives it, with ids of their own: %+v %+v", direct, given)
	}
	// without the rows, the call has the one site it had: its id is the same
	plain := callbackRules(t, `
sinks:
  - symbol: example.com/cases/cases/givensink.Sink
    args: [0]
    category: command_injection
`)
	before := find(enumerateGo(t, testutil.Testdata(), []string{"./cases/givensink"}, plain), "givensink.Mixed", RoleSink, sink, 1)
	if before == nil || before.ID != direct.ID {
		t.Errorf("the id of the call of the sink moved: %+v, now %s", before, direct.ID)
	}
}
