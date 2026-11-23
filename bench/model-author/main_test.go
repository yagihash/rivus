package main

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const synth = `package synth

import (
	"context"
	"io"
)

type Handler interface{ Serve(s string) }
type HandlerFunc func(s string)

type Dest struct{ Object string; Count int }
type Doc struct {
	Body    string
	Raw     []byte
	Title   string
	Reader  io.Reader
	private string
}
type Sink interface{ Write(data []byte) (int, error) }
type Client struct{}
type val struct{}

func Query(ctx context.Context, query string, args ...any) error { return nil }
func Run(cmd string) error                                      { return nil }
func Open(path string, mode int) error                          { return nil }
func Fetch(rawURL string) error                                 { return nil }
func Render(html string) string                                 { return html }
func Join(a, b string) string                                   { return a + b }
func EscapeText(s string) string                                { return s }
func Quote(s string) (string, error)                            { return s, nil }
func Walk(root string, fn func(string)) error                   { return nil }
func Serve(h Handler)                                           { }
func Use(h HandlerFunc)                                         { }
func Print(code int, format string, values ...any)               {}
func Body() []byte                                              { return nil }
func Count() int                                                { return 0 }
func hidden(query string)                                       {}

func (c *Client) Bucket(name string) *Client                    { return c }
func (c *Client) Move(ctx context.Context, d Dest) error        { return nil }
func (c *Client) Content() string                               { return "" }
func (c *Client) Plain(s string) string                         { return s }
func (v val) Value(query string)                                {}
func (c Client) ByValue(sql string)                             {}
`

