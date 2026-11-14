package sites

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/model"
)

const tmpLib = `package lib

func Source() string { return "" }
func Sink(s string)  {}
`

const tmpRules = `
sources:
  - symbol: example.com/tmp/lib.Source
    labels: [user_input]
sinks:
  - symbol: example.com/tmp/lib.Sink
`

// sitesOf loads a throw-away module whose main package has the given source.
func sitesOf(t *testing.T, src string) []*Site {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/tmp\n\ngo 1.27\n")
	write("lib/lib.go", tmpLib)
	write("main/main.go", src)
	p, err := gossa.Load(dir, []string{"./main"}, gossa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LoadErrors) != 0 {
		t.Fatalf("load errors: %v", p.LoadErrors)
	}
	rs, err := model.Parse(strings.NewReader(tmpRules), "tmp")
	if err != nil {
		t.Fatal(err)
	}
	return Enumerate(p, model.Merge(rs)).Sites
}

func sinks(ss []*Site) []*Site {
	var out []*Site
	for _, s := range ss {
		if s.Role == RoleSink {
			out = append(out, s)
		}
	}
	return out
}

const base = `package main

import "example.com/tmp/lib"

func F() {
	a := lib.Source()
	lib.Sink(a)
	lib.Sink("x")
}
`

func TestStabilityAgainstLineShifts(t *testing.T) {
	before := sinks(sitesOf(t, base))
	shifted := strings.Replace(base, "func F() {", "// a new comment\n\nfunc other() {}\n\nfunc F() {", 1)
	after := sinks(sitesOf(t, shifted))
	if len(before) != 2 || len(after) != 2 {
		t.Fatalf("want 2 sinks, got %d and %d", len(before), len(after))
	}
	for i := range before {
		if before[i].ID != after[i].ID {
			t.Errorf("ID changed by a line shift: %s -> %s", before[i].ID, after[i].ID)
		}
		for _, k := range []string{fingerprint.Structural, fingerprint.Content, fingerprint.Expr} {
			if before[i].Fingerprint[k] != after[i].Fingerprint[k] {
				t.Errorf("%s changed by a line shift", k)
			}
		}
		if before[i].Position.Line == after[i].Position.Line {
			t.Error("test is meaningless: line did not change")
		}
	}
}

func TestEditedArgumentKeepsStructuralChangesContent(t *testing.T) {
	before := sinks(sitesOf(t, base))
	edited := strings.Replace(base, "lib.Sink(a)", "lib.Sink(a + \"!\")", 1)
	after := sinks(sitesOf(t, edited))
	b, a := before[0], after[0]
	if b.ID != a.ID {
		t.Errorf("structural ID should survive an edited argument: %s -> %s", b.ID, a.ID)
	}
	if b.Fingerprint[fingerprint.Content] == a.Fingerprint[fingerprint.Content] {
		t.Error("content key should change when the expression changes")
	}
	if b.Fingerprint[fingerprint.Expr] == a.Fingerprint[fingerprint.Expr] {
		t.Error("expr fingerprint should detect the change")
	}
	if b.Fingerprint[fingerprint.Body] == a.Fingerprint[fingerprint.Body] {
		t.Error("body fingerprint should detect the change")
	}
}

func TestInsertedCallShiftsStructuralButNotContent(t *testing.T) {
	before := sinks(sitesOf(t, base))
	inserted := strings.Replace(base, "\ta := lib.Source()\n", "\tlib.Sink(\"first\")\n\ta := lib.Source()\n", 1)
	after := sinks(sitesOf(t, inserted))
	if len(after) != 3 {
		t.Fatalf("want 3 sinks after the insertion, got %d", len(after))
	}
	// The old first sink (lib.Sink(a)) is now the second one. The structural
	// key follows the ordinal and therefore points at the wrong site, while
	// the content key follows the expression. This is why matching tries
	// the content key first (ADR-028).
	old := before[0]
	if old.Fingerprint[fingerprint.Structural] != after[0].Fingerprint[fingerprint.Structural] {
		t.Error("structural key should have stayed with the ordinal")
	}
	if old.Fingerprint[fingerprint.Content] != after[1].Fingerprint[fingerprint.Content] {
		t.Error("content key should follow the expression to its new position")
	}
	if old.Fingerprint[fingerprint.Content] == after[0].Fingerprint[fingerprint.Content] {
		t.Error("content key must tell the inserted call apart")
	}
}

func TestIdenticalCallsGetDistinctContentKeys(t *testing.T) {
	src := "package main\n\nimport \"example.com/tmp/lib\"\n\nfunc F() {\n\tlib.Sink(\"x\")\n\tlib.Sink(\"x\")\n}\n"
	ss := sinks(sitesOf(t, src))
	if len(ss) != 2 {
		t.Fatalf("want 2 sinks, got %d", len(ss))
	}
	if ss[0].Fingerprint[fingerprint.Content] == ss[1].Fingerprint[fingerprint.Content] {
		t.Error("identical expressions need distinct content keys")
	}
	if ss[0].Fingerprint[fingerprint.Expr] != ss[1].Fingerprint[fingerprint.Expr] {
		t.Error("identical expressions share the expr fingerprint")
	}
}
