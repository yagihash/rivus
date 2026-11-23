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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// runCut runs the rules and the main of a workspace that has one call of a model that cuts a flow, with and
// without the models decided by an LLM.
func runCut(t *testing.T, libExtra, rules, main string) (rep, det report.Report) {
	t.Helper()
	w := newWorkspace(t)
	w.write("lib/lib.go", wsLib+libExtra)
	w.write("rules.yaml", wsRules+rules)
	w.write("main/main.go", main)
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "./main"), &rep); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "-deterministic-only", "./main"), &det); err != nil {
		t.Fatal(err)
	}
	return rep, det
}

const wsCutMain = `package main

import "example.com/tmp/lib"

func F() {
	a := lib.Source()
	lib.Sink(lib.Cut(a, "x"))
}
`

func checkCut(t *testing.T, rep, det report.Report) {
	t.Helper()
	if len(rep.Findings) != 0 {
		t.Fatalf("the model cuts the flow, so there is no finding: %d", len(rep.Findings))
	}
	if len(rep.Suppressions) != 1 {
		t.Fatalf("the cut must be recorded: %+v", rep.Suppressions)
	}
	sp := rep.Suppressions[0]
	if sp.Model.Origin != "llm" || sp.Model.Symbol != "example.com/tmp/lib.Cut" || sp.Model.Rationale == "" ||
		sp.Labels[0] != "user_input" || len(sp.Origins) != 1 || !strings.Contains(sp.Snippet, "lib.Cut") {
		t.Errorf("suppression = %+v", sp)
	}
	if len(det.Findings) != 1 || len(det.Suppressions) != 0 || det.IgnoredModels != 1 {
		t.Errorf("deterministic run: findings=%d suppressions=%d ignored=%d", len(det.Findings), len(det.Suppressions), det.IgnoredModels)
	}
}

func TestLLMPureModelSuppressionIsRecorded(t *testing.T) {
	t.Parallel()
	rep, det := runCut(t, "\nfunc Cut(s, t string) string { return s }\n", `
propagators:
  - symbol: example.com/tmp/lib.Cut
    pure: true
    origin: llm
    decided_at: "2026-11-20"
    rationale: "Cut returns a constant"
    model_version: test-model
`, wsCutMain)
	checkCut(t, rep, det)
}

func TestLLMFlowsModelSuppressionIsRecorded(t *testing.T) {
	t.Parallel()
	rep, det := runCut(t, "\nfunc Cut(s, t string) string { return s + t }\n", `
propagators:
  - symbol: example.com/tmp/lib.Cut
    flows: [{from: arg1, to: result}]
    origin: llm
    decided_at: "2026-11-20"
    rationale: "Cut returns its second argument"
    model_version: test-model
`, wsCutMain)
	checkCut(t, rep, det)
}

func TestLLMModelThatLetsTheOperandThroughRecordsNoCut(t *testing.T) {
	t.Parallel()
	// a model that lets the operand through records nothing
	rep, _ := runCut(t, "\nfunc Cut(s, t string) string { return s }\n", `
propagators:
  - symbol: example.com/tmp/lib.Cut
    flows: [{from: arg0, to: result}]
    origin: llm
    decided_at: "2026-11-20"
    rationale: "Cut returns its first argument"
    model_version: test-model
`, wsCutMain)
	if len(rep.Suppressions) != 0 || len(rep.Findings) != 1 || !rep.Findings[0].LLMInfluenced {
		t.Errorf("suppressions=%d findings=%d", len(rep.Suppressions), len(rep.Findings))
	}
}

func TestFindingIsInfluencedByModelThatCutAnotherWay(t *testing.T) {
	t.Parallel()
	rep, _ := runCut(t, "\nfunc Cut(s, t string) string { return s }\n", `
propagators:
  - symbol: example.com/tmp/lib.Cut
    pure: true
    origin: llm
    decided_at: "2026-11-20"
    rationale: "Cut returns a constant"
    model_version: test-model
`, `package main

import "example.com/tmp/lib"

func F() {
	a := lib.Source()
	lib.Sink(lib.Cut(a, "x"))
	lib.Sink(a)
}
`)
	if len(rep.Suppressions) != 1 || len(rep.Findings) != 1 {
		t.Fatalf("suppressions=%d findings=%d", len(rep.Suppressions), len(rep.Findings))
	}
	f := rep.Findings[0]
	if !f.LLMInfluenced {
		t.Error("the finding stands, but the model cut the other way from the same source in the same function")
	}
}

