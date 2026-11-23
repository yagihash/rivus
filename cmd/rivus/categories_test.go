package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/testutil"
)

const catRules = `ruleset: t/cat@1
sources:
  - symbol: example.com/cases/lib.Source
    labels: [user_input]
  - symbol: example.com/cases/lib.SourceBytes
    labels: [user_input]
sinks:
  - symbol: example.com/cases/lib.Sink
    category: alpha
  - symbol: example.com/cases/lib.SinkBytes
    category: beta
  - symbol: example.com/cases/lib.SinkAny
    category: gamma
`

func catRulesFile(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "cat.yaml")
	if err := os.WriteFile(p, []byte(catRules), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func countByCategory(rep *report.Report) map[string]int {
	m := map[string]int{}
	for _, f := range rep.Findings {
		m[f.Category]++
	}
	return m
}

// -categories and -skip-categories cut the findings of the output; the default reports everything, with no categories_excluded;
// what is left out is counted in summary.categories_excluded.
func TestAnalyzeCategoryFilter(t *testing.T) {
	rules := catRulesFile(t)
	var all report.Report
	rivusJSON(t, &all, "analyze", "-rules", rules, "./cases/intra")
	by := countByCategory(&all)
	if len(by) < 2 {
		t.Fatalf("the case must have findings of two categories at least: %v", by)
	}
	if all.Summary.CategoriesExcluded != nil {
		t.Errorf("default: categories_excluded must be absent: %+v", all.Summary.CategoriesExcluded)
	}
	raw := rivus(t, "analyze", "-rules", rules, "./cases/intra")
	if strings.Contains(string(raw), "categories_excluded") {
		t.Error("default: the key is in the JSON")
	}
	var only report.Report
	rivusJSON(t, &only, "analyze", "-rules", rules, "-categories", "alpha", "./cases/intra")
	if got := countByCategory(&only); len(got) != 1 || got["alpha"] != by["alpha"] {
		t.Errorf("-categories alpha: %v (all: %v)", got, by)
	}
	if only.Summary.Findings != by["alpha"] {
		t.Errorf("summary.findings must be what is reported: %d", only.Summary.Findings)
	}
	dropped := 0
	for k, n := range by {
		if k != "alpha" {
			dropped += n
		}
	}
	got := 0
	last := ""
	for _, e := range only.Summary.CategoriesExcluded {
		if e.Category <= last {
			t.Errorf("not sorted: %+v", only.Summary.CategoriesExcluded)
		}
		last = e.Category
		got += e.Findings
		if e.Findings != by[e.Category] {
			t.Errorf("%s: %d findings said, %d dropped", e.Category, e.Findings, by[e.Category])
		}
	}
	if got != dropped {
		t.Errorf("excluded %d, want %d", got, dropped)
	}

	var skip report.Report
	rivusJSON(t, &skip, "analyze", "-rules", rules, "-skip-categories", "alpha", "./cases/intra")
	gs := countByCategory(&skip)
	if gs["alpha"] != 0 || len(skip.Summary.CategoriesExcluded) != 1 || skip.Summary.CategoriesExcluded[0].Category != "alpha" || skip.Summary.CategoriesExcluded[0].Findings != by["alpha"] {
		t.Errorf("-skip-categories alpha: %v %+v", gs, skip.Summary.CategoriesExcluded)
	}
	if len(skip.Findings)+by["alpha"] != len(all.Findings) {
		t.Errorf("findings: %d + %d != %d", len(skip.Findings), by["alpha"], len(all.Findings))
	}
	// the analysis is the same: the other parts of the result do not change
	if len(skip.Uncertainties) != len(all.Uncertainties) || len(skip.Sites) != len(all.Sites) {
		t.Error("uncertainties or sites differ")
	}
	// everything skipped: no findings, and the one that was there is counted
	var none report.Report
	rivusJSON(t, &none, "analyze", "-rules", rules, "-skip-categories", "gamma,beta,alpha", "./cases/hello")
	if len(none.Findings) != 0 {
		t.Errorf("all skipped: %d findings", len(none.Findings))
	}
	if e := none.Summary.CategoriesExcluded; len(e) != 1 || e[0].Category != "alpha" || e[0].Findings != 1 || none.Summary.Findings != 0 {
		t.Errorf("hello has one finding to drop: %+v", e)
	}
}

// The default output does not change with a flag that keeps everything.
func TestAnalyzeCategoryFilterKeepsAll(t *testing.T) {
	rules := catRulesFile(t)
	a := rivus(t, "analyze", "-rules", rules, "./cases/intra")
	cats := []string{"alpha", "beta", "gamma"}
	b := rivus(t, "analyze", "-rules", rules, "-categories", strings.Join(cats, ","), "./cases/intra")
	var ra, rb map[string]any
	json.Unmarshal(a, &ra)
	json.Unmarshal(b, &rb)
	delete(ra["stats"].(map[string]any), "duration_ms")
	delete(rb["stats"].(map[string]any), "duration_ms")
	for _, m := range []map[string]any{ra, rb} {
		for k := range m["stats"].(map[string]any) {
			if strings.Contains(k, "milli") || strings.Contains(k, "duration") || strings.Contains(k, "phase") {
				delete(m["stats"].(map[string]any), k)
			}
		}
	}
	ja, _ := json.Marshal(ra["findings"])
	jb, _ := json.Marshal(rb["findings"])
	if string(ja) != string(jb) {
		t.Error("findings differ when every category is named")
	}
	if _, ok := rb["summary"].(map[string]any)["categories_excluded"]; ok {
		t.Error("nothing was left out, and categories_excluded is there")
	}
}

func TestAnalyzeCategoryErrors(t *testing.T) {
	rules := catRulesFile(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-categories", "nope"}, `unknown category "nope"`},
		{[]string{"-skip-categories", "alpha,nope"}, "resource_name"},
		{[]string{"-categories", "alpha", "-skip-categories", "beta"}, "cannot be combined"},
		{[]string{"-categories", "alpha,,beta"}, "empty category name"},
	} {
		args := append([]string{"analyze", "-rules", rules}, c.args...)
		cmd := exec.Command(binary, append(args, "./cases/hello")...)
		cmd.Dir = testutil.Testdata()
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), c.want) {
			t.Errorf("%v: want an error with %q: %v %s", c.args, c.want, err, out)
		}
	}
}

