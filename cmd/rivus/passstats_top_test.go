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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// -pass-stats says how the elements of the summaries are spread over the functions (summary_sizes) and what they are
// made of, in numbers only: the parts add up to summary_elems, the quantiles are in order, and nothing is there without
// the flag.
func TestPassStatsSummarySizes(t *testing.T) {
	t.Parallel()
	type sizes struct {
		Functions, Empty, Median, P90, P99, Max int
		Over100, Over1000, Over10000            int
		Top1PctShare                            int            `json:"top_1pct_share_permille"`
		OwnElems                                int            `json:"own_elems"`
		DepElems                                int            `json:"dep_elems"`
		ByOutput                                map[string]int `json:"by_output"`
		ByElement                               map[string]int `json:"by_element"`
		ByPathDepth                             map[string]int `json:"by_path_depth"`
	}
	type out struct {
		Stats struct {
			SummaryElems int    `json:"summary_elems"`
			Sizes        *sizes `json:"summary_sizes"`
		} `json:"stats"`
	}
	run := func(args ...string) (out, []byte) {
		cmd := exec.Command(binary, append([]string{"analyze", "-rules", "rules.yaml"}, append(args, "./...")...)...)
		cmd.Dir = bigSCC(t, 40)
		b, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze %v: %v", args, err)
		}
		var r out
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		return r, b
	}
	if plain, _ := run(); plain.Stats.Sizes != nil {
		t.Errorf("summary_sizes is there without -pass-stats: %+v", plain.Stats.Sizes)
	}
	r, _ := run("-pass-stats")
	s := r.Stats.Sizes
	if s == nil {
		t.Fatal("no summary_sizes with -pass-stats")
	}
	sum := func(m map[string]int) int {
		n := 0
		for _, v := range m {
			n += v
		}
		return n
	}
	if sum(s.ByOutput) != r.Stats.SummaryElems {
		t.Errorf("by_output adds up to %d, summary_elems is %d: %+v", sum(s.ByOutput), r.Stats.SummaryElems, s.ByOutput)
	}
	if s.ByElement["real"]+s.ByElement["param"] != r.Stats.SummaryElems {
		t.Errorf("real + param is %d, summary_elems is %d: %+v", s.ByElement["real"]+s.ByElement["param"], r.Stats.SummaryElems, s.ByElement)
	}
	if sum(s.ByPathDepth) != s.ByElement["param"] {
		t.Errorf("by_path_depth adds up to %d, the parameter elements are %d", sum(s.ByPathDepth), s.ByElement["param"])
	}
	if s.OwnElems+s.DepElems != r.Stats.SummaryElems {
		t.Errorf("own + dep is %d, summary_elems is %d", s.OwnElems+s.DepElems, r.Stats.SummaryElems)
	}
	if s.Functions == 0 || !(s.Median <= s.P90 && s.P90 <= s.P99 && s.P99 <= s.Max) || s.Max == 0 {
		t.Errorf("the sizes are not in order: %+v", s)
	}
	if s.Top1PctShare <= 0 || s.Top1PctShare > 1000 || s.Over100 < s.Over1000 || s.Over1000 < s.Over10000 {
		t.Errorf("the shares or counts are not consistent: %+v", s)
	}
	// the same run gives the same numbers (they have no clock)
	r2, _ := run("-pass-stats")
	a, _ := json.Marshal(r.Stats.Sizes)
	b, _ := json.Marshal(r2.Stats.Sizes)
	if string(a) != string(b) {
		t.Errorf("summary_sizes differs between two runs:\n%s\n%s", a, b)
	}
}

