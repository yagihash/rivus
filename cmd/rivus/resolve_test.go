package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

// A throw-away module in which the main package is edited between runs, the
// way code changes between two visits of an agent.
type workspace struct {
	t   *testing.T
	dir string
}

const wsLib = `package lib

func Source() string { return "" }
func Sink(s string)  {}
func Sanitize(s string) string { return s }
`

const wsRules = `
sources:
  - symbol: example.com/tmp/lib.Source
    labels: [user_input]
sinks:
  - symbol: example.com/tmp/lib.Sink
sanitizers:
  - symbol: example.com/tmp/lib.Sanitize
    drops: [user_input]
`

const wsBase = `package main

import "example.com/tmp/lib"

func F() {
	a := lib.Source()
	lib.Sink(a)
}

func Other() {
	lib.Sink("x")
}
`

func newWorkspace(t *testing.T) *workspace {
	w := &workspace{t: t, dir: t.TempDir()}
	w.write("go.mod", "module example.com/tmp\n\ngo 1.27\n")
	w.write("lib/lib.go", wsLib)
	w.write("rules.yaml", wsRules)
	w.write("main/main.go", wsBase)
	return w
}

func (w *workspace) write(name, content string) {
	w.t.Helper()
	p := filepath.Join(w.dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		w.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		w.t.Fatal(err)
	}
}

func (w *workspace) run(args ...string) []byte {
	w.t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = w.dir
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		w.t.Fatalf("rivus %v: %v\n%s", args, err, stderr)
	}
	return out
}

// bookmarks takes the findings and the sites of the current code as an agent
// would have kept them, and writes them to a file.
func (w *workspace) bookmarks() (path string, rep report.Report) {
	w.t.Helper()
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "-sites", "./main"), &rep); err != nil {
		w.t.Fatal(err)
	}
	var list []any
	for _, f := range rep.Findings {
		list = append(list, f)
	}
	for _, s := range rep.Sites {
		list = append(list, s)
	}
	data, err := json.Marshal(list)
	if err != nil {
		w.t.Fatal(err)
	}
	path = filepath.Join(w.dir, "bookmarks.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		w.t.Fatal(err)
	}
	return path, rep
}

func (w *workspace) resolve(bookmarks string, patterns ...string) map[string]*report.Resolution {
	w.t.Helper()
	args := append([]string{"resolve", "-rules", "rules.yaml", "-bookmarks", bookmarks}, patterns...)
	var res report.ResolveResult
	if err := json.Unmarshal(w.run(args...), &res); err != nil {
		w.t.Fatal(err)
	}
	out := map[string]*report.Resolution{}
	for _, r := range res.Results {
		out[r.Bookmark] = r
	}
	return out
}

// the bookmark of the sink in F, and of the finding.
func (w *workspace) ids(rep report.Report) (finding, sink, source string) {
	w.t.Helper()
	if len(rep.Findings) != 1 {
		w.t.Fatalf("want 1 finding, got %d", len(rep.Findings))
	}
	f := rep.Findings[0]
	return f.ID, f.Sink.ID, f.Source.ID
}

func TestResolveUnchanged(t *testing.T) {
	w := newWorkspace(t)
	bm, rep := w.bookmarks()
	finding, sink, _ := w.ids(rep)

	got := w.resolve(bm, "./main")
	if r := got[finding]; r.Status != report.StatusUnchanged || r.Flow.Status != report.FlowPresent {
		t.Errorf("finding: %+v flow %+v", r, r.Flow)
	}
	if r := got[sink]; r.Status != report.StatusUnchanged || r.MatchBasis != "content@1" || r.Current.ID != sink {
		t.Errorf("sink: %+v", r)
	}
}

func TestResolveSurvivesLineShifts(t *testing.T) {
	w := newWorkspace(t)
	bm, rep := w.bookmarks()
	finding, sink, _ := w.ids(rep)
	before := rep.Findings[0].Sink.Position.Line

	w.write("main/main.go", strings.Replace(wsBase, "func F() {", "// a comment\n\nfunc unrelated() {}\n\nfunc F() {", 1))
	got := w.resolve(bm, "./main")
	if r := got[finding]; r.Status != report.StatusUnchanged || r.Flow.Status != report.FlowPresent {
		t.Errorf("a line shift must not matter: %+v", r)
	}
	if r := got[sink]; r.Status != report.StatusUnchanged || r.Current.Position.Line == before {
		t.Errorf("the site should have moved to another line: %+v", r)
	}
}

