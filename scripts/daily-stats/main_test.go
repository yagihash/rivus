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
