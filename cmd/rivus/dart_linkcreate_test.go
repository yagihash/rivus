package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The builtin rows of dart:io: the target of a link (Link.create, Link.createSync, argument 0) is a path_traversal sink, and
// the path of the link (Link.new) is another one. A tainted value reaches the sink of the argument it is given as, and not
// the other (bench/dart-archive/README.md).
func TestDartLinkCreateTarget(t *testing.T) {
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
	write("app/pubspec.yaml", "name: lk\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("app/.dart_tool/package_config.json", `{"configVersion":2,"packages":[{"name":"lk","rootUri":"../","packageUri":"lib/","languageVersion":"3.10"}],"generator":"pub","generatorVersion":"3.13.7"}`)
	write("app/rules.yaml", "ruleset: t/lk@1\nsources:\n  - symbol: package:lk/lib.dart#source\n    labels: [user_input]\n")
	write("app/lib/lib.dart", `import 'dart:io';
String source() => 'x';
void targetAsync() { Link('/srv/l').create(source()); }
void targetSync() { Link('/srv/l').createSync(source()); }
void pathOnly() { Link(source()).createSync('/srv/t'); }
void constant() { Link('/srv/l').createSync('/srv/t'); }
`)
	cmd := exec.Command(binary, "analyze", "-rules", "rules.yaml", "./...")
	cmd.Dir = filepath.Join(root, "app")
	cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var r struct {
		Findings []struct {
			Category string `json:"category"`
			Sink     struct {
				Symbol string `json:"symbol"`
				Func   string `json:"func"`
			} `json:"sink"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range r.Findings {
		if f.Category == "path_traversal" {
			got[f.Sink.Func[strings.LastIndex(f.Sink.Func, "#")+1:]+" "+f.Sink.Symbol] = true
		}
	}
	for _, want := range []string{
		"targetAsync dart:io#Link.create",
		"targetSync dart:io#Link.createSync",
		"pathOnly dart:io#Link.new",
	} {
		if !got[want] {
			t.Errorf("missing path_traversal finding %q (got %v)", want, got)
		}
	}
	for _, not := range []string{
		"targetAsync dart:io#Link.new", "targetSync dart:io#Link.new",
		"pathOnly dart:io#Link.createSync", "pathOnly dart:io#Link.create",
	} {
		if got[not] {
			t.Errorf("unexpected finding %q: the value is given as the other argument", not)
		}
	}
	for k := range got {
		if strings.HasPrefix(k, "constant ") {
			t.Errorf("a constant reaches no sink, got %q", k)
		}
	}
}