func TestResolveNoticesAChangedFunctionBody(t *testing.T) {
	w := newWorkspace(t)
	bm, rep := w.bookmarks()
	finding, sink, _ := w.ids(rep)

	w.write("main/main.go", strings.Replace(wsBase, "\tlib.Sink(a)\n", "\tprintln(\"x\")\n\tlib.Sink(a)\n", 1))
	got := w.resolve(bm, "./main")
	r := got[sink]
	if r.Status != report.StatusChanged || len(r.ChangedKeys) != 1 || r.ChangedKeys[0] != "body@1" {
		t.Errorf("the call is the same but the function is not: %+v", r)
	}
	if got[finding].Status != report.StatusChanged || got[finding].Flow.Status != report.FlowPresent {
		t.Errorf("finding: %+v", got[finding])
	}
}

func TestResolveEditedArgumentIsRematchedByStructure(t *testing.T) {
	w := newWorkspace(t)
	bm, rep := w.bookmarks()
	finding, sink, _ := w.ids(rep)

	w.write("main/main.go", strings.Replace(wsBase, "lib.Sink(a)", "lib.Sink(a + \"!\")", 1))
	got := w.resolve(bm, "./main")
	r := got[sink]
	if r.Status != report.StatusRematched || r.MatchBasis != "structural@1" {
		t.Fatalf("sink: %+v", r)
	}
	if !strings.Contains(r.PreviousSnippet, "lib.Sink(a)") || !strings.Contains(r.Current.Snippet, `a + "!"`) {
		t.Errorf("both snippets must be there to compare: %q / %q", r.PreviousSnippet, r.Current.Snippet)
	}
	if len(r.Evidence) == 0 {
		t.Error("a match on the structural key alone needs its evidence")
	}
	if got[finding].Status != report.StatusRematched || got[finding].Flow.Status != report.FlowPresent {
		t.Errorf("finding: %+v flow %+v", got[finding], got[finding].Flow)
	}
}

func TestResolveInsertedCallIsFollowedByContent(t *testing.T) {
	w := newWorkspace(t)
	bm, rep := w.bookmarks()
	finding, sink, _ := w.ids(rep)

	// A call to the same function is added before the one that was seen: by
	// position the old site is now another call, by content it is still
	// the same expression.
	w.write("main/main.go", strings.Replace(wsBase, "\ta := lib.Source()\n", "\tlib.Sink(\"first\")\n\ta := lib.Source()\n", 1))
	got := w.resolve(bm, "./main")
	r := got[sink]
	if r.Status != report.StatusRematched || r.MatchBasis != "content@1" {
		t.Fatalf("sink: %+v", r)
	}
	if r.Current.ID == sink {
		t.Error("the ID should have changed with the position")
	}
	if !strings.Contains(r.Current.Snippet, "lib.Sink(a)") {
		t.Errorf("matched the wrong call: %q", r.Current.Snippet)
	}
	if got[finding].Flow.Status != report.FlowPresent {
		t.Errorf("flow: %+v", got[finding].Flow)
	}
}

func TestResolveSanitizerMakesTheFlowAbsent(t *testing.T) {
	w := newWorkspace(t)
	bm, rep := w.bookmarks()
	finding, _, _ := w.ids(rep)

	w.write("main/main.go", strings.Replace(wsBase, "lib.Sink(a)", "lib.Sink(lib.Sanitize(a))", 1))
	got := w.resolve(bm, "./main")
	f := got[finding]
	if f.Flow.Status != report.FlowAbsentNow {
		t.Fatalf("the flow is gone: %+v", f.Flow)
	}
	if f.Source.Status == report.StatusUnresolved || f.Sink.Status == report.StatusUnresolved {
		t.Errorf("both ends still exist: %+v %+v", f.Source, f.Sink)
	}
}

func TestResolveDeletedFunction(t *testing.T) {
	w := newWorkspace(t)
	bm, rep := w.bookmarks()
	finding, sink, _ := w.ids(rep)

	w.write("main/main.go", "package main\n\nimport \"example.com/tmp/lib\"\n\nfunc Other() {\n\tlib.Sink(\"x\")\n}\n")
	got := w.resolve(bm, "./main")
	if r := got[sink]; r.Status != report.StatusUnresolved || r.Reason != report.ReasonNotFound {
		t.Errorf("sink: %+v", r)
	}
	f := got[finding]
	if f.Status != report.StatusUnresolved || f.Flow.Status != report.FlowNotEvaluated {
		t.Errorf("finding: %+v flow %+v", f, f.Flow)
	}
}

