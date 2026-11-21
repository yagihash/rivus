package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// A Dart file with a syntax error is read on the tree that the analyzer recovers (a declaration may be missing or
// made up): the result says so. The file is in summary.unread; a finding that goes through a function of the file
// has a missing_source note, so it is not a clear one. A project with no syntax error is as it was.
func TestDartSyntaxErrorIsReported(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	project := func(files map[string]string) string {
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
		write("rules.yaml", "ruleset: t/dx@1\nsources:\n  - symbol: package:dx/lib.dart#source\n    labels: [user_input]\nsinks:\n  - symbol: package:dx/lib.dart#sink\n    category: test\n")
		write("lib/lib.dart", "String source() => 'x';\nvoid sink(String s) {}\n")
		for n, c := range files {
			write("lib/"+n, c)
		}
		get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get")...)
		get.Dir = dir
		if out, err := get.CombinedOutput(); err != nil {
			t.Fatalf("dart pub get: %v\n%s", err, out)
		}
		return dir
	}
	type result struct {
		Summary struct {
			Unread *struct {
				Packages int `json:"packages"`
				Top      []struct {
					Package string `json:"package"`
					Why     string `json:"why"`
				} `json:"top"`
			} `json:"unread"`
		} `json:"summary"`
		Findings []struct {
			Source        struct{ Func string } `json:"source"`
			Confidence    string                `json:"confidence"`
			Uncertainties []struct {
				Kind   string `json:"kind"`
				Detail string `json:"detail"`
			} `json:"uncertainties"`
		} `json:"findings"`
	}
	run := func(dir string) result {
		cmd := exec.Command(binary, "analyze", "-rules", "rules.yaml", "./...")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		var r result
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	flow := func(name string) string { return "void " + name + "() { final a = source(); sink(a); }\n" }

	bad := run(project(map[string]string{
		"ok.dart":       "import 'lib.dart';\n" + flow("okFlow"),
		"synbody.dart":  "import 'lib.dart';\nvoid synBody() {\n  final a = source();\n  var b = ;\n  sink(a);\n}\n",
		"synclass.dart": "import 'lib.dart';\nclass K {\n  void m() { final a = source(); sink(a); }\n  int x = ;\n}\n",
		"synfile.dart":  "import 'lib.dart';\n" + flow("before") + "}}} garbage ((\n" + flow("after"),
	}))
	if bad.Summary.Unread == nil || bad.Summary.Unread.Packages != 3 {
		t.Fatalf("summary.unread: %+v", bad.Summary.Unread)
	}
	// the extractor and the core agree on the prefix of the message (ir.SyntaxErrorPrefix)
	for _, u := range bad.Summary.Unread.Top {
		if !strings.HasPrefix(u.Why, ir.SyntaxErrorPrefix) {
			t.Errorf("the message of %s does not start with %q: %q", u.Package, ir.SyntaxErrorPrefix, u.Why)
		}
	}
	for _, f := range bad.Findings {
		inBad := !strings.HasSuffix(f.Source.Func, "#okFlow")
		note := false
		for _, u := range f.Uncertainties {
			note = note || (u.Kind == "missing_source" && strings.Contains(u.Detail, "syntax_error"))
		}
		switch {
		case inBad && (!note || f.Confidence == "no_uncertainty_on_path"):
			t.Errorf("a finding of a file with a syntax error has no note of it: %+v", f)
		case !inBad && (note || f.Confidence != "no_uncertainty_on_path"):
			t.Errorf("a finding of a clean file is not clear: %+v", f)
		}
	}

	good := run(project(map[string]string{"ok.dart": "import 'lib.dart';\n" + flow("okFlow")}))
	if good.Summary.Unread != nil || len(good.Findings) != 1 || good.Findings[0].Confidence != "no_uncertainty_on_path" {
		t.Errorf("a project with no syntax error changed: unread %+v, findings %+v", good.Summary.Unread, good.Findings)
	}
}

// A constructor that does not resolve (a compile error): what it makes is taken to come from its arguments, and the
// construct is said (unsupported_construct). It used to return a constant with no taint, and the flow went out of
// the result without a trace.
func TestDartUnresolvedConstructorKeepsTheFlow(t *testing.T) {
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
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("pubspec.yaml", "name: dx\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("rules.yaml", "ruleset: t/dx@1\nsources:\n  - symbol: package:dx/lib.dart#source\n    labels: [user_input]\nsinks:\n  - symbol: package:dx/lib.dart#sink\n    category: test\n")
	write("lib/lib.dart", "String source() => 'x';\nvoid sink(String s) {}\n")
	write("lib/c.dart", "import 'lib.dart';\nclass Box { Box(this.v); final String v; }\nvoid made() { final a = source(); final b = new Box.nope2(a); sink('$b'); }\n")
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
	var r struct {
		Findings []struct {
			Confidence string `json:"confidence"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Findings) != 1 || r.Findings[0].Confidence == "no_uncertainty_on_path" {
		t.Errorf("the flow through a constructor that does not resolve: %d findings, %+v", len(r.Findings), r.Findings)
	}
}

// A compile error in a function: a finding that goes through the function carries it (missing_source), also when the
// flow is found by another way; a finding through a function of the same file with no error is a clear one.
func TestDartCompileErrorIsOnTheFindingsOfItsFunction(t *testing.T) {
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
	write("rules.yaml", "ruleset: t/dx@1\nsources:\n  - symbol: package:dx/lib.dart#source\n    labels: [user_input]\nsinks:\n  - symbol: package:dx/lib.dart#sink\n    category: test\n")
	write("lib/lib.dart", "String source() => 'x';\nvoid sink(String s) {}\n")
	write("lib/c.dart", "import 'lib.dart';\nclass Box { Box(this.v); final String v; }\n"+
		"void broken() { final a = source(); final b = Box.nope(a); sink(b.v); sink(a); }\n"+
		"void fine() { final a = source(); sink(a); }\n"+
		"void viaClosure() { final f = () { final a = source(); var z = 1 + 'x'; sink(a); }; f(); }\n")
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
	var r struct {
		Findings []struct {
			Source        struct{ Func string }   `json:"source"`
			Confidence    string                  `json:"confidence"`
			Uncertainties []struct{ Kind string } `json:"uncertainties"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range r.Findings {
		name := f.Source.Func[strings.LastIndex(f.Source.Func, "#")+1:]
		if i := strings.Index(name, "$"); i >= 0 {
			name = name[:i]
		}
		seen[name] = true
		note := false
		for _, u := range f.Uncertainties {
			note = note || u.Kind == "missing_source"
		}
		switch name {
		case "fine":
			if note || f.Confidence != "no_uncertainty_on_path" {
				t.Errorf("a function with no error is not clear: %+v", f)
			}
		default:
			if !note || f.Confidence == "no_uncertainty_on_path" {
				t.Errorf("%s (a compile error in it): no missing_source on the finding: %+v", name, f)
			}
		}
	}
	for _, want := range []string{"broken", "fine", "viaClosure"} {
		if !seen[want] {
			t.Errorf("no finding of %s", want)
		}
	}
}
