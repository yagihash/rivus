package taint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
)

// A function whose local analysis does not reach a fixed point is named once,
// however many times it is analysed again (#20).
func TestNotConvergedNamesAFunctionOnce(t *testing.T) {
	const n = 61
	var vars, lhs, rhs []string
	for i := n - 1; i >= 0; i-- {
		vars = append(vars, fmt.Sprintf("a%d", i))
		lhs = append(lhs, fmt.Sprintf("a%d", i))
	}
	// a60, ..., a1, a0 = a59, ..., a0, x : the values move one place each round
	rhs = []string{}
	for i := n - 1; i >= 1; i-- {
		rhs = append(rhs, fmt.Sprintf("a%d", i-1))
	}
	rhs = append(rhs, "x")
	src := fmt.Sprintf(`package zz

func F(x string, n int) string {
	var %s string
	for i := 0; i < n; i++ {
		%s = %s
	}
	if n > 1000 {
		return F(x, n-1)
	}
	return a%d
}
`, strings.Join(vars, ", "), strings.Join(lhs, ", "), strings.Join(rhs, ", "), n-1)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module zz\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zz.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	l := loadIR(t, dir, "./...")
	rules := &model.Rules{}
	rules.Sinks = map[string][]model.Sink{}
	opts := DefaultOptions()
	opts.Workers = 1
	res := New(l.sg, l.texts, sites.Enumerate(l.ir, l.texts, rules), rules, opts).Run()
	if len(res.Stats.NotConverged) == 0 {
		t.Skip("the function converged: the test says nothing")
	}
	seen := map[string]bool{}
	for _, id := range res.Stats.NotConverged {
		if seen[id] {
			t.Errorf("%s is named more than once: %v", id, res.Stats.NotConverged)
		}
		seen[id] = true
	}
	for _, u := range res.Uncertainties {
		if u.Kind != KindRecursionApproximation {
			continue
		}
		seen := map[string]bool{}
		for _, c := range u.Candidates {
			if seen[c] {
				t.Errorf("candidate %s is repeated: %v", c, u.Candidates)
			}
			seen[c] = true
		}
	}
}