// summary_sizes also says how many source sites the Real elements come from and in how many functions each is carried, and
// how many elements the summaries put into the globals (numbers only).
func TestPassStatsSummarySizesOriginsAndGlobals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := `package main

import (
	"os"
	"os/exec"
)

var shared string

func set()   { shared = os.Getenv("A") }
func other() { shared = os.Getenv("B") }
func use()   { exec.Command(shared).Run() }
func relay() { use() }
func main()  { set(); other(); relay() }
`
	rules, err := os.ReadFile(filepath.Join(testutil.Testdata(), "..", "scc", "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"a.go": []byte(src), "go.mod": []byte("module example.com/g\n\ngo 1.22\n"), "rules.yaml": rules} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(binary, "analyze", "-rules", "rules.yaml", "-pass-stats", "./...")
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var r struct {
		Stats struct {
			Sizes struct {
				RealOrigins int `json:"real_origins"`
				OriginReach *struct {
					Max, Median, P90, Over100, Over1000 int
				} `json:"origin_reach"`
				GlobalsTop       []int          `json:"globals_top"`
				GlobalsWithElems int            `json:"globals_with_elems"`
				ByOutput         map[string]int `json:"by_output"`
			} `json:"summary_sizes"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	z := r.Stats.Sizes
	if z.RealOrigins < 2 || z.OriginReach == nil || z.OriginReach.Max < 1 || z.OriginReach.Median > z.OriginReach.P90 || z.OriginReach.P90 > z.OriginReach.Max {
		t.Errorf("the source sites are not counted: %+v", z)
	}
	if z.GlobalsWithElems < 1 || len(z.GlobalsTop) < 1 || z.GlobalsTop[0] < 1 {
		t.Errorf("the global that holds the sources is not counted: %+v", z)
	}
	sum := 0
	for _, n := range z.GlobalsTop {
		sum += n
	}
	if sum > z.ByOutput["global"] {
		t.Errorf("globals_top (%d) holds more than the elements of the global outputs (%d)", sum, z.ByOutput["global"])
	}
	for i := 1; i < len(z.GlobalsTop); i++ {
		if z.GlobalsTop[i] > z.GlobalsTop[i-1] {
			t.Errorf("globals_top is not in order: %v", z.GlobalsTop)
		}
	}
}

// With -pass-stats, summary says where the notes that the findings carry are (on_findings_top: by kind, the symbols with
// the most notes and in how many findings) and how many candidates the calls through an interface have
// (on_findings_dispatch_candidates). Without the flag the summary is as it was.
func TestPassStatsNotesOnFindings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := `package main

import (
	"os"
	"os/exec"
)

type Runner interface{ Run(s string) }

type A struct{}
type B struct{}

func (A) Run(s string) { exec.Command(s).Run() }
func (B) Run(s string) { exec.Command("echo", s).Run() }

func main() {
	var r Runner = A{}
	if len(os.Args) > 3 {
		r = B{}
	}
	r.Run(os.Getenv("X"))
}
`
	rules, err := os.ReadFile(filepath.Join(testutil.Testdata(), "..", "scc", "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"a.go": []byte(src), "go.mod": []byte("module example.com/n\n\ngo 1.22\n"), "rules.yaml": rules} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	type summary struct {
		Summary struct {
			Top map[string][]struct {
				Symbol          string
				Notes, Findings int
			} `json:"on_findings_top"`
			Candidates map[string]int `json:"on_findings_dispatch_candidates"`
		} `json:"summary"`
	}
	run := func(args ...string) summary {
		cmd := exec.Command(binary, append([]string{"analyze", "-rules", "rules.yaml"}, append(args, "./...")...)...)
		cmd.Dir = dir
		b, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze %v: %v", args, err)
		}
		var r summary
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	if plain := run(); plain.Summary.Top != nil || plain.Summary.Candidates != nil {
		t.Errorf("on_findings_top is there without -pass-stats: %+v", plain.Summary)
	}
	r := run("-pass-stats")
	top := r.Summary.Top["dynamic_dispatch"]
	if len(top) == 0 || !strings.Contains(top[0].Symbol, "Run") || top[0].Notes < 1 || top[0].Findings < 1 || top[0].Findings > top[0].Notes {
		t.Fatalf("the interface call is not the top of the dynamic_dispatch notes: %+v", r.Summary.Top)
	}
	if r.Summary.Candidates["2-5"] < 1 {
		t.Errorf("the call with 2 candidates is not counted: %+v", r.Summary.Candidates)
	}
}
