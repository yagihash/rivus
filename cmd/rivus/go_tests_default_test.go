package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Go: the _test.go files are not read unless -tests asks, and the output says how many were left out (ADR-062).
func TestGoTestFilesAreCountedWhenNotRead(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module skipped\n\ngo 1.21\n")
	write(t, filepath.Join(dir, "a.go"), "package skipped\n\nfunc A() {}\n")
	write(t, filepath.Join(dir, "a_test.go"), "package skipped\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { A() }\n")
	write(t, filepath.Join(dir, "b_test.go"), "package skipped_test\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n")
	files := func(args ...string) (int, bool) {
		var r struct {
			Stats struct {
				TestsSkipped *struct {
					Files int `json:"files"`
				} `json:"tests_skipped"`
			} `json:"stats"`
		}
		out := runIn(t, binary, dir, nil, append([]string{"analyze"}, append(args, "./...")...)...)
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		if r.Stats.TestsSkipped == nil {
			return 0, false
		}
		return r.Stats.TestsSkipped.Files, true
	}
	if n, ok := files(); !ok || n != 2 {
		t.Errorf("by default: tests_skipped.files = %d (present %v), want 2", n, ok)
	}
	if n, ok := files("-tests"); ok {
		t.Errorf("with -tests nothing is left out, but tests_skipped.files = %d", n)
	}
}
