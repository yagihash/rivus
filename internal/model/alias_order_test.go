package model

import (
	"strings"
	"testing"
)

// With several symbols that clash, the error names the same one every time (the first by name), not the one that the map gave first.
func TestAliasArgsErrorNamesTheFirstSymbol(t *testing.T) {
	var a, c strings.Builder
	a.WriteString("ruleset: a@1\npropagators:\n")
	c.WriteString("ruleset: c@1\npropagators:\n")
	for _, s := range []string{"s7", "s3", "s9", "s1", "s5", "s8", "s2"} {
		a.WriteString("  - symbol: " + s + "\n    flows: [{from: arg0, to: result, alias: true}]\n")
		c.WriteString("  - symbol: " + s + "\n    flows: [{from: arg1, to: result, alias: true}]\n")
	}
	ra, err := parseSet(t, a.String())
	if err != nil {
		t.Fatal(err)
	}
	rc, err := parseSet(t, c.String())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		err := Merge(ra, rc).Validate()
		if err == nil || !strings.HasPrefix(err.Error(), "s1: ") {
			t.Fatalf("run %d: %v, want the error of s1", i, err)
		}
	}
}
