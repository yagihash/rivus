package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A function value of the project that is given to a function of a dependency is not carried through the summary of the
// dependency (docs/issues/210, stage 1): the call, in the code of the project, says func_value_into_dependency, and the
// finding that goes through it carries it. Not so for a call that has no function value in its arguments, for a function
// that has a `calls:` model (a function of the SDK among them), and for a package whose body is not read (its callbacks
// are applied as called: assumed_call, and no second note).
func TestDartFuncValueIntoDependency(t *testing.T) {
	t.Parallel()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write := func(name, text string) {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// the dependency is in a pub cache (a package that is read as a dependency: its functions are Dep), laid out by hand: no
	// network, and no `pub get`
	cache := filepath.Join(root, "pubcache")
	depDir := filepath.Join(cache, "hosted", "pub.dev", "dep-1.0.0")
	write("pubcache/hosted/pub.dev/dep-1.0.0/pubspec.yaml", "name: dep\nversion: 1.0.0\nenvironment:\n  sdk: ^3.10.0\n")
	write("pubcache/hosted/pub.dev/dep-1.0.0/lib/dep.dart", `String run(String Function(String) f, String v) => f(v);
String plain(String v) => v;
String Function(String) wrap(String Function(String) f) => (x) => f(x);
`)
	write("app/pubspec.yaml", "name: fv\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\ndependencies:\n  dep: ^1.0.0\n")
	write("app/.dart_tool/package_config.json", `{"configVersion":2,"packages":[{"name":"dep","rootUri":"file://`+filepath.ToSlash(depDir)+`","packageUri":"lib/","languageVersion":"3.10"},{"name":"fv","rootUri":"../","packageUri":"lib/","languageVersion":"3.10"}],"generator":"pub","generatorVersion":"3.13.7"}`)
	write("app/rules.yaml", `ruleset: t/fv@1
sources:
  - symbol: package:fv/lib.dart#source
    labels: [user_input]
sinks:
  - symbol: package:fv/lib.dart#sink
    args: [0]
    category: cmd
`)
	write("app/lib/lib.dart", `import 'package:dep/dep.dart' as dep;
String source() => 'x';
void sink(Object o) {}
void intoDependency() { sink(dep.run((s) => s, source())); }
void returnedBack() { final g = dep.wrap((s) => s); sink(g(source())); }
void noFunction() { sink(dep.plain(source())); }
void sdk() { final l = [source()]; l.forEach((s) { sink(s); }); }
void own() { final f = (String s) => s; sink(f(source())); }
`)
	run := func(extra ...string) []byte {
		args := append([]string{"analyze", "-rules", "rules.yaml"}, extra...)
		cmd := exec.Command(binary, append(args, "./...")...)
		cmd.Dir = filepath.Join(root, "app")
		cmd.Env = append(os.Environ(), "PUB_CACHE="+cache, "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		return out
	}
	type report struct {
		Findings []struct {
			Source        struct{ Func string } `json:"source"`
			Uncertainties []struct {
				Kind string `json:"kind"`
			} `json:"uncertainties"`
		} `json:"findings"`
		Uncertainties []struct {
			Kind           string `json:"kind"`
			AttributableTo string `json:"attributable_to"`
			Func           string `json:"func"`
		} `json:"uncertainties"`
	}
	short := func(s string) string { return s[strings.LastIndex(s, "#")+1:] }
	var r report
	if err := json.Unmarshal(run(), &r); err != nil {
		t.Fatal(err)
	}
	noted := map[string]bool{}
	for _, u := range r.Uncertainties {
		if u.Kind == "func_value_into_dependency" {
			noted[short(u.Func)] = true
			if u.AttributableTo != "rivus" {
				t.Errorf("func_value_into_dependency in %s: attributable_to %q, want rivus", short(u.Func), u.AttributableTo)
			}
		}
	}
	for _, want := range []string{"intoDependency", "returnedBack"} {
		if !noted[want] {
			t.Errorf("%s: a function value is given to a function of a dependency, and the call does not say it (%v)", want, noted)
		}
	}
	for _, not := range []string{"noFunction", "sdk", "own"} {
		if noted[not] {
			t.Errorf("%s: it must not say func_value_into_dependency", not)
		}
	}
	found := map[string]bool{}
	for _, f := range r.Findings {
		fn := short(f.Source.Func)
		found[fn] = true
		said := false
		for _, u := range f.Uncertainties {
			said = said || u.Kind == "func_value_into_dependency"
		}
		if fn == "intoDependency" && !said {
			t.Errorf("%s: the finding goes through the call and does not carry the note", fn)
		}
		if (fn == "noFunction" || fn == "sdk" || fn == "own") && said {
			t.Errorf("%s: the finding carries a note that is not its", fn)
		}
	}
	for _, want := range []string{"intoDependency", "returnedBack", "noFunction", "sdk", "own"} {
		if !found[want] {
			t.Errorf("no finding from %s (have %v)", want, found)
		}
	}
	// a package whose body is not read: the callbacks are applied as called (assumed_call), and the new kind does not come too
	var q report
	if err := json.Unmarshal(run("-body-replaced", "package:dep"), &q); err != nil {
		t.Fatal(err)
	}
	for _, u := range q.Uncertainties {
		if u.Kind == "func_value_into_dependency" {
			t.Errorf("%s: a package whose body is not read says func_value_into_dependency (its callbacks are assumed_call)", short(u.Func))
		}
	}
}
