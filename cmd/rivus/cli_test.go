package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
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
	if out.SchemaVersion != "rivus.sites/1" || len(out.Sites) != 1 || out.Sites[0].Role != "sink" {
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
	want := "requires_review"
	if len(f.Uncertainties) == 0 {
		want = "no_uncertainty_on_path"
	}
	if f.Confidence != want || f.PathSteps != len(f.Paths[0].Steps) {
		t.Errorf("confidence = %q, path_steps = %d", f.Confidence, f.PathSteps)
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

func TestSummaryShowsDispatchCandidatesAndTopSymbols(t *testing.T) {
	var rep report.Report
	rivusJSON(t, &rep, "analyze", "-rules", "rules.yaml", "./cases/uncertain")
	if rep.Summary.DispatchCandidates["0"] == 0 {
		t.Errorf("the call without implementation is not counted: %+v", rep.Summary.DispatchCandidates)
	}
	if len(rep.Summary.TopSymbols["dynamic_dispatch_without_implementation"]) == 0 {
		t.Errorf("top symbols missing: %+v", rep.Summary.TopSymbols)
	}
}

func TestModelsCoverageRanksByFindings(t *testing.T) {
	var cov report.Coverage
	rivusJSON(t, &cov, "models", "coverage", "-rules", "rules.yaml", "./cases/heap")
	if len(cov.Symbols) == 0 {
		t.Fatal("no symbols")
	}
	first := cov.Symbols[0]
	// strings.ToValidUTF8 has no model and lies on a finding's path: it must
	// come before the calls that no finding depends on.
	if first.Symbol != "strings.ToValidUTF8" || first.Findings != 1 || first.Kind != "unmodeled_library_call" {
		t.Errorf("first = %+v", first)
	}
	if first.AttributableTo != "rivus" || first.Stub == "" {
		t.Errorf("first = %+v", first)
	}
	for i := 1; i < len(cov.Symbols); i++ {
		a, b := cov.Symbols[i-1], cov.Symbols[i]
		if a.Findings < b.Findings || a.Findings == b.Findings && a.Calls < b.Calls {
			t.Errorf("not sorted: %+v before %+v", a, b)
		}
	}
}

func TestModelsCoverageYAMLIsAValidRuleFile(t *testing.T) {
	out := rivus(t, "models", "coverage", "-rules", "rules.yaml", "-yaml", "-top", "5", "./cases/heap")
	rs, err := model.Parse(strings.NewReader(string(out)), "generated")
	if err != nil {
		t.Fatalf("the generated stub must load as rules: %v\n%s", err, out)
	}
	if len(rs.Propagators) == 0 || len(rs.Propagators) > 5 {
		t.Errorf("propagators = %d", len(rs.Propagators))
	}
	if !strings.Contains(string(out), "TODO") {
		t.Error("the placeholders must be marked")
	}
}

func TestModelsCoverageFromAnIndex(t *testing.T) {
	idx := filepath.Join(t.TempDir(), "index.json")
	rivus(t, "index", "-rules", "rules.yaml", "-o", idx, "./cases/heap")
	var cov report.Coverage
	if err := json.Unmarshal(rivus(t, "models", "coverage", "-index", idx), &cov); err != nil {
		t.Fatal(err)
	}
	if len(cov.Symbols) == 0 || cov.Total == 0 {
		t.Errorf("empty coverage from the index: %+v", cov)
	}
}

// "rivus sites" lists what "rivus analyze -sites" lists: both expand the
// rules that name an interface method (match: implementations), so the
// implementations are sites in both.
func TestSitesCommandAgreesWithAnalyze(t *testing.T) {
	ids := func(sites []struct {
		ID string `json:"id"`
	}) map[string]bool {
		m := map[string]bool{}
		for _, s := range sites {
			m[s.ID] = true
		}
		return m
	}
	type list struct {
		Sites []struct {
			ID string `json:"id"`
		} `json:"sites"`
	}
	var a, b list
	if err := json.Unmarshal(rivus(t, "sites", "-rules", "rules.yaml", "./cases/..."), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rivus(t, "analyze", "-sites", "-rules", "rules.yaml", "./cases/..."), &b); err != nil {
		t.Fatal(err)
	}
	sa, sb := ids(a.Sites), ids(b.Sites)
	for id := range sb {
		if !sa[id] {
			t.Errorf("site %s is in analyze -sites, not in sites", id)
		}
	}
	for id := range sa {
		if !sb[id] {
			t.Errorf("site %s is in sites, not in analyze -sites", id)
		}
	}
	if len(sa) < 100 {
		t.Errorf("only %d sites: the comparison says nothing", len(sa))
	}
}

// A bookmark of code that is gone is not_found, also when the index was made
// from an IR stream (-ir), which has no Go packages to list (#8). The packages
// analysed are then the units the functions of the stream belong to.
func TestResolveFromAnIRIndexTellsGoneFromNotAnalysed(t *testing.T) {
	dir := t.TempDir()
	irFile := filepath.Join(dir, "g.jsonl")
	if err := os.WriteFile(irFile, rivus(t, "extract", "-texts", "./cases/grpcsrv"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := filepath.Join(dir, "idx.json")
	rivus(t, "index", "-ir", irFile, "-o", idx)
	bm := func(fn string) string {
		p := filepath.Join(dir, "bm.json")
		doc := `[{"id":"s1:0000","func":"` + fn + `","symbol":"x","fingerprint":{"content@1":"00","structural@1":"00"}}]`
		if err := os.WriteFile(p, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	reason := func(fn string) string {
		var out struct {
			Results []struct {
				Reason string `json:"reason"`
			} `json:"results"`
		}
		if err := json.Unmarshal(rivus(t, "resolve", "-index", idx, "-bookmarks", bm(fn)), &out); err != nil || len(out.Results) != 1 {
			t.Fatalf("%v %+v", err, out)
		}
		return out.Results[0].Reason
	}
	if r := reason("example.com/cases/cases/grpcsrv.Removed"); r != report.ReasonNotFound {
		t.Errorf("a function gone from an analysed package: %q", r)
	}
	if r := reason("example.com/cases/cases/neverloaded.F"); r != report.ReasonNotAnalysed {
		t.Errorf("a package that is not in the stream: %q", r)
	}
}

// The IR of -ir is not the code of the directory the command runs in, so
// without an explicit -C the output names no revision; with -C it does (#12).
func TestIRInputHasNoTargetUnlessDirIsGiven(t *testing.T) {
	dir := t.TempDir()
	irFile := filepath.Join(dir, "g.jsonl")
	if err := os.WriteFile(irFile, rivus(t, "extract", "-texts", "./cases/grpcsrv"), 0o644); err != nil {
		t.Fatal(err)
	}
	hasTarget := func(args ...string) bool {
		var out struct {
			Target *struct {
				Revision string `json:"revision"`
			} `json:"target"`
		}
		if err := json.Unmarshal(rivus(t, args...), &out); err != nil {
			t.Fatal(err)
		}
		return out.Target != nil
	}
	if !hasTarget("sites", "-C", ".") {
		t.Skip("the test data is not in a git working tree")
	}
	for _, cmd := range []string{"sites", "analyze"} {
		if hasTarget(cmd, "-ir", irFile) {
			t.Errorf("%s -ir: a target is recorded from the current directory", cmd)
		}
		if !hasTarget(cmd, "-ir", irFile, "-C", ".") {
			t.Errorf("%s -ir -C .: the target the user asked for is missing", cmd)
		}
	}
}

// rivusFails runs the command and returns its stderr; it must exit non-zero.
func rivusFails(t *testing.T, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = testutil.Testdata()
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("rivus %v must fail", args)
	}
	return stderr.String()
}

// Flags that are wrong or do not go together are an error, before the analysis
// starts (#16).
func TestFlagValidation(t *testing.T) {
	dir := t.TempDir()
	idx := filepath.Join(dir, "idx.json")
	rivus(t, "index", "-rules", "rules.yaml", "-o", idx, "./cases/hello")

	cases := []struct {
		name string
		env  []string
		args []string
		want string
	}{
		{"role", nil, []string{"sites", "-role", "sinks", "./cases/hello"}, "role"},
		{"sites and summary", nil, []string{"analyze", "-sites", "-detail", "summary", "./cases/hello"}, "sites"},
		{"detail", nil, []string{"analyze", "-detail", "bogus", "./cases/hello"}, "detail"},
		{"query detail", nil, []string{"query", "-sink", "x", "-detail", "bogus", "./cases/hello"}, "detail"},
		{"query without question", nil, []string{"query", "./cases/hello"}, "exactly one"},
		{"index and packages", nil, []string{"query", "-index", idx, "-sink", "x", "./cases/hello"}, "-index"},
		{"index and analysis flag", nil, []string{"query", "-index", idx, "-sink", "x", "-tests"}, "-index"},
		{"blank extractor flag", nil, []string{"analyze", "-lang", "dart", "-dart-extractor", " ", "./cases/hello"}, "extractor"},
		{"blank extractor env", []string{"RIVUS_DART_EXTRACTOR= "}, []string{"analyze", "-lang", "dart", "./cases/hello"}, "extractor"},
	}
	for _, c := range cases {
		got := rivusFails(t, c.env, c.args...)
		if strings.Contains(got, "panic") || !strings.Contains(got, c.want) {
			t.Errorf("%s: stderr = %q, want an error naming %q", c.name, got, c.want)
		}
	}
}

// A query on an index about a symbol that the index holds no site of says so:
// it is not the same as "nothing reaches it" (#16).
func TestIndexQueryAboutUnknownSymbolSaysSo(t *testing.T) {
	idx := filepath.Join(t.TempDir(), "idx.json")
	rivus(t, "index", "-rules", "rules.yaml", "-o", idx, "./cases/hello")
	var res report.QueryResult
	rivusJSON(t, &res, "query", "-index", idx, "-sink", "fmt.Sprintf")
	if len(res.Matched) != 0 || len(res.Notes) == 0 {
		t.Errorf("matched %d, notes %v", len(res.Matched), res.Notes)
	}
}
