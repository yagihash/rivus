package model

import (
	"strings"
	"testing"
)

func parseRejected(body string) (*RuleSet, error) {
	return Parse(strings.NewReader("ruleset: t@1\nrejected:\n"+body), "t.yaml")
}

func TestRejectedIsReadAndNotUsed(t *testing.T) {
	rs, err := parseRejected(`  - symbol: github.com/jmoiron/sqlx.LoadFile
    kind: sink
    why: "the argument is a path"
  - symbol: (*github.com/jmoiron/sqlx.NamedStmt).Exec
    kind: sink
    position: arg0
    why: "the query was fixed when it was prepared"
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rejected) != 2 || rs.Rejected[1].Position != "arg0" || rs.Rejected[0].Kind != "sink" {
		t.Fatalf("%+v", rs.Rejected)
	}
	// it is a record: no rule comes out of it
	r := Merge(rs)
	if r.HasModel("github.com/jmoiron/sqlx.LoadFile") || len(r.Symbols()) != 0 {
		t.Fatalf("a rejected candidate became a rule: %v", r.Symbols())
	}
}

func TestRejectedIsChecked(t *testing.T) {
	for name, c := range map[string]struct{ body, want string }{
		"no symbol":    {"  - kind: sink\n    why: x\n", "symbol is required"},
		"no kind":      {"  - symbol: a.B\n    why: x\n", "kind is required"},
		"unknown kind": {"  - symbol: a.B\n    kind: propagator\n    why: x\n", "unknown kind"},
		"no why":       {"  - symbol: a.B\n    kind: sink\n", "why is required"},
		"blank why":    {"  - symbol: a.B\n    kind: sink\n    why: \"  \"\n", "why is required"},
		"twice":        {"  - symbol: a.B\n    kind: sink\n    why: x\n  - symbol: a.B\n    kind: sink\n    why: y\n", "twice"},
		"unknown key":  {"  - symbol: a.B\n    kind: sink\n    why: x\n    note: y\n", "field note not found"},
	} {
		if _, err := parseRejected(c.body); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want %q, got %v", name, c.want, err)
		}
	}
	// the same symbol with another kind or position is a different entry; an empty list is fine
	if _, err := parseRejected("  - symbol: a.B\n    kind: sink\n    why: x\n  - symbol: a.B\n    kind: source\n    why: y\n  - symbol: a.B\n    kind: sink\n    position: arg1\n    why: z\n"); err != nil {
		t.Error(err)
	}
	if _, err := Parse(strings.NewReader("ruleset: t@1\nrejected: []\n"), "t.yaml"); err != nil {
		t.Error(err)
	}
}

func TestDeferred(t *testing.T) {
	ok := "ruleset: t@1\ndeferred:\n  - symbol: a.F\n    kind: sink\n    why: w\n    ref: model-x-y\n  - symbol: a.G\n    kind: sink\n    why: w\n    ref: \"#12\"\n"
	rs, err := Parse(strings.NewReader(ok), "t.yaml")
	if err != nil || len(rs.Deferred) != 2 || len(Merge(rs).Symbols()) != 0 {
		t.Fatalf("deferred = %v, %v", rs, err)
	}
	for name, body := range map[string]string{
		"no ref":  "  - symbol: a.F\n    kind: sink\n    why: w\n",
		"bad ref": "  - symbol: a.F\n    kind: sink\n    why: w\n    ref: later\n",
		"no why":  "  - symbol: a.F\n    kind: sink\n    ref: \"#1\"\n",
		"no kind": "  - symbol: a.F\n    why: w\n    ref: \"#1\"\n",
		"twice":   "  - symbol: a.F\n    kind: sink\n    why: w\n    ref: \"#1\"\n  - symbol: a.F\n    kind: sink\n    why: w\n    ref: \"#1\"\n",
	} {
		if _, err := Parse(strings.NewReader("ruleset: t@1\ndeferred:\n"+body), "t.yaml"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
