package main

import (
	"encoding/json"
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
	write("go-a.yaml", "ruleset: a@1\n# a comment is not a change\npropagators:\n  - symbol: strings.Join\n    flows: [{from: arg0, to: result}]\n  - symbol: path.Base\n    flows: [{from: arg0, to: result}]\n  - symbol: net/url.Parse\n    flows: [{from: arg0, to: result0}]\n")
	got, err := changedPackages(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "go net/url,go strings" {
		t.Errorf("changed = %v, want the packages of the changed and the added row (not path, not dart)", got)
	}
}