func TestFindingNamesTheModelsOnItsPath(t *testing.T) {
	t.Parallel()
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

// The example of the brief (section 7.3): an HTML page that is escaped only
// when the caller says so.
const wsRender = `package main

import (
	"html"
	"net/http"
)

type config struct{ AllowHTML bool }

var cfg config

func render(s string, escape bool) string {
	if escape {
		return html.EscapeString(s)
	}
	return s
}

func handler(w http.ResponseWriter, r *http.Request) {
	bio := r.FormValue("bio")
	out := render(bio, cfg.AllowHTML)
	w.Write([]byte(out))
}
`

func TestConditionalNodeInTheBriefsExample(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	w.write("main/main.go", wsRender)
	w.write("rules.yaml", "ruleset: empty\n")

	var rep report.Report
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "./main"), &rep); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range rep.Findings {
		if f.Category != "xss" {
			continue
		}
		if len(f.ConditionalNodes) != 1 {
			t.Fatalf("nodes = %+v", f.ConditionalNodes)
		}
		n := f.ConditionalNodes[0]
		found = true
		if n.Kind != "value_dependent_sanitization" || n.BranchCondition != "escape" ||
			n.DerivedFrom != "parameter 1 (escape bool)" {
			t.Errorf("node = %+v", n)
		}
		if n.Outcomes["escape == true"] != "sanitized by html.EscapeString" || n.Outcomes["escape == false"] != "tainted (passthrough)" {
			t.Errorf("outcomes = %+v", n.Outcomes)
		}
		if n.CallSiteArgument != "render(bio, cfg.AllowHTML)" || n.Argument != "cfg.AllowHTML" || n.Resolved {
			t.Errorf("call site: %q argument %q resolved %v", n.CallSiteArgument, n.Argument, n.Resolved)
		}
		p := n.Provenance
		if p == nil || len(p.Resolved) != 0 || len(p.Unresolved) != 1 || p.Unresolved[0].TerminalKind != "global_var" ||
			p.Unresolved[0].AttributableTo != "target" {
			t.Errorf("provenance = %+v", p)
		}
		if !strings.Contains(n.FunctionSource, "html.EscapeString") {
			t.Errorf("the function source is part of the facts: %q", n.FunctionSource)
		}
	}
	if !found {
		t.Fatalf("no xss finding in %d findings", len(rep.Findings))
	}
}

// git runs git in the workspace, with an identity of its own.
func (w *workspace) git(args ...string) {
	w.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = w.dir
	if out, err := cmd.CombinedOutput(); err != nil {
		w.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// The revision is not in the IDs, but it is in every output, and a bookmark
// carries the revision it was taken from: the "target" of the output it was
// copied from.
func TestResolveRecordsTheRevision(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	w := newWorkspace(t)
	w.git("init", "-q")
	w.git("add", "-A")
	w.git("commit", "-q", "-m", "base")

	var rep report.Report
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "-sites", "./main"), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Target == nil || rep.Target.Revision == "" {
		t.Fatalf("target of a clean tree: %+v", rep.Target)
	}
	// An agent keeps the findings together with the target they came from.
	data, _ := json.Marshal(map[string]any{"target": rep.Target, "bookmarks": rep.Findings})
	bm := filepath.Join(w.dir, "wrapped.json")
	if err := os.WriteFile(bm, data, 0o644); err != nil {
		t.Fatal(err)
	}
	resolve := func() (report.ResolveResult, *report.Resolution) {
		var res report.ResolveResult
		if err := json.Unmarshal(w.run("resolve", "-rules", "rules.yaml", "-bookmarks", bm, "./main"), &res); err != nil {
			t.Fatal(err)
		}
		return res, res.Results[0]
	}

	res, r := resolve()
	if r.RevisionChange != "same" || r.FromRevision == nil || r.FromRevision.Revision != rep.Target.Revision || res.Target.Revision != rep.Target.Revision {
		t.Errorf("same commit: %+v from %+v to %+v", r.RevisionChange, r.FromRevision, res.Target)
	}

	// A new commit: the revision differs, the ID does not.
	w.write("main/main.go", strings.Replace(wsBase, "func Other() {", "func Another() {}\n\nfunc Other() {", 1))
	w.git("commit", "-q", "-am", "next")
	res, r = resolve()
	if r.RevisionChange != "different" || res.Target.Revision == rep.Target.Revision {
		t.Errorf("new commit: %+v to %+v", r.RevisionChange, res.Target)
	}
	if r.Bookmark != rep.Findings[0].ID {
		t.Errorf("the ID changed with the revision: %s", r.Bookmark)
	}

	// A bookmark with no revision, or outside git: unknown, and no target.
	bare := filepath.Join(w.dir, "bare.json")
	data, _ = json.Marshal(rep.Findings)
	os.WriteFile(bare, data, 0o644)
	var plain report.ResolveResult
	if err := json.Unmarshal(w.run("resolve", "-rules", "rules.yaml", "-bookmarks", bare, "./main"), &plain); err != nil {
		t.Fatal(err)
	}
	if plain.Results[0].RevisionChange != "unknown" || plain.Results[0].FromRevision != nil {
		t.Errorf("no revision: %+v", plain.Results[0])
	}
}

// The rules of a project live in <dir>/.rivus/rules and are read without a
// flag; -no-project-rules leaves them out.
func TestProjectRulesArePluggedIn(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	count := func(args ...string) int {
		var rep report.Report
		if err := json.Unmarshal(w.run(append([]string{"analyze", "-no-builtin-rules"}, args...)...), &rep); err != nil {
			t.Fatal(err)
		}
		return len(rep.Findings)
	}
	if n := count("./main"); n != 0 {
		t.Fatalf("no rules at all, and %d findings", n)
	}
	w.write(".rivus/rules/project.yaml", wsRules)
	if n := count("./main"); n != 1 {
		t.Errorf("the project's rules were not read: %d findings", n)
	}
	if n := count("-no-project-rules", "./main"); n != 0 {
		t.Errorf("-no-project-rules still read them: %d findings", n)
	}
}
