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

// `builtin` is the origin of the rules that rivus embeds: a file that comes from outside cannot claim it
// (yagihash/rivus #57).
func TestRuleFilesCannotClaimTheBuiltinOrigin(t *testing.T) {
	for name, text := range map[string]string{
		"the set": "ruleset: t@1\norigin: builtin\nsanitizers:\n  - symbol: f\n    drops: [\"*\"]\n",
		"a rule":  "ruleset: t@1\nsanitizers:\n  - symbol: f\n    drops: [\"*\"]\n    origin: builtin\n",
	} {
		if _, err := parseSet(t, text); err == nil || !strings.Contains(err.Error(), "origin: builtin is for the rules that rivus embeds") {
			t.Errorf("%s: %v", name, err)
		}
	}
	// the other origins are theirs to say, and so is nothing
	for _, text := range []string{
		"ruleset: t@1\norigin: human\nsanitizers:\n  - symbol: f\n    drops: [\"*\"]\n",
		"ruleset: t@1\nsanitizers:\n  - symbol: f\n    drops: [\"*\"]\n    origin: human\n",
		"ruleset: t@1\nsanitizers:\n  - symbol: f\n    drops: [\"*\"]\n",
	} {
		if _, err := parseSet(t, text); err != nil {
			t.Errorf("%v\n%s", err, text)
		}
	}
	// what rivus embeds is read under a name that says so, and may say it
	if _, err := Parse(strings.NewReader("ruleset: t@1\norigin: builtin\nsanitizers:\n  - symbol: f\n    drops: [\"*\"]\n"), "builtin:t.yaml"); err != nil {
		t.Errorf("an embedded rule set cannot say builtin: %v", err)
	}
}

func TestRedactValuesHidesWhatTheDecoderQuotes(t *testing.T) {
	_, err := parseSet(t, "root:x:0:0:root:/root:/bin/sh\n")
	if err == nil {
		t.Fatal("not a rule file, and no error")
	}
	if strings.Contains(RedactValues(err).Error(), "root:x") {
		t.Errorf("the value is still in the message: %v", RedactValues(err))
	}
}

func TestSymbolWithASpaceAroundItIsRejected(t *testing.T) {
	_, err := parseSet(t, "ruleset: t@1\nsinks:\n  - symbol: \" example.com/gen.Sink\"\n    args: [0]\n    category: x\n")
	if err == nil || !strings.Contains(err.Error(), "space around") {
		t.Fatalf("a symbol with a space around it is accepted: %v", err)
	}
}
