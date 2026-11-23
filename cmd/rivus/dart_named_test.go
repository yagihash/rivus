package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A sink can name a named parameter (`named:`), in `args`-like form and in `parts`, and a symbol can have sinks of
// several categories (#139, #140). The calls below pass a source to each form; the analysis has to find the ones that
// reach the named parameter, and say so for a name that no call has.
func TestDartNamedArgumentSinks(t *testing.T) {
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
  - symbol: package:dx/lib.dart#Query.new
    named: [kind]
    category: by_name
  - symbol: package:dx/lib.dart#Client.list
    named: [prefix]
    category: by_name
  - symbol: package:dx/lib.dart#Client.get
    args: [0]
    category: positional
  - symbol: package:dx/lib.dart#Client.get
    named: [name]
    category: second_category
  - symbol: package:dx/lib.dart#Client.send
    parts:
      - {named: request, field: url}
    category: part
  - symbol: package:dx/lib.dart#Client.misspelt
    named: [prefx]
    category: misspelt
`)
	write("lib/lib.dart", `String source() => 'x';
class Query { Query({String? namespace, String? kind, int? limit}); }
class Req { final String url; Req({required this.url}); }
class Client {
  void list(String bucket, {String? delimiter, String? prefix, int? max}) {}
  void get(String id, {String? name}) {}
  void send({Req? request, String? other}) {}
  void misspelt(String a, {String? prefix}) {}
}
void kindGiven() { Query(kind: source(), limit: 1); }
void kindOnlyOther() { Query(namespace: source()); }
void kindNotPassed() { Query(limit: 3); }
void prefixGiven(Client c) { c.list('b', max: 2, prefix: source()); }
void prefixOnlyOther(Client c) { c.list('b', delimiter: source()); }
void positionalAndNamed(Client c) { c.get(source(), name: 'n'); }
void namedOfTwoCategories(Client c) { c.get('i', name: source()); }
void inRequest(Client c) { c.send(request: Req(url: source())); }
void inOther(Client c) { c.send(other: source()); }
void wrongName(Client c) { c.misspelt(source(), prefix: 'p'); }
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
	var r struct {
		Findings []struct {
			Source   struct{ Func string } `json:"source"`
			Category string                `json:"category"`
		} `json:"findings"`
		Uncertainties []struct {
			Kind string `json:"kind"`
			Func string `json:"func"`
		} `json:"uncertainties"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, f := range r.Findings {
		found[f.Source.Func[strings.LastIndex(f.Source.Func, "#")+1:]+" "+f.Category] = true
	}
	for _, want := range []string{
		"kindGiven by_name", "prefixGiven by_name", "positionalAndNamed positional", "namedOfTwoCategories second_category",
		"inRequest part", "wrongName misspelt", // a name no call has: the sink reads the whole call
	} {
		if !found[want] {
			t.Errorf("no finding %q (have %v)", want, found)
		}
	}
	for _, not := range []string{
		"kindOnlyOther by_name", "kindNotPassed by_name", "prefixOnlyOther by_name", "inOther part",
		"positionalAndNamed second_category", "namedOfTwoCategories positional",
	} {
		if found[not] {
			t.Errorf("finding %q: the argument is not the named one", not)
		}
	}
	unresolved := 0
	for _, u := range r.Uncertainties {
		if u.Kind == "named_unresolved" {
			unresolved++
		}
	}
	if unresolved == 0 {
		t.Errorf("a name that no call has is not said (named_unresolved): %s", out)
	}
}