// index does not take the flags: the graph and the index are made of the whole analysis.
func TestIndexHasNoCategoryFlags(t *testing.T) {
	cmd := exec.Command(binary, "index", "-categories", "alpha", "./cases/hello")
	cmd.Dir = testutil.Testdata()
	if err := cmd.Run(); err == nil {
		t.Fatal("index must not take -categories")
	}
}

// rawFindings are the findings of an output, each as the bytes of its JSON object, in order.
func rawFindings(t *testing.T, out []byte) []json.RawMessage {
	t.Helper()
	var v struct {
		Findings []json.RawMessage `json:"findings"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	return v.Findings
}

// expectedByCategory is the list of all the findings cut as the flags say: with include, the findings that have one of the
// categories; without it, those that have one that is not in skip (a finding with several categories stays when one stays).
func expectedByCategory(t *testing.T, all []json.RawMessage, include, skip []string) []json.RawMessage {
	t.Helper()
	var out []json.RawMessage
	for _, raw := range all {
		var f struct {
			Category   string   `json:"category"`
			Categories []string `json:"categories"`
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatal(err)
		}
		cats := f.Categories
		if len(cats) == 0 {
			cats = []string{f.Category}
		}
		keep := false
		for _, c := range cats {
			if len(include) > 0 && slices.Contains(include, c) || len(include) == 0 && !slices.Contains(skip, c) {
				keep = true
			}
		}
		if keep {
			out = append(out, raw)
		}
	}
	return out
}

// The findings of a run with a flag are the findings of the run without it, minus those that the flag leaves out: the same
// objects in the same order (a count alone would pass with another finding in the place of one).
func TestAnalyzeCategoryFilterIsExactSubset(t *testing.T) {
	rules := catRulesFile(t)
	all := rawFindings(t, rivus(t, "analyze", "-rules", rules, "./cases/intra"))
	for _, c := range []struct {
		flag, value   string
		include, skip []string
	}{
		{"-skip-categories", "alpha", nil, []string{"alpha"}},
		{"-skip-categories", "beta,gamma", nil, []string{"beta", "gamma"}},
		{"-categories", "beta", []string{"beta"}, nil},
		{"-categories", "alpha,gamma", []string{"alpha", "gamma"}, nil},
	} {
		want := expectedByCategory(t, all, c.include, c.skip)
		if len(want) == 0 || len(want) == len(all) {
			t.Fatalf("%s %s: the case must drop some and keep some (%d of %d)", c.flag, c.value, len(want), len(all))
		}
		got := rawFindings(t, rivus(t, "analyze", "-rules", rules, c.flag, c.value, "./cases/intra"))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s %s: %d findings, want the %d of the full run, as they are", c.flag, c.value, len(got), len(want))
		}
	}
}
