package model

import (
	"strings"
	"testing"
)

func TestNeutralisesNeedsAddsAndNoDropAll(t *testing.T) {
	for name, src := range map[string]string{
		"no adds": "ruleset: t@1\nsanitizers:\n  - symbol: f\n    drops: [user_input]\n    neutralises: [xss]\n",
		"drop *":  "ruleset: t@1\nsanitizers:\n  - symbol: f\n    drops: [\"*\"]\n    adds: [a]\n    neutralises: [xss]\n",
		"empty":   "ruleset: t@1\nsanitizers:\n  - symbol: f\n    drops: [user_input]\n    adds: [a]\n    neutralises: [\"\"]\n",
	} {
		if _, err := Parse(strings.NewReader(src), "t.yaml"); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	rs, err := Parse(strings.NewReader("ruleset: t@1\nsanitizers:\n  - symbol: f\n    drops: [user_input]\n    adds: [a]\n    neutralises: []\n"), "t.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if rs.Sanitizers[0].Neutralises == nil || len(*rs.Sanitizers[0].Neutralises) != 0 {
		t.Errorf("an empty list is written, and is not the same as none: %v", rs.Sanitizers[0].Neutralises)
	}
}

func TestAcceptsLabel(t *testing.T) {
	x, none := []string{"xss"}, []string{}
	r := Merge(&RuleSet{Sanitizers: []Sanitizer{
		{Symbol: "a", Drops: []string{"user_input"}, Adds: []string{"html_escaped"}, Neutralises: &x},
		{Symbol: "b", Drops: []string{"user_input"}, Adds: []string{"url_encoded"}, Neutralises: &none},
		{Symbol: "c", Drops: []string{"user_input"}}, // the old meaning: no derived label
	}})
	ui := []string{"user_input"}
	for _, c := range []struct {
		accepts []string
		label   string
		cats    []string
		want    bool
	}{
		{ui, "user_input", []string{"xss"}, true},
		{ui, "html_escaped", []string{"xss"}, false},
		{ui, "html_escaped", []string{"command_injection"}, true},
		{ui, "html_escaped", []string{"xss", "command_injection"}, true},
		{nil, "html_escaped", []string{"xss"}, false},
		{[]string{"other"}, "html_escaped", []string{"command_injection"}, false},
		{[]string{"html_escaped"}, "html_escaped", []string{"xss"}, true},
		{ui, "url_encoded", []string{"xss"}, true},
		{ui, "html_escaped", nil, true},
		{ui, "unknown", []string{"xss"}, false},
	} {
		if got := r.AcceptsLabel(c.accepts, c.label, c.cats); got != c.want {
			t.Errorf("AcceptsLabel(%v, %s, %v) = %v, want %v", c.accepts, c.label, c.cats, got, c.want)
		}
	}
}
