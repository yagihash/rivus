package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestCountParityTodos(t *testing.T) {
	bench := t.TempDir()
	write := func(name, body string) {
		p := filepath.Join(bench, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("zoo-rust/cells/src/lib.rs", "//! @todo(F/a/x: PARITY: one)\n//! @todo(F/a/y: PARITY: two)\n//! @todo(F/a/z: not a difference)\n//! @na(F/a/n: PARITY is in the text of an @na)\n")
	write("zoo-rust/cells/src/a.rs", "// @todo(F/a/x: PARITY: again)\n")
	write("zoo-dart/cells/lib/a.dart", "// @todo(F/b/q: PARITY: one)\n")
	if got := countParityTodos(bench, "rust", ".rs"); got != 2 {
		t.Errorf("rust = %d, want 2 (a cell that is on two lines is one; no PARITY, an @na)", got)
	}
	if got := countParityTodos(bench, "dart", ".dart"); got != 1 {
		t.Errorf("dart = %d, want 1", got)
	}
	if got := countParityTodos(bench, "go", ".go"); got != 0 {
		t.Errorf("go = %d, want 0", got)
	}
}
