package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// A getter and a setter of the same name are two functions: reading `a.x` runs the getter, assigning to it runs the
// setter, and `a.x += v` does both (it reads, then writes). The extractor's table of the methods of a class is by
// name, and the rules speak of the field (`x`), so the two must not be mixed: a read must not reach the body of the
// setter (a false edge: whatever the setter does with `this`, for example sinking a field, would be "run" by a read),
// and an assignment must not reach the body of the getter.
func TestDartGetterAndSetterAreNotMixed(t *testing.T) {
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
	write("pubspec.yaml", "name: gs\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("lib/lib.dart", `class A {
  String? _x;
  String? get x => _x;
  set x(String? v) { _x = v; }
  void assignThis() { this.x = 'v'; }
  void assignImplicit() { x = 'v'; }
  void readThis() { var r = this.x; }
  void readImplicit() { var r = x; }
  void compoundImplicit() { x += 'v'; }
}
class B extends A {
  void assignSuper() { super.x = 'v'; }
  void readSuper() { var r = super.x; }
  void compoundSuper() { super.x += 'v'; }
}
void read(A a) { var r = a.x; }
void assign(A a) { a.x = 'v'; }
void compound(A a) { a.x += 'v'; }
void orAssign(A a) { a.x ??= 'v'; }
void cascade(A a) { a..x = 'v'; }
void assignOnSub(B b) { b.x = 'v'; }
`)
	get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get")...)
	get.Dir = dir
	if out, err := get.CombinedOutput(); err != nil {
		t.Fatalf("dart pub get: %v\n%s", err, out)
	}
	cmd := exec.Command(words[0], append(append([]string{}, words[1:]...), "run", "bin/rivus_dart.dart", dir)...)
	cmd.Dir = extractor
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v\n%s", err, stderr.String())
	}
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	// for each function: the functions that its calls of the property x may reach (a static call: the function; a
	// dynamic call: its candidates), split into getters and setters (a setter's ID ends with "=")
	reach := func(fn string) (getters, setters []string) {
		for _, f := range prog.Funcs {
			if f.ID != ir.FuncID("package:gs/lib.dart#"+fn) {
				continue
			}
			for _, b := range f.Blocks {
				for _, in := range b.Instrs {
					if in.Op != ir.OpCall || in.Callee == nil || !strings.HasSuffix(in.Callee.Symbol, ".x") {
						continue
					}
					ids := in.Callee.Candidates
					if in.Callee.Func != "" {
						ids = append([]ir.FuncID{in.Callee.Func}, ids...)
					}
					for _, id := range ids {
						if strings.HasSuffix(string(id), "=") {
							setters = append(setters, string(id))
						} else {
							getters = append(getters, string(id))
						}
					}
				}
			}
		}
		sort.Strings(getters)
		sort.Strings(setters)
		return
	}
	const (
		getter = 1 // a call that reaches a getter
		setter = 2 // a call that reaches a setter
	)
	// what each form calls: a read (the getter), an assignment (the setter), a compound assignment (both)
	want := map[string]int{
		"read": getter, "assign": setter, "compound": getter | setter, "orAssign": getter | setter, "cascade": setter,
		"assignOnSub":        setter,
		"A.readThis":         getter,
		"A.readImplicit":     getter,
		"A.assignThis":       setter,
		"A.assignImplicit":   setter,
		"A.compoundImplicit": getter | setter,
		"B.assignSuper":      setter,
		"B.readSuper":        getter,
		"B.compoundSuper":    getter | setter,
	}
	for fn, kinds := range want {
		gets, sets := reach(fn)
		if kinds&getter != 0 && len(gets) == 0 {
			t.Errorf("%s: no getter is reached", fn)
		}
		if kinds&getter == 0 && len(gets) != 0 {
			t.Errorf("%s: an assignment reaches a getter: %v", fn, gets)
		}
		if kinds&setter != 0 && len(sets) == 0 {
			t.Errorf("%s: no setter is reached", fn)
		}
		if kinds&setter == 0 && len(sets) != 0 {
			t.Errorf("%s: a read reaches a setter: %v", fn, sets)
		}
	}
}
