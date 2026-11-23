package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSymbolPackage(t *testing.T) {
	for _, c := range []struct{ lang, sym, want string }{
		{"go", "strings.Join", "strings"},
		{"go", "(*strings.Builder).WriteString", "strings"},
		{"go", "(net/url.Values).Get", "net/url"},
		{"go", "(*google.golang.org/grpc.serverStream).RecvMsg", "google.golang.org/grpc"},
		{"go", "gopkg.in/yaml.v3.Unmarshal", "gopkg.in/yaml.v3"},
		{"go", "example.com/synth.Join", "example.com/synth"},
		{"dart", "package:http/src/client.dart#Client.get", "package:http"},
		{"dart", "dart:io#File.new", ""},
	} {
		if got := SymbolPackage(c.lang, c.sym); got != c.want {
			t.Errorf("SymbolPackage(%s, %s) = %q, want %q", c.lang, c.sym, got, c.want)
		}
	}
}

func TestParseNotes(t *testing.T) {
	text := `# prose that says model-diff: in a sentence is not a note
# model-diff: extra a.B because the body ignores it
# model-diff: value Client Client()
# model-diff: extra a.C
# model-diff: wrong a.D x
# model-diff: onlyonetoken
`
	notes, errs := ParseNotes("m.yaml", text)
	if len(notes) != 2 || notes[0].Kind != "extra" || notes[0].Text != "because the body ignores it" || notes[1].Kind != "value" {
		t.Errorf("notes = %+v", notes)
	}
	if len(errs) != 3 {
		t.Fatalf("errors = %v, want 3 (no reason, unknown kind, malformed)", errs)
	}
	if !strings.Contains(errs[0], "no reason") {
		t.Errorf("errs[0] = %s", errs[0])
	}
}

func TestDropPackage(t *testing.T) {
	text := `ruleset: x@1
propagators:
  # strings
  - symbol: strings.Join
    flows: [{from: args, to: result}]
  - symbol: (*strings.Builder).Write
    flows:
      - {from: arg0, to: recv}
    # a comment inside the row
  # model-diff: extra bytes.Buffer.Write because
  - symbol: bytes.Buffer.Write
    flows: [{from: arg0, to: recv}]
sinks:
  - symbol: strings.Foo
    category: x
  - symbol: os/exec.Command
    category: command_injection
`
	got := DropPackage(text, "go", "strings")
	for _, gone := range []string{"strings.Join", "Builder", "strings.Foo", "a comment inside"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q is still there:\n%s", gone, got)
		}
	}
	for _, kept := range []string{"bytes.Buffer.Write", "os/exec.Command", "model-diff: extra bytes.Buffer.Write", "ruleset: x@1", "sinks:"} {
		if !strings.Contains(got, kept) {
			t.Errorf("%q is gone:\n%s", kept, got)
		}
	}
}

func yes() *bool { b := true; return &b }
func no() *bool  { b := false; return &b }