func TestResolveDistinguishesNotAnalysedFromGone(t *testing.T) {
	w := newWorkspace(t)
	bm, rep := w.bookmarks()
	_, sink, _ := w.ids(rep)

	// Only the lib package is analysed now: the place is not gone, it was
	// not looked at.
	got := w.resolve(bm, "./lib")
	if r := got[sink]; r.Status != report.StatusUnresolved || r.Reason != report.ReasonNotAnalysed {
		t.Errorf("not analysed: %+v", r)
	}

	// A package that no longer compiles is not analysed either.
	w.write("main/main.go", "package main\n\nfunc F( {\n")
	got = w.resolve(bm, "./main")
	if r := got[sink]; r.Status != report.StatusUnresolved || r.Reason != report.ReasonNotAnalysed {
		t.Errorf("broken package: %+v", r)
	}
}

func TestResolveFromAnIndex(t *testing.T) {
	w := newWorkspace(t)
	bm, rep := w.bookmarks()
	finding, _, _ := w.ids(rep)

	w.run("index", "-rules", "rules.yaml", "-o", "idx.json", "./main")
	var res report.ResolveResult
	if err := json.Unmarshal(w.run("resolve", "-bookmarks", bm, "-index", "idx.json"), &res); err != nil {
		t.Fatal(err)
	}
	for _, r := range res.Results {
		if r.Bookmark == finding && r.Status != report.StatusUnchanged {
			t.Errorf("resolving against an index must give the same answer: %+v", r)
		}
	}
}

func TestResolveRejectsBookmarksWithoutFingerprints(t *testing.T) {
	w := newWorkspace(t)
	w.write("bm.json", `[{"id": "s1:abc"}]`)
	cmd := exec.Command(binary, "resolve", "-rules", "rules.yaml", "-bookmarks", "bm.json", "./main")
	cmd.Dir = w.dir
	if err := cmd.Run(); err == nil {
		t.Fatal("a bookmark that is only an ID cannot be resolved and must be refused")
	}
}

const wsLLMRules = wsRules + `
  - symbol: example.com/tmp/lib.Clean
    drops: [user_input]
    origin: llm
    decided_at: "2026-11-20"
    rationale: "Clean escapes everything; checked in lib/clean.go"
    model_version: test-model
`

const wsLLMMain = `package main

import "example.com/tmp/lib"

func F() {
	a := lib.Source()
	lib.Sink(lib.Clean(a))
}
`

func TestLLMModelSuppressesAndIsRecorded(t *testing.T) {
	w := newWorkspace(t)
	w.write("lib/lib.go", wsLib+"\nfunc Clean(s string) string { return s }\n")
	w.write("rules.yaml", wsLLMRules)
	w.write("main/main.go", wsLLMMain)

	var rep report.Report
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "./main"), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("the model removes the taint, so there is no finding: %d", len(rep.Findings))
	}
	if len(rep.Suppressions) != 1 {
		t.Fatalf("the suppression must be recorded: %+v", rep.Suppressions)
	}
	sp := rep.Suppressions[0]
	if sp.Model.Origin != "llm" || sp.Model.ModelVersion != "test-model" || sp.Model.Rationale == "" ||
		sp.Labels[0] != "user_input" || !strings.Contains(sp.Snippet, "lib.Clean") {
		t.Errorf("suppression = %+v", sp)
	}

	// Without the rules decided by an LLM, the flow is there.
	var det report.Report
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "-deterministic-only", "./main"), &det); err != nil {
		t.Fatal(err)
	}
	if len(det.Findings) != 1 || !det.DeterministicOnly || det.IgnoredModels != 1 || len(det.Suppressions) != 0 {
		t.Errorf("deterministic run: findings=%d deterministic=%v ignored=%d suppressions=%d",
			len(det.Findings), det.DeterministicOnly, det.IgnoredModels, len(det.Suppressions))
	}
}

func TestFindingNamesTheModelsOnItsPath(t *testing.T) {
	w := newWorkspace(t)
	w.write("lib/lib.go", wsLib+"\nfunc Wrap(s string) string { return s }\n")
	w.write("rules.yaml", wsRules+`
propagators:
  - symbol: example.com/tmp/lib.Wrap
    flows: [{from: arg0, to: result}]
    origin: llm
    decided_at: "2026-11-20"
    rationale: "Wrap returns its argument unchanged"
    model_version: test-model
`)
	w.write("main/main.go", "package main\n\nimport \"example.com/tmp/lib\"\n\nfunc F() {\n\tlib.Sink(lib.Wrap(lib.Source()))\n}\n")

	var rep report.Report
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "./main"), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("findings = %d", len(rep.Findings))
	}
	f := rep.Findings[0]
	if !f.LLMInfluenced {
		t.Error("the path goes through a model decided by an LLM")
	}
	var wrap bool
	for _, m := range f.Models {
		if m.Symbol == "example.com/tmp/lib.Wrap" && m.Role == "propagator" && m.Origin == "llm" {
			wrap = true
		}
	}
	if !wrap {
		t.Errorf("models = %+v", f.Models)
	}
}
