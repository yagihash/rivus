package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// An instance of a class that has a `call` method is called like a function: r(x) is r.call(x), a method that the extractor
// can name from the static type (#97). Each form below has a source passed to such a call and the sink in the method.
func TestDartCallableObjectIsCalledAsItsCallMethod(t *testing.T) {
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
	write("rules.yaml", "ruleset: t/dx@1\nsources:\n  - symbol: package:dx/lib.dart#source\n    labels: [user_input]\nsinks:\n  - symbol: package:dx/lib.dart#sink\n    category: test\n")
	write("lib/lib.dart", "String source() => 'x';\nvoid sink(String s) {}\n")
	write("lib/c.dart", `import 'lib.dart';
class Runner { void call(String s) { sink(s); } }
class Base { void call(String s) { sink(s); } }
class Sub extends Base {}
mixin M { void call(String s) { sink(s); } }
class UsesMixin with M {}
class Foo {}
extension on Foo { void call(String s) { sink(s); } }
class G<T> { void call(T s) { sink('$s'); } }
class H { final Runner r = Runner(); void go() { r(source()); } }
void direct() { final r = Runner(); r(source()); }
void inSuper() { final r = Sub(); r(source()); }
void inMixin() { final r = UsesMixin(); r(source()); }
void inExtension() { final r = Foo(); r(source()); }
void generic() { final r = G<String>(); r(source()); }
void nullable() { Runner? r = Runner(); r!(source()); }
void field() { H().go(); }
void aFunction() { void Function(String) f = sink; f(source()); }
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
			Source struct{ Func string } `json:"source"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range r.Findings {
		seen[f.Source.Func[strings.LastIndex(f.Source.Func, "#")+1:]] = true
	}
	for _, want := range []string{"direct", "inSuper", "inMixin", "inExtension", "generic", "nullable", "H.go", "aFunction"} {
		if !seen[want] {
			t.Errorf("no finding of %s", want)
		}
	}
}