func TestCompare(t *testing.T) {
	probes := []Probe{
		{ID: 0, Symbol: "p.Agree", Kind: "flow", Variant: "arg0", Outputs: []string{"result0"}},
		{ID: 1, Symbol: "p.Miss", Kind: "flow", Variant: "arg0", Outputs: []string{"result0", "arg1"}},
		{ID: 2, Symbol: "p.Extra", Kind: "flow", Variant: "arg0", Outputs: []string{"result0"}},
		{ID: 3, Symbol: "p.Noted", Kind: "flow", Variant: "arg0", Outputs: []string{"result0"}},
		{ID: 4, Symbol: "p.None", Kind: "flow", Variant: "arg0", Outputs: []string{"result0"}},
		{ID: 5, Symbol: "p.SinkOK", Kind: "sink", Variant: "arg0", Category: "c", Expect: yes()},
		{ID: 6, Symbol: "p.SinkMiss", Kind: "sink", Variant: "arg0", Category: "c", Expect: yes()},
		{ID: 7, Symbol: "p.SinkExtra", Kind: "sink", Variant: "arg1", Category: "c", Expect: no()},
		{ID: 8, Symbol: "p.SinkControl", Kind: "sink", Variant: "const", Category: "c", Expect: no()},
	}
	a := Hits{key(0, "result0"): true, key(1, "result0"): true, key(1, "arg1"): true}
	b := Hits{key(0, "result0"): true, key(1, "result0"): true, key(2, "result0"): true, key(3, "result0"): true,
		key(5, "cat:c"): true, key(7, "cat:c"): true}
	notes := []Note{{Kind: "extra", Symbol: "p.Noted", Text: "wider on purpose"}, {Kind: "extra", Symbol: "p.Gone", Text: "stale", File: "m.yaml", Line: 9}}
	rep := Compare(probes, []Skip{{Symbol: "p.Generic", Reason: "generic"}}, a, b, notes)
	want := map[string]string{"p.Agree": "agree", "p.Miss": "MISS", "p.Extra": "EXTRA", "p.Noted": "EXTRA-intended", "p.None": "agree",
		"p.SinkOK": "agree", "p.SinkMiss": "MISS", "p.SinkExtra": "EXTRA", "p.SinkControl": "agree"}
	for _, r := range rep.Results {
		if r.Status != want[r.Symbol] {
			t.Errorf("%s: %s, want %s", r.Symbol, r.Status, want[r.Symbol])
		}
	}
	if got := rep.Results[1].Miss; len(got) != 1 || got[0] != "arg1" {
		t.Errorf("the place that is missed: %v", got)
	}
	if rep.Counts["unverified"] != 1 || rep.Counts["agree_no_flow"] != 1 || len(rep.Stale) != 1 {
		t.Errorf("counts = %v, stale = %v", rep.Counts, rep.Stale)
	}
	if !rep.Failed() {
		t.Error("a MISS must fail")
	}
	// only the allowed EXTRA and the unverified: not a failure
	ok := Compare(probes[3:4], []Skip{{Symbol: "x", Reason: "r"}}, a, b, notes)
	if ok.Failed() {
		t.Error("an EXTRA with a note and an unverified function must not fail")
	}
	var sb strings.Builder
	rep.Print(&sb)
	for _, s := range []string{"MISS", "EXTRA-intended", "unverified", "stale note"} {
		if !strings.Contains(sb.String(), s) {
			t.Errorf("the table lacks %q:\n%s", s, sb.String())
		}
	}
	if _, err := json.Marshal(rep); err != nil {
		t.Error(err)
	}
}

func TestReadHits(t *testing.T) {
	data := `{"findings":[
	 {"category":"model_diff","source":{"func":"fx.fix3"},"sink":{"symbol":"fx.sink3_result0"}},
	 {"category":"model_diff","source":{"func":"fx.fix3"},"sink":{"symbol":"fx.sink4_result0"}},
	 {"category":"sql_injection","source":{"func":"fx.fix7"},"sink":{"symbol":"example.com/p.Query"}},
	 {"category":"x","source":{"func":"fx.main"},"sink":{"symbol":"fx.sink3_arg0"}}]}`
	h, err := ReadHits([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if !h[key(3, "result0")] || h[key(4, "result0")] || !h[key(7, "cat:sql_injection")] || len(h) != 3 {
		t.Errorf("hits = %v (a sink of another function does not count)", h)
	}
}

// --- the generator and the whole run, on a library of the tests (testdata/synth) ---

var (
	buildOnce sync.Once
	rivusBin  string
	buildErr  error
)

func rivus(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds rivus and runs it")
	}
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "model-diff-test-")
		if err != nil {
			buildErr = err
			return
		}
		rivusBin = filepath.Join(dir, "rivus")
		c := exec.Command("go", "build", "-o", rivusBin, "../../cmd/rivus")
		if out, err := c.CombinedOutput(); err != nil {
			buildErr = &exec.ExitError{Stderr: out}
		}
	})
	if buildErr != nil {
		t.Fatalf("go build: %v %s", buildErr, buildErr.(*exec.ExitError).Stderr)
	}
	return rivusBin
}

// runSynth runs the tool on testdata/synth with a model file of testdata and returns the exit status and the report.
func runSynth(t *testing.T, model string) (int, *Report) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "not", "there", "yet") // the directory is made by the tool
	code := realMain([]string{"-lang", "go", "-pkg", "example.com/synth", "-local", "testdata/synth", "-model", filepath.Join("testdata", model),
		"-rivus", rivus(t), "-repo", "../..", "-out", out})
	data, err := os.ReadFile(filepath.Join(out, "report.json"))
	if err != nil {
		t.Fatalf("%s: no report (exit %d): %v", model, code, err)
	}
	var rep Report
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}
	return code, &rep
}

