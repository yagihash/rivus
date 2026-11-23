package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureAPI reads the API of the fake package testdata/dartpkg with the analyzer program. It needs `dart` and the analyzer in the
// pub cache (no network: the reader is run with --offline); without them the test is skipped.
func fixtureAPI(t *testing.T) *dartAPIJSON {
	t.Helper()
	if _, err := exec.LookPath("dart"); err != nil {
		t.Skip("dart is not installed")
	}
	abs, _ := filepath.Abs("testdata/dartpkg")
	dir := t.TempDir()
	pub := "name: fx\npublish_to: none\nenvironment:\n  sdk: ^3.8.0\ndependencies:\n  fixpkg:\n    path: " + abs + "\n"
	if _, err := dartPubGet(dir, "fixpkg", pub); err != nil {
		t.Skipf("cannot make the project: %v", err)
	}
	repo, _ := filepath.Abs("../..")
	a, err := DartReadAPI(helperDir(repo), dir, "fixpkg", nil, false)
	if err != nil {
		t.Skipf("the API reader cannot run offline: %v", err)
	}
	return a
}

func dartGot(api *API) map[string]string {
	got := map[string]string{}
	for _, it := range api.Items {
		got[it.Kind+" "+it.Symbol] = strings.Join(it.Positions, ",")
	}
	return got
}

func TestDartLayer1Rules(t *testing.T) {
	a := fixtureAPI(t)
	api := ScanDart(a, "package:fixpkg", true)
	got := dartGot(api)
	const c = "package:fixpkg/src/client.dart#"
	const e = "package:fixpkg/src/escape.dart#"
	want := map[string]string{
		"sink " + c + "Client.new":       "arg0",
		"sink " + c + "Client.fromUrl":   "arg0",
		"sink " + c + "Client.get":       "arg0,named:savePath",
		"sink " + c + "Client.run":       "arg0",
		"sink " + c + "Client.open":      "arg0",
		"sink " + c + "Client.sql":       "arg0",
		"sink " + c + "Client.move":      "arg0.bucketName",
		"sink " + c + "Client.bucket":    "arg0",
		"sink " + c + "Client.baseUrl":   "arg0",
		"sink " + c + "fetchDocument":    "arg0",
		"source " + c + "Client.content": "result",
		"source " + c + "Client.headers": "result",
		"source " + c + "Document.body":  "field",
		"source " + c + "Document.title": "field",
		"source " + c + "Document.raw":   "field",
		"source " + c + "fetchDocument":  "",
		"calls " + c + "Client.listen":   "arg0,arg1",
		"sanitizer " + e + "escapeText":  "result",
		"sanitizer " + e + "quoteName":   "result",
	}
	delete(want, "source "+c+"fetchDocument") // Document is not a carrier of text: not a candidate
	for k, v := range want {
		if g, ok := got[k]; !ok || g != v {
			t.Errorf("%s: positions %q (found %v), want %q", k, g, ok, v)
		}
	}
	for k, v := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected candidate: %s (%s)", k, v)
		}
	}
	// a constant, a private member and a function that the package does not export are not candidates; `join` is of layer 2 only if it were exported
	for k := range got {
		if strings.Contains(k, "contentTypeHeader") || strings.Contains(k, "_private") || strings.Contains(k, "_hidden") {
			t.Errorf("%s should not be a candidate", k)
		}
	}
	if _, ok := api.Symbols[e+"join"]; ok {
		t.Error("join is not exported by fixpkg.dart (show escapeText, quoteName)")
	}
	l2 := strings.Join(api.Layer2, "\n")
	if !strings.Contains(l2, c+"Client.plain") || strings.Contains(l2, "Client.get") {
		t.Errorf("layer 2 = %v", api.Layer2)
	}
	// without calls candidates (the bodies of the package are read), no `calls`
	for _, it := range ScanDart(a, "package:fixpkg", false).Items {
		if it.Kind == KindCalls {
			t.Errorf("calls candidate listed for a package whose bodies are read: %s", it.Symbol)
		}
	}
}

