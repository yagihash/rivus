package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/testutil"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "rivus-cli")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "rivus")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func rivus(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = testutil.Testdata()
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("rivus %v: %v\n%s", args, err, stderr)
	}
	return out
}

func TestSitesCommand(t *testing.T) {
	var out struct {
		SchemaVersion string `json:"schema_version"`
		Sites         []struct {
			Role   string `json:"role"`
			Symbol string `json:"symbol"`
		} `json:"sites"`
	}
	if err := json.Unmarshal(rivus(t, "sites", "-rules", "rules.yaml", "-role", "sink", "./cases/hello"), &out); err != nil {
		t.Fatal(err)
	}
	if out.SchemaVersion != "rivus.sites/0" || len(out.Sites) != 1 || out.Sites[0].Role != "sink" {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestAnalyzeCommand(t *testing.T) {
	var rep report.Report
	if err := json.Unmarshal(rivus(t, "analyze", "-rules", "rules.yaml", "./cases/hello"), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(rep.Findings))
	}
	f := rep.Findings[0]
	if f.Source.Position.Line != 6 || f.Sink.Position.Line != 7 || len(f.Paths) != 1 || len(f.Paths[0].Steps) == 0 {
		t.Errorf("unexpected finding: %+v", f)
	}
	if f.Confidence != "requires_review" {
		t.Errorf("confidence = %q", f.Confidence)
	}
	if !rep.Stats.Converged {
		t.Error("analysis did not converge")
	}
}

func TestSummaryDetailDropsPathsAndSnippets(t *testing.T) {
	var rep report.Report
	if err := json.Unmarshal(rivus(t, "analyze", "-rules", "rules.yaml", "-detail", "summary", "./cases/hello"), &rep); err != nil {
		t.Fatal(err)
	}
	f := rep.Findings[0]
	if f.Paths != nil || f.Source.Snippet != "" || f.Sink.Snippet != "" {
		t.Errorf("summary detail still has paths or snippets: %+v", f)
	}
}

func TestIndexAndQueryWithoutSource(t *testing.T) {
	idx := filepath.Join(t.TempDir(), "index.json")
	rivus(t, "index", "-rules", "rules.yaml", "-o", idx, "./cases/inter/...")

	// The query must be answered from the index alone.
	var res report.QueryResult
	out := rivus(t, "query", "-index", idx, "-sink", "example.com/cases/lib.Sink")
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) == 0 || len(res.Groups) == 0 || len(res.Matched) == 0 {
		t.Fatalf("empty answer: %d findings, %d groups, %d matched", len(res.Findings), len(res.Groups), len(res.Matched))
	}
	for _, f := range res.Findings {
		if f.Sink.Symbol != "example.com/cases/lib.Sink" {
			t.Errorf("finding for another sink: %s", f.Sink.Symbol)
		}
	}

	// Asking about one site: its ID comes from the sites listing.
	var rep report.Report
	rivusJSON(t, &rep, "analyze", "-rules", "rules.yaml", "-sites", "./cases/hello")
	var sinkID string
	for _, s := range rep.Sites {
		if s.Role == "sink" {
			sinkID = s.ID
		}
	}
	var bySite report.QueryResult
	rivusJSON(t, &bySite, "query", "-rules", "rules.yaml", "-site", sinkID, "./cases/hello")
	if len(bySite.Findings) != 1 || bySite.Findings[0].Sink.ID != sinkID {
		t.Fatalf("site query: %+v", bySite.Findings)
	}
}

func TestOneSidedQueryWithAdHocSymbol(t *testing.T) {
	// "os.ReadFile" is not in the test rules but is a built-in sink; and the
	// query can also name a symbol no rule mentions.
	var res report.QueryResult
	rivusJSON(t, &res, "query", "-rules", "rules.yaml", "-source", "example.com/cases/lib.SourceBytes", "./cases/intra")
	if len(res.Findings) == 0 {
		t.Fatal("source query found nothing")
	}
	for _, f := range res.Findings {
		if f.Source.Symbol != "example.com/cases/lib.SourceBytes" {
			t.Errorf("finding from another source: %s", f.Source.Symbol)
		}
	}
}

func TestQueryNeedsExactlyOneQuestion(t *testing.T) {
	cmd := exec.Command(binary, "query", "-rules", "rules.yaml", "./cases/hello")
	cmd.Dir = testutil.Testdata()
	if err := cmd.Run(); err == nil {
		t.Fatal("query without a question must fail")
	}
}

func rivusJSON(t *testing.T, v any, args ...string) {
	t.Helper()
	if err := json.Unmarshal(rivus(t, args...), v); err != nil {
		t.Fatal(err)
	}
}

func TestImplementationLimitIsReported(t *testing.T) {
	var rep report.Report
	rivusJSON(t, &rep, "analyze", "-rules", "rules.yaml", "-max-implementations", "1", "./cases/impl")
	var limited bool
	for _, e := range rep.Expansions {
		if e.Limited && e.Count == 2 {
			limited = true
		}
	}
	if !limited {
		t.Fatalf("the limited expansion is not in the report: %+v", rep.Expansions)
	}
	var recorded bool
	for _, u := range rep.Uncertainties {
		if u.Kind == "dynamic_dispatch" && strings.HasPrefix(u.Detail, "expansion_limit") {
			recorded = true
		}
	}
	if !recorded {
		t.Error("the report must record that the rule was not expanded")
	}

	var all report.Report
	rivusJSON(t, &all, "analyze", "-rules", "rules.yaml", "./cases/impl")
	if len(all.Findings) != 3 {
		t.Errorf("with the expansion there are 3 findings, got %d", len(all.Findings))
	}
}

func TestSummaryCountsUncertainties(t *testing.T) {
	var rep report.Report
	rivusJSON(t, &rep, "analyze", "-rules", "rules.yaml", "./cases/uncertain")
	if rep.Summary.Findings != len(rep.Findings) || rep.Summary.OnFindings["concurrency"] == 0 {
		t.Errorf("summary = %+v", rep.Summary)
	}
	if rep.Summary.Program.Total != len(rep.Uncertainties) {
		t.Errorf("total %d != %d", rep.Summary.Program.Total, len(rep.Uncertainties))
	}
}