func TestGenGoUnverified(t *testing.T) {
	rows, _, err := LoadRows("testdata/good.yaml", "go", "example.com/synth")
	if err != nil {
		t.Fatal(err)
	}
	gen, err := GenGo(GoOptions{Pkg: "example.com/synth", Out: t.TempDir(), Local: "testdata/synth", Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{}
	for _, s := range gen.Skips {
		reasons[s.Symbol] = s.Reason
	}
	if !strings.Contains(reasons["example.com/synth.Apply"], "callback") || !strings.Contains(reasons["example.com/synth.Map"], "generic") || len(gen.Skips) != 2 {
		t.Errorf("unverified = %v: the callback and the generic function must be said, and nothing else", gen.Skips)
	}
	code, err := os.ReadFile(filepath.Join(gen.Dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"tp.Join(a0, a1)", "rc.Set(a0)", "sink4_arg0(a0)", "tp.NewBox(a0)", "tp.Open(a0)"} {
		if !strings.Contains(string(code), s) {
			t.Errorf("the generated code lacks %q", s)
		}
	}
}

func TestModelDiffRuns(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		code, rep := runSynth(t, "good.yaml")
		if code != 0 || rep.Counts["MISS"] != 0 || rep.Counts["EXTRA"] != 0 || rep.Counts["unverified"] != 2 {
			t.Errorf("exit %d, counts %v: a model that is right must pass, with the callback and the generic function unverified", code, rep.Counts)
		}
		if rep.Counts["agree"] < 15 || rep.Runs["A"].Seconds == 0 || rep.Runs["B"].Seconds == 0 {
			t.Errorf("counts %v, runs %v", rep.Counts, rep.Runs)
		}
	})
	t.Run("miss", func(t *testing.T) { // the row of Join lost the flow from arg1
		code, rep := runSynth(t, "miss.yaml")
		if code != 1 || rep.Counts["MISS"] != 1 {
			t.Fatalf("exit %d, counts %v", code, rep.Counts)
		}
		for _, r := range rep.Results {
			if r.Status == "MISS" && (r.Symbol != "example.com/synth.Join" || r.Variant != "arg1") {
				t.Errorf("MISS at %s %s", r.Symbol, r.Variant)
			}
		}
	})
	t.Run("extra", func(t *testing.T) {
		code, rep := runSynth(t, "extra.yaml")
		if code != 1 || rep.Counts["EXTRA"] != 1 {
			t.Errorf("exit %d, counts %v", code, rep.Counts)
		}
	})
	t.Run("extra with a note", func(t *testing.T) {
		code, rep := runSynth(t, "extra-noted.yaml")
		if code != 0 || rep.Counts["EXTRA-intended"] != 1 || rep.Results[0].Reason == "" {
			t.Errorf("exit %d, counts %v", code, rep.Counts)
		}
	})
	t.Run("note without a reason", func(t *testing.T) {
		code, rep := runSynth(t, "extra-noreason.yaml")
		if code != 1 || len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "no reason") {
			t.Errorf("exit %d, errors %v", code, rep.Errors)
		}
	})
	t.Run("sinks against the declaration", func(t *testing.T) {
		// Query declares arg0 only (its second argument and the control must stay silent), Run declares a position that it has not,
		// and Open declares a field for a label that the source does not have (the declared position gives no finding)
		code, rep := runSynth(t, "sink-wrong.yaml")
		if code != 1 || len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "arg5") || rep.Counts["MISS"] != 1 {
			t.Errorf("exit %d, errors %v, counts %v", code, rep.Errors, rep.Counts)
		}
		if rep.Counts["agree"] != 7 {
			t.Errorf("counts %v: the positions and controls of Query and Run, and the control of Open, must agree", rep.Counts)
		}
	})
}

