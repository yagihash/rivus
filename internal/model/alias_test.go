package model

import (
	"strings"
	"testing"
)

func parseSet(t *testing.T, text string) (*RuleSet, error) {
	t.Helper()
	return Parse(strings.NewReader(text), "test.yaml")
}

// A function has one argument that its result is the same storage as. A model that says two,
// or two models of one symbol that say one each, would have one of them win without a word.
func TestAliasIsOnePerFunction(t *testing.T) {
	_, err := parseSet(t, `
ruleset: t@1
propagators:
  - symbol: f
    flows:
      - {from: arg0, to: result, alias: true}
      - {from: arg1, to: result, alias: true}
`)
	if err == nil || !strings.Contains(err.Error(), "at most one flow with alias") {
		t.Errorf("two alias flows in one model: %v", err)
	}

	_, err = parseSet(t, `
ruleset: t@1
propagators:
  - symbol: f
    flows: [{from: arg0, to: result, alias: true}]
  - symbol: f
    flows: [{from: arg1, to: result, alias: true}]
`)
	if err == nil || !strings.Contains(err.Error(), "has another model with an alias flow") {
		t.Errorf("two models of one symbol in one set: %v", err)
	}

	// what one set says does not clash with another set's other symbols, and an alias beside plain flows is fine
	a, err := parseSet(t, "ruleset: a@1\npropagators:\n  - symbol: f\n    flows: [{from: arg0, to: result, alias: true}, {from: args, to: result}]\n")
	if err != nil {
		t.Fatal(err)
	}
	b, err := parseSet(t, "ruleset: b@1\npropagators:\n  - symbol: g\n    flows: [{from: arg0, to: result, alias: true}]\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := Merge(a, b).Validate(); err != nil {
		t.Errorf("different symbols: %v", err)
	}

	// the same symbol in two sets (the built-in models and a project's): only the merged rules show it
	c, err := parseSet(t, "ruleset: c@1\npropagators:\n  - symbol: f\n    flows: [{from: arg1, to: result, alias: true}]\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := Merge(a, c).Validate(); err == nil || !strings.Contains(err.Error(), "more than one model has an alias flow") {
		t.Errorf("one symbol in two sets: %v", err)
	}
}