func scanSynth(t *testing.T) *API {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "synth.go", synth, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("example.com/synth", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return Scan(pkg, true)
}

func TestWords(t *testing.T) {
	for in, want := range map[string]string{
		"bucketName": "bucket name", "URLPath": "url path", "query": "query", "fileName2": "file name", "raw_sql": "raw sql", "HTMLBody": "html body",
	} {
		if got := strings.Join(words(in), " "); got != want {
			t.Errorf("words(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLayer1Rules(t *testing.T) {
	api := scanSynth(t)
	got := map[string]string{}
	for _, it := range api.Items {
		got[it.Kind+" "+it.Symbol] = strings.Join(it.Positions, ",")
	}
	want := map[string]string{
		"sink example.com/synth.Query":               "arg1",
		"sink example.com/synth.Run":                 "arg0",
		"sink example.com/synth.Open":                "arg0",
		"sink example.com/synth.Fetch":               "arg0",
		"sink example.com/synth.Print":               "arg1,arg2",
		"sink (example.com/synth.Sink).Write":        "arg0",
		"sink example.com/synth.Render":              "arg0",
		"calls example.com/synth.Walk":               "arg1",
		"calls example.com/synth.Serve":              "arg0",
		"calls example.com/synth.Use":                "arg0",
		"sanitizer example.com/synth.EscapeText":     "result",
		"sanitizer example.com/synth.Quote":          "result",
		"source example.com/synth.Body":              "result",
		"sink (*example.com/synth.Client).Bucket":    "arg0",
		"sink (*example.com/synth.Client).Move":      "arg1.Object",
		"source (*example.com/synth.Client).Content": "result",
		"sink (example.com/synth.Client).ByValue":    "arg0",
		"source example.com/synth.Doc.Body":          "field",
		"source example.com/synth.Doc.Raw":           "field",
		"source example.com/synth.Doc.Reader":        "field",
		"source example.com/synth.Doc.Title":         "field",
	}
	// Render also returns a string, but is not named like a sanitizer; Join and Plain have no name that a rule knows.
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: positions %q, want %q", k, got[k], v)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected candidate: %s (%s)", k, got[k])
		}
	}
	// the private function is not in the API, and a function that takes a string and is in no item is of layer 2
	if _, ok := api.Symbols["example.com/synth.hidden"]; ok {
		t.Error("an unexported function is in the API")
	}
	joined := strings.Join(api.Layer2, "\n")
	if !strings.Contains(joined, "example.com/synth.Join") || !strings.Contains(joined, "(*example.com/synth.Client).Plain") || strings.Contains(joined, "synth.Query") {
		t.Errorf("layer 2 = %v", api.Layer2)
	}
}

func TestDeterministic(t *testing.T) {
	a, b := scanSynth(t), scanSynth(t)
	if len(a.Items) != len(b.Items) {
		t.Fatal("the number of candidates changed")
	}
	for i := range a.Items {
		if a.Items[i].Symbol != b.Items[i].Symbol || a.Items[i].Kind != b.Items[i].Kind {
			t.Fatalf("the order changed at %d", i)
		}
	}
	for i := 1; i < len(a.Items); i++ {
		if a.Items[i-1].Symbol > a.Items[i].Symbol {
			t.Fatalf("not in the order of the symbol: %s, %s", a.Items[i-1].Symbol, a.Items[i].Symbol)
		}
	}
}

func writeModel(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "go-synth.yaml")
	if err := os.WriteFile(p, []byte("ruleset: t/synth@1\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMatchRowsRejectedAndStale(t *testing.T) {
	api := scanSynth(t)
	p := writeModel(t, `
sinks:
  - symbol: example.com/synth.Run
    args: [0]
    category: command_injection
sources:
  - kind: param_type
    type: "*example.com/synth.Client"
    labels: [user_input]
propagators:
  - symbol: example.com/synth.Quote
    flows: [{from: arg0, to: result}]
rejected:
  - symbol: example.com/synth.Open
    kind: sink
    why: "x"
  - symbol: example.com/synth.Walk
    kind: calls
    position: arg1
    why: "y"
  - symbol: example.com/synth.Gone
    kind: sink
    why: "the function was removed"
  - symbol: example.com/other.F
    kind: sink
    why: "another package"
`)
	f, err := LoadFacts([]string{p}, "example.com/synth")
	if err != nil {
		t.Fatal(err)
	}
	m := Match(api, f)
	has := func(list []Item, kind, sym string) bool {
		for _, it := range list {
			if it.Kind == kind && it.Symbol == sym {
				return true
			}
		}
		return false
	}
	if !has(m.ByRow, KindSink, "example.com/synth.Run") || !has(m.ByRow, KindSanitizer, "example.com/synth.Quote") {
		t.Errorf("rows: %+v", m.ByRow)
	}
	// the source row is of a type: the methods of the type are decided
	if !has(m.ByRow, KindSource, "(*example.com/synth.Client).Content") {
		t.Errorf("a param_type row decides the methods of the type: %+v", m.ByRow)
	}
	if !has(m.ByRejected, KindSink, "example.com/synth.Open") || !has(m.ByRejected, KindCalls, "example.com/synth.Walk") {
		t.Errorf("rejected: %+v", m.ByRejected)
	}
	// the entry of Walk names arg1, the one position of its calls candidate: decided; one that names another position would not
	if has(m.Undecided, KindCalls, "example.com/synth.Walk") || !has(m.Undecided, KindSink, "example.com/synth.Fetch") {
		t.Errorf("undecided: %+v", m.Undecided)
	}
	if len(m.Stale) != 1 || m.Stale[0].Symbol != "example.com/synth.Gone" {
		t.Errorf("stale = %+v (an entry of another package is not this package's)", m.Stale)
	}
	// the work file is a rejected: list whose `why` is empty: the loader refuses it until the reasons are written
	y := UndecidedYAML("example.com/synth", m.Undecided)
	q := filepath.Join(t.TempDir(), "u.yaml")
	os.WriteFile(q, []byte("ruleset: t@1\n"+y), 0o644)
	if _, err := LoadFacts([]string{q}, "example.com/synth"); err == nil || !strings.Contains(err.Error(), "why is required") {
		t.Errorf("the empty reasons must be refused: %v", err)
	}
}

func TestRejectedWithoutWhyIsALoadError(t *testing.T) {
	p := writeModel(t, "rejected:\n  - symbol: a.B\n    kind: sink\n")
	if _, err := LoadFacts([]string{p}, "a"); err == nil {
		t.Fatal("no error")
	}
	var out, errb bytes.Buffer
	if code := realMain([]string{"-pkg", "html", "-model", p, "-work", t.TempDir()}, &out, &errb); code != ExitUsage {
		t.Errorf("exit %d, want %d (%s)", code, ExitUsage, errb.String())
	}
}

// The completion criterion, on a package of the standard library (no fetch): undecided -> 70 and a list; the list written as rejected: -> 0
// and the rows of the model are the same.
func TestExitCodes(t *testing.T) {
	work := t.TempDir()
	rows := "sinks:\n  - symbol: html.UnescapeString\n    args: [0]\n    category: xss\n"
	p := writeModel(t, rows)
	run := func(args ...string) (int, string, string) {
		var out, errb bytes.Buffer
		code := realMain(append([]string{"-pkg", "html", "-model", p, "-work", work}, args...), &out, &errb)
		return code, out.String(), errb.String()
	}
	code, out, _ := run()
	if code != ExitUndecided {
		t.Fatalf("exit %d, want %d\n%s", code, ExitUndecided, out)
	}
	list := filepath.Join(work, "html-undecided.yaml")
	data, err := os.ReadFile(list)
	if err != nil || !strings.Contains(string(data), "symbol: html.EscapeString") {
		t.Fatalf("the undecided list: %v\n%s", err, data)
	}
	if !strings.Contains(out, "layer 2:") || !strings.Contains(out, "layer 1:") {
		t.Errorf("the report: %s", out)
	}
	// write the reasons, and put the list into the model file
	filled := strings.ReplaceAll(string(data), `why: ""`, `why: "a reason"`)
	text, _ := os.ReadFile(p)
	filled = strings.Replace(filled, "rejected:\n", "", 1)
	before := string(text)
	os.WriteFile(p, []byte(before+"rejected:\n"+filled), 0o644)
	code, out, errs := run()
	if code != ExitOK {
		t.Fatalf("exit %d, want 0\n%s%s", code, out, errs)
	}
	// the rows are the same: the file with `rejected:` cut off is what it was
	after, _ := os.ReadFile(p)
	if !strings.HasPrefix(string(after), before) {
		t.Error("the rows changed")
	}
	// a stale entry is a warning and not a failure
	os.WriteFile(p, []byte(string(after)+"  - symbol: html.Gone\n    kind: sink\n    why: \"gone\"\n"), 0o644)
	code, out, _ = run()
	if code != ExitOK || !strings.Contains(out, "warning:") || !strings.Contains(out, "html.Gone") {
		t.Errorf("stale: exit %d\n%s", code, out)
	}
	// usage
	var o, e bytes.Buffer
	if c := realMain([]string{"-pkg", "html", "-lang", "dart"}, &o, &e); c != ExitUsage {
		t.Errorf("-lang dart: exit %d", c)
	}
	if c := realMain(nil, &o, &e); c != ExitUsage {
		t.Errorf("no -pkg: exit %d", c)
	}
	// a package that cannot be fetched
	if c := realMain([]string{"-pkg", "example.invalid/nothing", "-model", p, "-work", work}, &o, &e); c != ExitFetch {
		t.Errorf("fetch failure: exit %d (%s)", c, e.String())
	}
}

// Recall on rows that are in the repository and need no fetch: the rows of database/sql in go-stdlib.yaml.
func TestRecallOnDatabaseSQL(t *testing.T) {
	repo := filepath.Join("..", "..")
	files, err := BuiltinFiles(repo)
	if err != nil || len(files) == 0 {
		t.Fatal(err)
	}
	f, err := LoadFacts(files, "database/sql")
	if err != nil {
		t.Fatal(err)
	}
	l, err := FetchAndLoad(t.TempDir(), "database/sql", "")
	if err != nil {
		t.Fatal(err)
	}
	r := ComputeRecall(Scan(l.Types, true), f)
	if r.SinkRows < 10 || r.SinkFound != r.SinkRows {
		t.Errorf("layer 1 must find the sink rows of database/sql: %+v", r)
	}
}

func TestUpdateVerified(t *testing.T) {
	const day = "2026-11-24"
	got, err := UpdateVerified("ruleset: x@1\n\nsinks:\n  - symbol: a.B\n", "example.com/m", "v1.4.0", day)
	if err != nil || !strings.Contains(got, "verified_versions:\n  - package: example.com/m\n    versions: \">=1.4.0 <2.0.0\"\n    checked_with: v1.4.0\n    checked: "+day+"\n\nsinks:") {
		t.Errorf("no list: %v\n%s", err, got)
	}
	old := "ruleset: x@1\n\nverified_versions:\n  - package: example.com/m\n    versions: \">=1.0.0 <2.0.0\"\n    checked_with: v1.0.0\n    checked: 2026-01-01\n  - package: example.com/n\n    versions: v2.0.0\n    checked: 2026-01-02\n\nsinks:\n"
	// a version that the record covers: nothing changes
	if got, _ = UpdateVerified(old, "example.com/m", "v1.5.0", day); got != old {
		t.Errorf("a covered version changed the file:\n%s", got)
	}
	got, _ = UpdateVerified(old, "example.com/m", "v2.0.1", day)
	if strings.Count(got, "package: example.com/m") != 1 || !strings.Contains(got, "checked_with: v2.0.1") || !strings.Contains(got, "package: example.com/n") || strings.Contains(got, "v1.0.0") {
		t.Errorf("replace:\n%s", got)
	}
	got, _ = UpdateVerified(old, "example.com/o", "v0.3.2", day)
	if !strings.Contains(got, `">=0.3.2 <0.4.0"`) || !strings.Contains(got, "package: example.com/m") || !strings.Contains(got, "package: example.com/n") {
		t.Errorf("add:\n%s", got)
	}
	// the result is read by the loader
	p := writeModel(t, "")
	if txt, _ := os.ReadFile(p); true {
		upd, _ := UpdateVerified(string(txt), "example.com/m", "v1.2.3", "2026-11-01")
		os.WriteFile(p, []byte(upd), 0o644)
		if _, err := LoadFacts([]string{p}, "example.com/m"); err != nil {
			t.Error(err)
		}
	}
	if _, err := UpdateVerified("", "m", "latest", day); err == nil {
		t.Error("a version that is not one is refused")
	}
}

func TestDeadAllow(t *testing.T) {
	log := "=== RUN\n    --- FAIL\n    x_test.go:1: 3 rows of go are not hit (the allow file lists the ones that may stay):\n        go a.B | \n        go (*a.T).C | \n        go z.Y | \n"
	dead := ParseDeadRows(log)
	if len(dead) != 3 {
		t.Fatalf("%v", dead)
	}
	lines := DeadAllowLines(dead, "a", "# header\n(*a.T).C | already\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "a.B | dependency: ") {
		t.Errorf("%v", lines)
	}
}

func TestNoCallsCandidatesWhenBodiesAreRead(t *testing.T) {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, "synth.go", synth, 0)
	pkg, err := (&types.Config{Importer: importer.ForCompiler(fset, "source", nil)}).Check("example.com/synth", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range Scan(pkg, false).Items {
		if it.Kind == KindCalls {
			t.Errorf("a calls candidate for a package whose bodies are read: %s", it.Symbol)
		}
	}
	p := writeModel(t, "body_replaced_packages:\n  - package: example.com/declared\n    reason: x\n  - package: example.com/tree/...\n    reason: x\n")
	facts, err := LoadFacts([]string{p}, "example.com/synth")
	if err != nil {
		t.Fatal(err)
	}
	for pk, want := range map[string]bool{"example.com/synth": true, "example.com/declared": false, "example.com/tree/sub": false, "example.com/declaredx": true, "strings": false} {
		if got := facts.BodiesRead(pk); got != want {
			t.Errorf("BodiesRead(%s) = %v", pk, got)
		}
	}
}

// Two majors of one module: a row or an entry written with /v* is read for the major that is checked, and what only the other major
// has is not a vanished candidate. The records of verified_versions of two majors stand side by side, and a write adds one only.
func TestMajors(t *testing.T) {
	p := writeModel(t, `
verified_versions:
  - package: example.com/m/v2
    versions: ">=2.1.0 <3.0.0"
    checked_with: v2.1.0
    checked: 2026-11-01
sinks:
  - symbol: example.com/m/v*/pkg.Run
    args: [0]
    category: command_injection
rejected:
  - symbol: example.com/m/v*/pkg.OnlyInV3
    kind: sink
    why: "only a later major has it"
  - symbol: example.com/m/v2/pkg.OnlyInV2
    kind: sink
    why: "x"
`)
	f2, err := LoadFacts([]string{p}, "example.com/m/v2/pkg")
	if err != nil {
		t.Fatal(err)
	}
	if f2.Sinks["example.com/m/v2/pkg.Run"] == "" {
		t.Errorf("the row with /v* is not read for v2: %v", f2.Sinks)
	}
	api := &API{Symbols: map[string]string{"example.com/m/v2/pkg.Run": "", "example.com/m/v2/pkg.OnlyInV2": ""}}
	if m := Match(api, f2); len(m.Stale) != 0 {
		t.Errorf("stale in v2: %+v", m.Stale)
	}
	// the run of v3: the entry of v2 is not of this package, the entry with /v* is not stale though v3 has no OnlyInV3 here
	f3, _ := LoadFacts([]string{p}, "example.com/m/v3/pkg")
	api3 := &API{Symbols: map[string]string{"example.com/m/v3/pkg.Run": ""}}
	if m := Match(api3, f3); len(m.Stale) != 0 || len(f3.Rejected) != 1 {
		t.Errorf("stale in v3: %+v, rejected %+v", m.Stale, f3.Rejected)
	}
	// the v3 run adds its own record and keeps the v2 one
	txt, _ := os.ReadFile(p)
	upd, err := UpdateVerified(string(txt), "example.com/m/v3", "v3.0.4", "2026-11-02")
	if err != nil || !strings.Contains(upd, "package: example.com/m/v2") || !strings.Contains(upd, "checked_with: v2.1.0") || !strings.Contains(upd, "package: example.com/m/v3") || !strings.Contains(upd, `">=3.0.4 <4.0.0"`) {
		t.Fatalf("%v\n%s", err, upd)
	}
	os.WriteFile(p, []byte(upd), 0o644)
	if _, err := LoadFacts([]string{p}, "example.com/m/v3/pkg"); err != nil {
		t.Errorf("the loader refuses the two records: %v", err)
	}
	// a rerun for v2 at a version that its record covers changes nothing; at another one it replaces its record only
	if same, _ := UpdateVerified(upd, "example.com/m/v2", "v2.2.0", "2026-11-03"); same != upd {
		t.Errorf("a covered version changed the file:\n%s", same)
	}
	upd2, _ := UpdateVerified(strings.Replace(upd, ">=2.1.0 <3.0.0", ">=2.1.0 <2.2.0", 1), "example.com/m/v2", "v2.2.0", "2026-11-03")
	if strings.Count(upd2, "package: example.com/m/v2") != 1 || !strings.Contains(upd2, "checked_with: v3.0.4") || !strings.Contains(upd2, "checked_with: v2.2.0") {
		t.Errorf("%s", upd2)
	}
	// a /v* in -pkg needs the version
	var o, e bytes.Buffer
	if c := realMain([]string{"-pkg", "example.com/m/v*/pkg", "-model", p, "-work", t.TempDir()}, &o, &e); c != ExitUsage {
		t.Errorf("/v* without -version: exit %d", c)
	}
	if m, _ := majorOfVersion("v93.0.1"); m != "93" || majorOf("example.com/m/v93/pkg") != "93" || majorOf("example.com/m/pkg") != "" {
		t.Error("major")
	}
}

func TestRowsNoRuleFinds(t *testing.T) {
	api := scanSynth(t)
	p := writeModel(t, "sinks:\n  - symbol: example.com/synth.Join\n    args: [0]\n    category: path_traversal\n  - symbol: example.com/synth.Run\n    args: [0]\n    category: command_injection\n")
	f, err := LoadFacts([]string{p}, "example.com/synth")
	if err != nil {
		t.Fatal(err)
	}
	r := ComputeRecall(api, f)
	if r.SinkRows != 2 || r.SinkFound != 1 || len(r.SinkMissed) != 1 || r.SinkMissed[0] != "example.com/synth.Join" {
		t.Errorf("%+v", r)
	}
}
