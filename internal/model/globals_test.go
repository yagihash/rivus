package model

import (
	"strings"
	"testing"
)

func parseGlobals(t *testing.T, src string) *RuleSet {
	t.Helper()
	rs, err := Parse(strings.NewReader(src), "test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

// A rule says that a package variable holds nothing; a rule of another set says everything, and takes it out.
func TestGlobalsHoldNothing(t *testing.T) {
	builtin := parseGlobals(t, "ruleset: a@1\nglobals:\n  - symbol: os.Stderr\n    holds: nothing\n  - symbol: os.Stdout\n    holds: nothing\n")
	r := Merge(builtin)
	if !r.HoldsNothing("os.Stderr") || !r.HoldsNothing("os.Stdout") || r.HoldsNothing("os.Stdin") {
		t.Errorf("holds nothing: %v", r.GlobalsHoldNothing)
	}
	project := parseGlobals(t, "ruleset: b@1\nglobals:\n  - symbol: os.Stderr\n    holds: everything\n")
	for _, order := range [][]*RuleSet{{builtin, project}, {project, builtin}} {
		m := Merge(order...)
		if m.HoldsNothing("os.Stderr") || !m.HoldsNothing("os.Stdout") {
			t.Errorf("a rule that says everything takes the variable out, in any order: %v", m.GlobalsHoldNothing)
		}
	}
}

func TestGlobalsAreChecked(t *testing.T) {
	for name, src := range map[string]string{
		"no symbol":  "ruleset: a@1\nglobals:\n  - holds: nothing\n",
		"no holds":   "ruleset: a@1\nglobals:\n  - symbol: os.Stderr\n",
		"bad holds":  "ruleset: a@1\nglobals:\n  - symbol: os.Stderr\n    holds: metadata\n",
		"builtin":    "ruleset: a@1\nglobals:\n  - symbol: os.Stderr\n    holds: nothing\n    origin: builtin\n",
		"llm no why": "ruleset: a@1\nglobals:\n  - symbol: os.Stderr\n    holds: nothing\n    origin: llm\n",
	} {
		if _, err := Parse(strings.NewReader(src), "test.yaml"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
