package main

import "testing"

func TestIsEngineChange(t *testing.T) {
	cases := []struct {
		files []string
		want  bool
	}{
		{[]string{"internal/taint/engine.go"}, true},
		{[]string{"internal/model/builtin/x.yaml"}, true},
		{[]string{"extractor/rust/src/main.rs"}, true},
		{[]string{"cmd/rivus/analyze.go"}, true},
		{[]string{"cmd/rivus/analyze_test.go"}, false},
		{[]string{"docs/PRINCIPLES.md", "Makefile"}, false},
		{[]string{"docs/PRINCIPLES.md", "internal/sg/x.go"}, true},
		{[]string{"internal/report/report.go", "bench/compare.py", ".github/workflows/ci.yml"}, false},
		{nil, false},
	}
	for _, c := range cases {
		if got := isEngineChange(c.files); got != c.want {
			t.Errorf("isEngineChange(%v) = %v, want %v", c.files, got, c.want)
		}
	}
}

func TestClassifyBranches(t *testing.T) {
	ahead := map[string]int{"main": 0, "HEAD": 3, "a": 2, "b": 1, "c": 0, "d": 5, "renovate/x": 1, "renovate/y": 2, "renovate/z": 0}
	open := map[string]bool{"b": true}
	orphans, renovate := classifyBranches(ahead, open)
	if len(orphans) != 2 || orphans[0] != (orphan{"a", 2}) || orphans[1] != (orphan{"d", 5}) {
		t.Errorf("orphans = %v", orphans)
	}
	if renovate != 2 {
		t.Errorf("renovate = %d, want 2", renovate)
	}
}
