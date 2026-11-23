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

// A sanitizer hides the value from the sinks of the categories in `neutralises` and from no others, and the value that goes on keeps
// the label that `adds` gives, with the sanitizer on its path (ADR-115). A method sanitizer takes what it is called on as data.
const scopeLib = `package lib

func Source() string { return "" }
func SinkHTML(s string)  {}
func SinkCmd(s string)   {}
func Escape(s string) string { return s }
func Encode(s string) string { return s }

type Text string

func (t Text) Clean() string { return string(t) }
`

const scopeRules = `
sources:
  - symbol: example.com/tmp/lib.Source
    labels: [user_input]
sinks:
  - symbol: example.com/tmp/lib.SinkHTML
    category: xss
  - symbol: example.com/tmp/lib.SinkCmd
    category: command_injection
sanitizers:
  - symbol: example.com/tmp/lib.Escape
    drops: [user_input]
    adds: [html_escaped]
    neutralises: [xss]
  - symbol: example.com/tmp/lib.Encode
    drops: [user_input]
    adds: [url_encoded]
    neutralises: []
  - symbol: (example.com/tmp/lib.Text).Clean
    drops: [other]
`

const scopeMain = `package main

import "example.com/tmp/lib"

func EscapeToHTML()  { lib.SinkHTML(lib.Escape(lib.Source())) }
func EscapeToCmd()   { lib.SinkCmd(lib.Escape(lib.Source())) }
func EncodeToHTML()  { lib.SinkHTML(lib.Encode(lib.Source())) }
func MethodKeeps()   { lib.SinkCmd(lib.Text(lib.Source()).Clean()) }
`

func TestSanitizerHidesTheValueFromItsOwnCategoryOnly(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	w.write("lib/lib.go", scopeLib)
	w.write("rules.yaml", scopeRules)
	w.write("main/main.go", scopeMain)
	var rep report.Report
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "./main"), &rep); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range rep.Findings {
		fn := f.Sink.Func
		if i := strings.LastIndex(fn, "."); i >= 0 {
			fn = fn[i+1:]
		}
		got[fn] = strings.Join(f.Labels, ",")
		if fn == "EncodeToHTML" {
			var steps []string
			for _, p := range f.Paths {
				for _, s := range p.Steps {
					steps = append(steps, s.Desc)
				}
			}
			if !strings.Contains(strings.Join(steps, "|"), "sanitized by example.com/tmp/lib.Encode") {
				t.Errorf("the path of EncodeToHTML does not show the sanitizer: %v", steps)
			}
		}
	}
	want := map[string]string{
		"EscapeToCmd":  "html_escaped",
		"EncodeToHTML": "url_encoded",
		"MethodKeeps":  "user_input",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: labels %q, want %q (findings: %v)", k, got[k], v, got)
		}
	}
	if _, ok := got["EscapeToHTML"]; ok {
		t.Errorf("EscapeToHTML: the HTML escape is for xss, and there is a finding: %v", got)
	}
	if len(got) != len(want) {
		t.Errorf("findings: %v", got)
	}
}

// The sanitizers of the built-in model say which kind of sink they are safe for (ADR-115): an HTML escape of Go and of Dart is safe for
// xss and for no other, percent encoding is safe for none (and the value says that it was encoded), and Uri.encodeFull is no sanitizer.
func TestBuiltinSanitizersAreScopedGo(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	w.write("lib/lib.go", "package lib\n\nfunc Source() string { return \"\" }\nfunc SinkHTML(s string) {}\nfunc SinkCmd(s string)  {}\n")
	w.write("rules.yaml", `
sources:
  - symbol: example.com/tmp/lib.Source
    labels: [user_input]
sinks:
  - symbol: example.com/tmp/lib.SinkHTML
    category: xss
  - symbol: example.com/tmp/lib.SinkCmd
    category: command_injection
`)
	w.write("main/main.go", `package main

import (
	"html"
	"html/template"

	"example.com/tmp/lib"
)

func EscapeToHTML()  { lib.SinkHTML(html.EscapeString(lib.Source())) }
func EscapeToCmd()   { lib.SinkCmd(html.EscapeString(lib.Source())) }
func HTMLToHTML()    { lib.SinkHTML(template.HTMLEscapeString(lib.Source())) }
func HTMLToCmd()     { lib.SinkCmd(template.HTMLEscapeString(lib.Source())) }
func JSToHTML()      { lib.SinkHTML(template.JSEscapeString(lib.Source())) }
func JSToCmd()       { lib.SinkCmd(template.JSEscapeString(lib.Source())) }
`)
	var rep report.Report
	if err := json.Unmarshal(w.run("analyze", "-rules", "rules.yaml", "./main"), &rep); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range rep.Findings {
		fn := f.Sink.Func
		got[fn[strings.LastIndex(fn, ".")+1:]] = strings.Join(f.Labels, ",")
	}
	want := map[string]string{"EscapeToCmd": "html_escaped", "HTMLToCmd": "html_escaped", "JSToCmd": "js_escaped"}
	if len(got) != len(want) {
		t.Errorf("findings %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: labels %q, want %q (%v)", k, got[k], v, got)
		}
	}
}

func TestBuiltinSanitizersAreScopedDart(t *testing.T) {
	t.Parallel()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pubspec.yaml", "name: dx\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("rules.yaml", `ruleset: t/dx@1
sources:
  - symbol: package:dx/lib.dart#source
    labels: [user_input]
sinks:
  - symbol: package:dx/lib.dart#sinkHtml
    category: xss
  - symbol: package:dx/lib.dart#sinkCmd
    category: command_injection
`)
	write("lib/lib.dart", "String source() => 'x';\nvoid sinkHtml(String s) {}\nvoid sinkCmd(String s) {}\n")
	write("lib/c.dart", `import 'dart:convert';
import 'lib.dart';
void escapeToHtml() { sinkHtml(const HtmlEscape().convert(source())); }
void escapeToCmd() { sinkCmd(const HtmlEscape().convert(source())); }
void componentToHtml() { sinkHtml(Uri.encodeComponent(source())); }
void queryToCmd() { sinkCmd(Uri.encodeQueryComponent(source())); }
void fullToCmd() { sinkCmd(Uri.encodeFull(source())); }
`)
	get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get")...)
	get.Dir = dir
	if out, err := get.CombinedOutput(); err != nil {
		t.Fatalf("dart pub get: %v\n%s", err, out)
	}
	cmd := exec.Command(binary, "analyze", "-rules", "rules.yaml", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range rep.Findings {
		got[f.Source.Func[strings.LastIndex(f.Source.Func, "#")+1:]] = strings.Join(f.Labels, ",")
	}
	want := map[string]string{"escapeToCmd": "html_escaped", "componentToHtml": "url_encoded", "queryToCmd": "url_encoded", "fullToCmd": "user_input"}
	if len(got) != len(want) {
		t.Errorf("findings %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: labels %q, want %q (%v)", k, got[k], v, got)
		}
	}
}
