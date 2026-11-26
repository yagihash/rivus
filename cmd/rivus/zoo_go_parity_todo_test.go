package main

import (
	"strings"
	"testing"
)

// TestZooGoParityTodoMarks holds the checks of bench/zoo-go/parity-todo.txt (zooGoMarkErrors) without the Dart SDK: TestZooDart skips
// without it.
func TestZooGoParityTodoMarks(t *testing.T) {
	dart := map[string]*zooCell{
		"F/a/both":   {name: "F/a/both"},
		"F/a/dartna": {name: "F/a/dartna", na: []string{"go: no such form"}},
		"F/a/todo":   {name: "F/a/todo"},
		"F/a/else":   {name: "F/a/else", elsewhere: []string{"go: F/pkg/gin/query"}},
	}
	goHas := map[string]bool{"F/a/both": true}
	for _, tc := range []struct {
		name, src string
		want      []string // the substrings of the errors, in order
		marked    int
	}{
		{"ok", "@todo(F/a/todo: PARITY: Go で未測定: net/http)", nil, 1},
		{"not a Dart cell", "@todo(F/a/none: PARITY: x)", []string{"F/a/none"}, 1},
		{"Dart has @na(go:", "@todo(F/a/dartna: PARITY: x)", []string{"F/a/dartna"}, 1},
		{"Dart has @elsewhere(go:", "@todo(F/a/else: PARITY: x)", []string{"F/a/else"}, 1},
		{"Go has the cell", "@todo(F/a/both: PARITY: x)", []string{"F/a/both"}, 1},
		{"@na in the file", "@na(F/a/todo: x)", []string{"F/a/todo"}, 0},
	} {
		marked, errs := zooGoMarkErrors(tc.src, dart, goHas)
		if len(marked) != tc.marked || len(errs) != len(tc.want) {
			t.Errorf("%s: marked %d, errors %q; want marked %d, errors %q", tc.name, len(marked), errs, tc.marked, tc.want)
			continue
		}
		for i, w := range tc.want {
			if !strings.Contains(errs[i], w) {
				t.Errorf("%s: error %q has no %q", tc.name, errs[i], w)
			}
		}
	}
}