func TestDartMatchAndRejected(t *testing.T) {
	a := fixtureAPI(t)
	api := ScanDart(a, "package:fixpkg", false)
	dir := t.TempDir()
	model := filepath.Join(dir, "dart-fixpkg.yaml")
	text := `ruleset: rivus/dart/fixpkg@1
sinks:
  - symbol: package:fixpkg/src/client.dart#Client.run
    args: [0]
    category: command_injection
  - symbol: package:fixpkg/src/client.dart#Client.get
    args: [0]
    category: ssrf
sources:
  - symbol: package:fixpkg/src/client.dart#Client.content
    labels: [user_input]
  - kind: field
    field: package:fixpkg/src/client.dart#Document.body
    labels: [user_input]
rejected:
  - symbol: package:fixpkg/src/client.dart#Client.open
    kind: sink
    why: "test"
  - symbol: package:fixpkg/src/client.dart#Client.gone
    kind: sink
    why: "the method was removed"
`
	if err := os.WriteFile(model, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := LoadFacts([]string{model}, "package:fixpkg")
	if err != nil {
		t.Fatal(err)
	}
	if f.BodiesRead("package:fixpkg") != true {
		t.Error("a Dart package without body_replaced_packages has its bodies read")
	}
	m := Match(api, f)
	byRow := map[string]bool{}
	for _, it := range m.ByRow {
		byRow[it.Kind+" "+it.Symbol] = true
	}
	for _, k := range []string{"sink Client.run", "sink Client.get", "source Client.content", "source Document.body"} {
		if !byRow[strings.Replace(k, " ", " package:fixpkg/src/client.dart#", 1)] {
			t.Errorf("%s should have a row", k)
		}
	}
	if len(m.ByRejected) != 1 || !strings.HasSuffix(m.ByRejected[0].Symbol, "Client.open") {
		t.Errorf("rejected = %v", m.ByRejected)
	}
	if len(m.Stale) != 1 || !strings.HasSuffix(m.Stale[0].Symbol, "Client.gone") {
		t.Errorf("stale = %v", m.Stale)
	}
	if len(m.Undecided) == 0 {
		t.Error("the candidates without a row or a rejected entry are undecided")
	}
	r := ComputeRecall(api, f)
	if r.SinkRows != 2 || r.SinkFound != 2 || r.SinkPosFound != 2 || r.SourceRows != 2 || r.SourceFound != 2 {
		t.Errorf("recall = %+v", r)
	}
}

func TestDartPackageName(t *testing.T) {
	for in, want := range map[string]string{"dio": "dio", "package:dio": "dio", "package:dio/dio.dart": "dio"} {
		if got := dartName(in); got != want {
			t.Errorf("dartName(%q) = %q, want %q", in, got, want)
		}
	}
	if !inPackage("package:dio/src/dio.dart#Dio.get", "package:dio") || inPackage("package:dio_foo/a.dart#A.b", "package:dio") {
		t.Error("inPackage mixes dio and dio_foo")
	}
	if got := receiverType("package:a/b.dart#Type.member"); got != "package:a/b.dart#Type" {
		t.Errorf("receiverType = %q", got)
	}
}

func TestDartHooksStop(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "hooky")
	if err := os.MkdirAll(filepath.Join(pkg, "hook"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "hook", "build.dart"), []byte("void main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".dart_tool"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"configVersion":2,"packages":[{"name":"hooky","rootUri":"` + "file://" + pkg + `","packageUri":"lib/"},{"name":"fx","rootUri":"../","packageUri":"lib/"}]}`
	if err := os.WriteFile(filepath.Join(dir, ".dart_tool", "package_config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DartHooks(dir)
	if len(got) != 1 || got[0] != "hooky/hook/build.dart" {
		t.Errorf("hooks = %v", got)
	}
}

func TestDartDeadRowsAndVerified(t *testing.T) {
	log := "    dead_rows_test.go:1: 2 rows of dart are not hit:\n    dart package:dio/src/dio.dart#Dio.get | \n    go example.com/x.F | \n"
	got := ParseDeadRows(log)
	if len(got) != 2 {
		t.Fatalf("dead rows = %v", got)
	}
	lines := DeadAllowLines(got, "package:dio", "")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "package:dio/src/dio.dart#Dio.get | dependency:") {
		t.Errorf("lines = %v", lines)
	}
	text := "ruleset: x@1\nverified_versions:\n  - package: dio\n    versions: \">=5.0.0 <6.0.0\"\n    checked_with: 5.0.0\n    checked: 2026-01-01\n\nsinks:\n"
	upd, err := UpdateVerified(text, "dio", "5.11.1", "2026-11-23")
	if err != nil || !strings.Contains(upd, "checked_with: 5.11.1") || strings.Contains(upd, "5.0.0\n") {
		t.Errorf("update = %q, %v", upd, err)
	}
}
