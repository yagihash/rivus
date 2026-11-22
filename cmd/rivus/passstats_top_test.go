package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/testutil"
)

type passStatsOut struct {
	Stats struct {
		TimedOut       bool   `json:"timed_out"`
		LargestSCC     int    `json:"largest_scc"`
		DurationMilli  int64  `json:"duration_ms"`
		LargestSummary string `json:"largest_summary"`
		PackagesTop    []struct {
			Package    string `json:"package"`
			Functions  int    `json:"functions"`
			Reanalyses int    `json:"reanalyses"`
		} `json:"packages_top"`
		LargestSCCPackages []struct {
			Package   string `json:"package"`
			Functions int    `json:"functions"`
		} `json:"largest_scc_packages"`
	} `json:"stats"`
}

func runPassStats(t *testing.T, dir string, args ...string) passStatsOut {
	t.Helper()
	cmd := exec.Command(binary, append([]string{"analyze", "-rules", "rules.yaml", "-pass-stats"}, append(args, "./...")...)...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze %v: %v", args, err)
	}
	var r passStatsOut
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// bigSCC writes a module of n functions that call each other in one component (like testdata/scc, but larger).
func bigSCC(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("package main\n\nimport (\"os\"; \"os/exec\")\n\ntype T struct{ a, b string; next *T }\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "func f%03d(x string, t *T, k int) string { if k <= 0 { return x }; t.a = x; y := f%03d(t.b, t, k-1); t.b = y; if k%%2 == 0 { return f%03d(x, t.next, k-1) + f%03d(y, t, k-1) }; exec.Command(t.a).Run(); return y }\n",
			i, (i*7+1)%n, (i*3+2)%n, (i+1)%n)
	}
	b.WriteString("\nfunc main() { t := &T{}; exec.Command(f000(os.Getenv(\"X\"), t, 5)).Run() }\n")
	rules, err := os.ReadFile(filepath.Join(testutil.Testdata(), "..", "scc", "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"a.go": []byte(b.String()), "go.mod": []byte("module example.com/big\n\ngo 1.22\n"), "rules.yaml": rules} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// A run that timed out still says where the work went: packages_top and the largest summary of what was analysed,
// and largest_scc_packages, which is made from the call graph before the analysis (so it is there even when the
// limit is gone before the first function).
func TestPassStatsOfATimedOutRun(t *testing.T) {
	dir := filepath.Join(testutil.Testdata(), "..", "scc")
	r := runPassStats(t, dir, "-timeout", "1ns")
	if !r.Stats.TimedOut {
		t.Fatal("stats.timed_out should be true")
	}
	if len(r.Stats.PackagesTop) == 0 || len(r.Stats.LargestSCCPackages) == 0 {
		t.Errorf("packages_top and largest_scc_packages should be there: %+v", r.Stats)
	}
	if got := r.Stats.LargestSCCPackages[0]; got.Package != "example.com/scc" || got.Functions != r.Stats.LargestSCC {
		t.Errorf("largest_scc_packages: %+v, largest_scc %d", got, r.Stats.LargestSCC)
	}
}

// The limit is looked at inside the rounds of a large component too (it used to be looked at between the
// components and the functions of the small ones only): the run stops soon after the limit, and says what it did.
func TestTimeoutInsideALargeSCC(t *testing.T) {
	if testing.Short() {
		t.Skip("a synthetic component of 60 functions, a few seconds")
	}
	dir := bigSCC(t, 60)
	r := runPassStats(t, dir, "-timeout", "50ms")
	if !r.Stats.TimedOut {
		t.Fatal("stats.timed_out should be true")
	}
	// the whole analysis takes about 3 s on a laptop; the bound is loose so that a slow machine passes
	if r.Stats.DurationMilli > 1500 {
		t.Errorf("the run took %d ms with a limit of 50 ms", r.Stats.DurationMilli)
	}
	if r.Stats.LargestSummary == "" || len(r.Stats.PackagesTop) == 0 || len(r.Stats.LargestSCCPackages) == 0 {
		t.Errorf("the partial breakdown is missing: %+v", r.Stats)
	}
	if r.Stats.LargestSCC != 60 {
		t.Errorf("largest_scc = %d, want 60", r.Stats.LargestSCC)
	}
}

func TestPassStatsTopAndSort(t *testing.T) {
	dir := filepath.Join(testutil.Testdata(), "..", "scc")
	all := runPassStats(t, dir, "-pass-stats-top", "0")
	one := runPassStats(t, dir, "-pass-stats-top", "1", "-pass-stats-sort", "functions")
	if len(one.Stats.PackagesTop) != 1 || len(all.Stats.PackagesTop) < 1 {
		t.Fatalf("top 1: %d rows, top 0: %d rows", len(one.Stats.PackagesTop), len(all.Stats.PackagesTop))
	}
	cmd := exec.Command(binary, "analyze", "-rules", "rules.yaml", "-pass-stats", "-pass-stats-sort", "bogus", "./...")
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		t.Error("an unknown -pass-stats-sort should be an error")
	}
}
