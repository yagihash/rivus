package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yagihash/rivus/internal/testutil"
)

func addsRun(t *testing.T, workers string) ([]byte, map[string]any) {
	t.Helper()
	cmd := exec.Command(binary, "analyze", "-workers", workers, "-rules", "rules.yaml", "-detail", "summary", "./...")
	cmd.Dir = filepath.Join(testutil.Testdata(), "..", "scc")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var r map[string]any
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	return stable(t, out), r["stats"].(map[string]any)
}

// stats has the work of the analysis (adds), in a shape that adds up, and the same numbers under any -workers.
func TestStatsHaveTheAddsAndTheyDoNotDependOnWorkers(t *testing.T) {
	t.Parallel()
	first, st := addsRun(t, "1")
	total, _ := st["adds_total"].(float64)
	maxRun, _ := st["adds_max_single_run"].(float64)
	maxFn, _ := st["adds_max_function_total"].(float64)
	rows, _ := st["adds_per_function"].([]any)
	if total <= 0 || maxRun <= 0 || len(rows) != 5 {
		t.Fatalf("adds_total %v, adds_max_single_run %v, rows %d", total, maxRun, len(rows))
	}
	if !(maxRun <= maxFn && maxFn <= total) {
		t.Errorf("single run %v, function total %v, total %v: not in order", maxRun, maxFn, total)
	}
	var sum float64
	for _, r := range rows {
		sum += r.(map[string]any)["adds"].(float64)
	}
	if sum != total {
		t.Errorf("the rows add up to %v, adds_total is %v", sum, total)
	}
	for i := 0; i < 3; i++ {
		for _, w := range []string{"1", "4", "16"} {
			got, _ := addsRun(t, w)
			if !bytes.Equal(first, got) {
				t.Fatalf("-workers %s (run %d) gave another report\n%s", w, i, firstDifference(first, got))
			}
		}
	}
}