// TestChangedPackages: a pull request checks the packages of the rows that it changes, and no other.
func TestChangedPackages(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	dir := filepath.Join(repo, "internal", "model", "builtin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// git in an environment without the user's settings
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
		c.Dir = repo
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go-a.yaml", "ruleset: a@1\npropagators:\n  - symbol: strings.Join\n    flows: [{from: args, to: result}]\n  - symbol: path.Base\n    flows: [{from: arg0, to: result}]\n")
	write("dart-a.yaml", "ruleset: b@1\nsinks:\n  - symbol: package:http/src/client.dart#Client.get\n    args: [0]\n    category: ssrf\n")
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	write("go-a.yaml", "ruleset: a@1\n# a comment is not a change\npropagators:\n  - symbol: strings.Join\n    flows: [{from: arg0, to: result}]\n  - symbol: path.Base\n    flows: [{from: arg0, to: result}]\n  - symbol: net/url.Parse\n    flows: [{from: arg0, to: result0}]\n  - symbol: os.Getpid\n    pure: true\nbody_replaced_packages:\n  - {package: encoding/json, reason: x}\n")
	got, err := changedPackages(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	detail, _ := changedDetail(repo, "HEAD")
	if d := detail["go os"]; d == nil || d.Declared || !d.Symbols["os.Getpid"] || detail["go encoding/json"] == nil || !detail["go encoding/json"].Declared {
		t.Errorf("detail = %+v: os has the changed row, encoding/json the declaration", detail)
	}
	if strings.Join(got, ",") != "go encoding/json,go net/url,go os,go strings" {
		t.Errorf("changed = %v, want the packages of the changed, the added row and the added declaration (not path, not dart)", got)
	}
}

func TestCompareMissReviewed(t *testing.T) {
	probes := []Probe{
		{ID: 0, Symbol: "p.F", Kind: "flow", Variant: "arg0", Outputs: []string{"result0"}},
		{ID: 1, Symbol: "p.F", Kind: "flow", Variant: "arg1", Outputs: []string{"result0"}},
	}
	a := Hits{key(0, "result0"): true, key(1, "result0"): true}
	notes := []Note{{Kind: "miss-reviewed", Symbol: "p.F@arg1", Text: "type-only the type"}, {Kind: "miss-reviewed", Symbol: "p.G@arg0", Text: "x", File: "m.yaml", Line: 3}}
	rep := Compare(probes, nil, a, Hits{}, notes)
	if rep.Results[0].Status != "MISS" || rep.Results[1].Status != "MISS-reviewed" {
		t.Errorf("statuses: %s, %s", rep.Results[0].Status, rep.Results[1].Status)
	}
	if !rep.Failed() || len(rep.Stale) != 1 {
		t.Errorf("failed %v, stale %v: the position without a note still fails, the unused note is stale", rep.Failed(), rep.Stale)
	}
	if ok := Compare(probes[1:], nil, a, Hits{}, notes); ok.Failed() {
		t.Error("a reviewed MISS must not fail")
	}
}

func TestParseNotesMissReviewed(t *testing.T) {
	good := "  # model-diff: miss-reviewed a.F@arg1 type-only the type of v\n  # model-diff: miss-reviewed a.G@arg0 body-default #150 unresolved call\n"
	notes, errs := ParseNotes("m.yaml", good)
	if len(errs) != 0 || len(notes) != 2 {
		t.Fatalf("notes %v, errors %v", notes, errs)
	}
	for _, bad := range []string{
		"# model-diff: miss-reviewed a.F@arg1 because it is fine\n",
		"# model-diff: miss-reviewed a.F@arg1 type-only\n",
		"# model-diff: miss-reviewed a.F@arg1 body-default no number\n",
		"# model-diff: miss-reviewed a.F type-only x\n",
	} {
		if _, errs := ParseNotes("m.yaml", bad); len(errs) != 1 {
			t.Errorf("%q: errors %v", bad, errs)
		}
	}
}

func TestMissReviewedNonPayload(t *testing.T) {
	good := "# model-diff: miss-reviewed time.Parse@arg1 non-payload the zone name: 3 to 5 capital letters, or GMT+n; src=time/format.go:parseTimeZone\n"
	notes, errs := ParseNotes("m.yaml", good)
	if len(errs) != 0 || len(notes) != 1 {
		t.Fatalf("notes %v, errors %v", notes, errs)
	}
	if k, _ := ReviewKind(notes[0]); k != "non-payload" {
		t.Errorf("kind %q", k)
	}
	for _, bad := range []string{
		"# model-diff: miss-reviewed time.Parse@arg1 non-payload the zone name\n",            // no source
		"# model-diff: miss-reviewed time.Parse@arg1 non-payload src=time/format.go:parse\n", // no narrowing
		"# model-diff: miss-reviewed time.Parse@arg1 non-payload\n",
	} {
		if _, errs := ParseNotes("m.yaml", bad); len(errs) != 1 {
			t.Errorf("%q: errors %v", bad, errs)
		}
	}
	probes := []Probe{{ID: 0, Symbol: "time.Parse", Kind: "flow", Variant: "arg1", Outputs: []string{"result0"}}}
	rep := Compare(probes, nil, Hits{key(0, "result0"): true}, Hits{}, notes)
	if rep.Results[0].Status != "MISS-reviewed" || len(rep.Reviewed) != 1 || rep.Failed() {
		t.Errorf("status %s, reviewed %v, failed %v", rep.Results[0].Status, rep.Reviewed, rep.Failed())
	}
}

func TestDartNamedConstructorIsCalledAsConstructor(t *testing.T) {
	dir := t.TempDir()
	src := "class Response {\n  Response.ok(Object? body, {int x = 1}) : this(200);\n  factory Response.found(Object location) => Response(302);\n  Response change({String? a}) => this;\n}\n"
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "response.dart"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &dartGen{pkg: "p", lib: dir}
	for _, n := range []string{"ok", "found"} {
		sig, why := g.signature("package:p/src/response.dart#Response."+n, false)
		if sig == nil || !sig.Ctor || sig.Name != n {
			t.Errorf("%s: %+v %s", n, sig, why)
		}
	}
	if sig, _ := g.signature("package:p/src/response.dart#Response.change", false); sig == nil || sig.Ctor {
		t.Errorf("change is a method: %+v", sig)
	}
}

// A row written with /v* (any major of a Go module, docs/DESIGN-package-models.md 6.9) is a row of the package of the major that is
// checked (-version names it): the package that the symbol names has /v*, the package that is checked has its digits.
func TestMajorWildcardRows(t *testing.T) {
	if got := SymbolPackage("go", "(*github.com/google/go-github/v*/github.RepositoriesService).GetContents"); got != "github.com/google/go-github/v*/github" {
		t.Errorf("SymbolPackage = %q", got)
	}
	if got := SymbolPackage("go", "github.com/jackc/pgx/v*.Connect"); got != "github.com/jackc/pgx/v*" {
		t.Errorf("SymbolPackage of a root package = %q", got)
	}
	if got := concreteIn("(*github.com/google/go-github/v*/github.RepositoriesService).GetContents", "github.com/google/go-github/v93/github"); got != "(*github.com/google/go-github/v93/github.RepositoriesService).GetContents" {
		t.Errorf("concreteIn = %q", got)
	}
	if got := concreteIn("github.com/google/go-github/v93/github.F", "github.com/google/go-github/v93/github"); got != "github.com/google/go-github/v93/github.F" {
		t.Errorf("a symbol without /v* is as it was: %q", got)
	}
	for in, want := range map[string]string{"v93.0.0": "93", "93.1.2": "93", "v2": "2", "latest": "", "": ""} {
		if got, ok := majorOfVersion(in); got != want || ok != (want != "") {
			t.Errorf("majorOfVersion(%q) = %q, %v", in, got, ok)
		}
	}
	text := "sinks:\n  - symbol: x.org/m/v*/pkg.F\n    args: [0]\n  - symbol: x.org/m/v*/other.F\n    args: [0]\n  - symbol: strings.Join\n    args: [0]\n"
	got := DropPackage(text, "go", "x.org/m/v7/pkg")
	if strings.Contains(got, "v*/pkg.F") || !strings.Contains(got, "v*/other.F") || !strings.Contains(got, "strings.Join") {
		t.Errorf("DropPackage:\n%s", got)
	}
	if got := DropPackage(text, "go", "x.org/m/vault/pkg"); got != text {
		t.Errorf("a path that is not a major drops nothing:\n%s", got)
	}
}

func TestCoverageGapsFindsAFunctionWithoutARow(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "m.yaml")
	if err := os.WriteFile(f, []byte("flows:\n  - symbol: strings.ToUpper\n    flows: [{from: arg0, to: result}]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := coverageGaps("strings", []string{f})
	if err != nil {
		t.Fatal(err)
	}
	var upper, lower bool
	for _, g := range got {
		upper = upper || strings.HasPrefix(g, "strings.ToUpper(")
		lower = lower || strings.HasPrefix(g, "strings.ToLower(")
	}
	if upper || !lower {
		t.Errorf("ToUpper has a row (%v), ToLower has none (%v)", upper, lower)
	}
}

// TestRunChangedListsWhatItDoesNotRun: a changed package of a language that is not run is listed as not run (never as passed), a package
// whose rows were all removed has nothing to check, and a table of those two is not a failure.
func TestRunChangedListsWhatItDoesNotRun(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	dir := filepath.Join(repo, "internal", "model", "builtin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
		c.Dir = repo
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go-a.yaml", "ruleset: a@1\npropagators:\n  - symbol: strings.Join\n    flows: [{from: args, to: result}]\n")
	write("dart-a.yaml", "ruleset: b@1\nsinks:\n  - symbol: package:http/src/client.dart#Client.get\n    args: [0]\n    category: ssrf\n")
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	write("go-a.yaml", "ruleset: a@1\n")
	write("dart-a.yaml", "ruleset: b@1\nsinks:\n  - symbol: package:http/src/client.dart#Client.get\n    args: [0, 1]\n    category: ssrf\n")
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	code := runChanged(repo, "HEAD", "go", t.TempDir(), nil)
	os.Stdout = old
	w.Close()
	data, _ := io.ReadAll(r)
	text := string(data)
	if code != 0 || !strings.Contains(text, "not run") || !strings.Contains(text, "dart package:http") || !strings.Contains(text, "nothing  ") {
		t.Errorf("exit %d:\n%s", code, text)
	}
}

func TestVerifiedVersion(t *testing.T) {
	f := filepath.Join(t.TempDir(), "go-x.yaml")
	text := "ruleset: x@1\nsinks:\n  - symbol: example.com/m/v*/p.F\n    args: [0]\n    category: c\nverified_versions:\n  - package: example.com/m/v*/p\n    versions: \">=3.0.0 <4.0.0\"\n    checked_with: v3.1.0\n    checked: 2026-11-23\n"
	if err := os.WriteFile(f, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasRows([]string{f}, "go", "example.com/m/v3/p") || hasRows([]string{f}, "go", "example.com/m/v3/q") {
		t.Error("the row with /v* is a row of the package of the major that is checked, and of no other package")
	}
	if got := verifiedVersion([]string{f}, "example.com/m/v*/p"); got != "v3.1.0" {
		t.Errorf("version = %q", got)
	}
	if got := verifiedVersion([]string{f}, "example.com/m/v*/p/sub"); got != "v3.1.0" {
		t.Errorf("a package under the module that has the entry: %q", got)
	}
	if got := verifiedVersion([]string{f}, "example.com/other"); got != "" {
		t.Errorf("version = %q for a package that has no entry", got)
	}
}

func hasError(rep *Report, sub string) bool {
	for _, e := range rep.Errors {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

func rowState(rep *Report, sym string) *RowState {
	for i := range rep.Rows {
		if rep.Rows[i].Symbol == sym {
			return &rep.Rows[i]
		}
	}
	return nil
}

// A pure row has to be right about the body: a flow that the body has (to the result, to a callback, from the receiver) and the row
// loses is a MISS.
func TestPureRows(t *testing.T) {
	t.Run("right", func(t *testing.T) {
		code, rep := runSynth(t, "pure-good.yaml")
		if code != 0 || rep.Counts["MISS"] != 0 || rep.Counts["rows_judged"] != 1 {
			t.Errorf("exit %d, counts %v", code, rep.Counts)
		}
	})
	t.Run("wrong", func(t *testing.T) {
		code, rep := runSynth(t, "pure-wrong.yaml")
		if code != 1 || rep.Counts["MISS"] != 3 {
			t.Fatalf("exit %d, counts %v: the argument, the callback and the receiver must be found", code, rep.Counts)
		}
		got := map[string]string{}
		for _, r := range rep.Results {
			if r.Status == "MISS" {
				got[r.Symbol] = r.Variant + " " + strings.Join(r.Miss, ",")
			}
		}
		if got["example.com/synth.Walk"] != "arg0 cb1_0" || got["example.com/synth.Upper"] != "arg0 result0" {
			t.Errorf("MISS = %v", got)
		}
	})
}

// A row whose symbol is not in the package fails, whatever the kind of the row, and the row is listed as not judged.
func TestRowOfASymbolThatIsNotThere(t *testing.T) {
	code, rep := runSynth(t, "pure-missing.yaml")
	if code != 1 || len(rep.Errors) != 4 {
		t.Fatalf("exit %d, errors %v", code, rep.Errors)
	}
	for _, sym := range []string{"example.com/synth.Gone", "(*example.com/synth.Box).Nope", "(*example.com/synth.Nothing).Get", "example.com/synth.GoneToo"} {
		if !hasError(rep, sym) {
			t.Errorf("no error for %s: %v", sym, rep.Errors)
		}
		if st := rowState(rep, sym); st == nil || st.Judged {
			t.Errorf("%s must be listed as not judged: %+v", sym, st)
		}
	}
	if st := rowState(rep, "example.com/synth.Drop"); st == nil || !st.Judged {
		t.Errorf("the row that is there is judged: %+v", st)
	}
}

// A sanitizer row is read against the body: the value has to reach the result, the flows of the row are the body's, and a body that
// carries nothing is "not judged" (never agreed).
func TestSanitizerRows(t *testing.T) {
	t.Run("right", func(t *testing.T) {
		code, rep := runSynth(t, "sanitizer-good.yaml")
		if code != 0 || rep.Counts["MISS"] != 0 || rep.Counts["EXTRA"] != 0 {
			t.Fatalf("exit %d, counts %v", code, rep.Counts)
		}
		if st := rowState(rep, "example.com/synth.Esc"); st == nil || !st.Judged {
			t.Errorf("Esc: %+v", st)
		}
		if st := rowState(rep, "example.com/synth.Drop"); st == nil || st.Judged || rep.Counts["rows_not_judged"] != 1 {
			t.Errorf("Drop carries nothing and must be not judged: %+v, counts %v", st, rep.Counts)
		}
	})
	t.Run("wrong", func(t *testing.T) {
		code, rep := runSynth(t, "sanitizer-wrong.yaml")
		if code != 1 || rep.Counts["MISS"] != 1 || rep.Counts["EXTRA"] != 1 || !hasError(rep, "arg3") {
			t.Fatalf("exit %d, counts %v, errors %v: Pair omits arg1 (MISS), Drop's flow is not in the body (EXTRA), Esc names arg3", code, rep.Counts, rep.Errors)
		}
		for _, r := range rep.Results {
			if r.Status == "MISS" && (r.Symbol != "example.com/synth.Pair" || r.Variant != "arg1") {
				t.Errorf("MISS at %s %s", r.Symbol, r.Variant)
			}
		}
	})
}

// A source row is looked up in the package (the symbol, the field, the type, the positions); one that cannot be seen in a probe is not judged.
func TestSourceRows(t *testing.T) {
	t.Run("right", func(t *testing.T) {
		code, rep := runSynth(t, "source-rows.yaml")
		if code != 0 || rep.Counts["rows_judged"] != 3 || rep.Counts["rows_not_judged"] != 1 {
			t.Errorf("exit %d, counts %v", code, rep.Counts)
		}
		if st := rowState(rep, "example.com/synth.Copy"); st == nil || st.Judged {
			t.Errorf("a source whose result is a number: %+v", st)
		}
	})
	t.Run("wrong", func(t *testing.T) {
		code, rep := runSynth(t, "source-wrong.yaml")
		if code != 1 || len(rep.Errors) != 4 {
			t.Fatalf("exit %d, errors %v", code, rep.Errors)
		}
		for _, sub := range []string{"synth.Nope", "Opts.Nope", "arg4", "arg1, whose type string cannot be written into"} {
			if !hasError(rep, sub) {
				t.Errorf("no error that has %q: %v", sub, rep.Errors)
			}
		}
		if rep.Counts["rows_judged"] != 0 {
			t.Errorf("a wrong row is not judged: %v", rep.Counts)
		}
	})
}

// A position whose type the source cannot be put into (a struct pointer) is listed as unverified, not skipped without a word, and the row
// is still judged by the positions that can be probed.
func TestArgumentThatCannotHoldTheSource(t *testing.T) {
	code, rep := runSynth(t, "arg-struct.yaml")
	if code != 0 || len(rep.Unverified) != 1 || rep.Unverified[0].Variant != "arg0" || !strings.Contains(rep.Unverified[0].Reason, "cannot be put into: *example.com/synth.Opts") {
		t.Errorf("exit %d, unverified %+v", code, rep.Unverified)
	}
	if rep.Counts["rows_judged"] != 1 {
		t.Errorf("counts %v", rep.Counts)
	}
}

// The counts of the rows say what the test really checked: a flows row whose probes show no flow in either run is not judged.
func TestJudgeCountsOnlyWhatWasShown(t *testing.T) {
	rows := []Row{{Kind: RowFlow, Symbol: "p.Seen"}, {Kind: RowFlow, Symbol: "p.Silent"}, {Kind: RowFlow, Symbol: "p.Generic"}, {Kind: RowSink, Symbol: "p.Sink"}}
	probes := []Probe{
		{ID: 0, Symbol: "p.Seen", Kind: "flow", Variant: "arg0", Outputs: []string{"result0"}},
		{ID: 1, Symbol: "p.Silent", Kind: "flow", Variant: "arg0", Outputs: []string{"result0"}},
		{ID: 2, Symbol: "p.Sink", Kind: "sink", Variant: "arg0", Category: "c", Expect: yes()},
	}
	hits := Hits{key(0, "result0"): true, key(2, "cat:c"): true}
	rep := Compare(probes, []Skip{{Symbol: "p.Generic", Reason: "generic function (not checked)"}}, hits, hits, nil)
	rep.Judge(rows, nil)
	if rep.Counts["rows_judged"] != 2 || rep.Counts["rows_not_judged"] != 2 {
		t.Fatalf("counts %v", rep.Counts)
	}
	if st := rowState(rep, "p.Silent"); st == nil || st.Judged || !strings.Contains(st.Reason, "no flow") {
		t.Errorf("Silent: %+v", st)
	}
	if st := rowState(rep, "p.Generic"); st == nil || st.Judged || !strings.Contains(st.Reason, "generic") {
		t.Errorf("Generic: %+v", st)
	}
	if line := rep.RowSummary(); !strings.Contains(line, "2 judged / 2 not judged") || !strings.Contains(line, "flow 1/2") {
		t.Errorf("summary: %s", line)
	}
}

// The count of the .dead-allow rows: judged by a report, in a package that was run and not judged, and in a package that has no report.
func TestDeadAllowCount(t *testing.T) {
	repo, reports := t.TempDir(), t.TempDir()
	dir := filepath.Join(repo, "internal", "model", "builtin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dead := "# comment\nstrings.Join | debt | x\nstrings.Cut | debt | x\nexample.com/m/v*/p.F | not called\nnet/url.Parse | debt | x\n"
	if err := os.WriteFile(filepath.Join(dir, "go-a.dead-allow"), []byte(dead), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Report{Package: "strings", Rows: []RowState{{Symbol: "strings.Join", Row: "flow", Judged: true}, {Symbol: "strings.Cut", Row: "flow", Reason: "x"}}}
	rep2 := Report{Package: "example.com/m/v3/p", Rows: []RowState{{Symbol: "example.com/m/v3/p.F", Row: "sink", Judged: true}}}
	for i, r := range []Report{rep, rep2} {
		data, _ := json.Marshal(r)
		if err := os.WriteFile(filepath.Join(reports, fmt.Sprintf("report-%d.json", i)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	err := deadAllowCount(repo, reports)
	os.Stdout = old
	w.Close()
	data, _ := io.ReadAll(r)
	if err != nil || !strings.Contains(string(data), "total") || !strings.Contains(string(data), "     4          2              0            1           1") {
		t.Errorf("err %v:\n%s", err, data)
	}
}

// A pull request answers for the rows that it changed: a MISS on a row that it did not touch is a notice (pre-existing), not a failure; a row
// that it changed fails; a package whose declaration changed is not restricted (every finding fails).
func TestOnlyTheChangedRowsFail(t *testing.T) {
	run := func(only string) (int, *Report) {
		t.Helper()
		out := filepath.Join(t.TempDir(), "o")
		args := []string{"-lang", "go", "-pkg", "example.com/synth", "-local", "testdata/synth", "-model", "testdata/pure-wrong.yaml", "-rivus", rivus(t), "-repo", "../..", "-out", out}
		if only != "" {
			args = append(args, "-only", only)
		}
		code := realMain(args)
		data, err := os.ReadFile(filepath.Join(out, "report.json"))
		if err != nil {
			t.Fatal(err)
		}
		var rep Report
		if err := json.Unmarshal(data, &rep); err != nil {
			t.Fatal(err)
		}
		return code, &rep
	}
	// pure-wrong.yaml has three MISS (Upper, Walk, Box.Get)
	code, rep := run("example.com/synth.Drop") // an unrelated row changed
	if code != 0 || len(rep.PreExisting) != 3 || rep.Counts["MISS"] != 0 || rep.Counts["pre-existing"] != 3 {
		t.Errorf("unchanged rows: exit %d, counts %v, pre-existing %v", code, rep.Counts, rep.PreExisting)
	}
	code, rep = run("example.com/synth.Walk")
	if code != 1 || rep.Counts["MISS"] != 1 || len(rep.PreExisting) != 2 {
		t.Errorf("a changed row: exit %d, counts %v, pre-existing %v", code, rep.Counts, rep.PreExisting)
	}
	code, rep = run("") // a changed declaration: every finding counts
	if code != 1 || rep.Counts["MISS"] != 3 || len(rep.PreExisting) != 0 {
		t.Errorf("a declaration: exit %d, counts %v", code, rep.Counts)
	}
}
