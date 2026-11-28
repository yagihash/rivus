package docsgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Targets with the same pass_line come in the order of their names: the file is read into a map, whose order changes from run to run.
func TestLoadPassLineSameLineOrder(t *testing.T) {
	var b strings.Builder
	b.WriteString("{\n")
	var want []string
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("t%02d", (i*5)%12)
		fmt.Fprintf(&b, "%q: {\"pass_line\": \"A\", \"lang\": \"go\"},\n", name)
		want = append(want, name)
	}
	b.WriteString("\"z\": {\"pass_line\": \"A\"}\n}\n")
	want = append(want, "z")
	sort.Strings(want)
	file := filepath.Join(t.TempDir(), "targets.json")
	if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 20; run++ {
		rows, err := LoadPassLine(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, r := range rows {
			if r.Target != want[i] {
				t.Fatalf("run %d: the %dth is %s, want %s", run, i, r.Target, want[i])
			}
		}
	}
}
