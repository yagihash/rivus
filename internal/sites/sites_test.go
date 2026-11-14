package sites

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
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
	return Enumerate(prog, goir.NewTexts(p), rules)
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
