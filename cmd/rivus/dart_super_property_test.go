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

// `super.x`, `super.x = v` and `super.x += v` run the getter and the setter of the direct superclass, and only those: a
// dynamic call would also reach what the subclasses override them with (the class that is calling, for example), which
// is a false edge (a body that the call does not run).
func TestDartSuperPropertyIsStatic(t *testing.T) {
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
	write("pubspec.yaml", "name: sp\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("lib/lib.dart", `class A {
  String _x = '';
  String get x => _x;
  set x(String v) { _x = v; }
}
class B extends A {
  @override
  String get x => 'b';
  @override
  set x(String v) {}
  void readSuper() { var r = super.x; }
  void assignSuper() { super.x = 'v'; }
  void compoundSuper() { super.x += 'v'; }
  void readThis() { var r = this.x; }
}
class C extends B {
  @override
  String get x => 'c';
  @override
  set x(String v) {}
}
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
	// the functions that the calls of the property x in a function may reach (a static call: the function; a dynamic call: its candidates)
	reach := func(fn string) string {
		got := map[string]bool{}
		for _, f := range prog.Funcs {
			if f.ID != ir.FuncID("package:sp/lib.dart#"+fn) {
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
						got[string(id)[strings.LastIndex(string(id), "#")+1:]] = true
					}
				}
			}
		}
		var names []string
		for n := range got {
			names = append(names, n)
		}
		sort.Strings(names)
		return strings.Join(names, " ")
	}
	for fn, want := range map[string]string{
		"B.readSuper":     "A.x",
		"B.assignSuper":   "A.x=",
		"B.compoundSuper": "A.x A.x=",
		"B.readThis":      "B.x C.x", // this.x is a dynamic call: B's own getter, and the one of its subclass
	} {
		if got := reach(fn); got != want {
			t.Errorf("%s reaches [%s], want [%s]", fn, got, want)
		}
	}
}

// `super + o`, `super == o`, `super[i]`, `super[i] = v` and `super[i] += v` run the operators of the direct superclass, and
// only those (a dynamic call would also reach the ones that the subclasses override them with).
func TestDartSuperOperatorIsStatic(t *testing.T) {
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
	write("pubspec.yaml", "name: sp\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("lib/lib.dart", `class A {
  String _v = '';
  A operator +(A o) => this;
  @override
  bool operator ==(Object o) => true;
  @override
  int get hashCode => 0;
  String operator [](int i) => _v;
  void operator []=(int i, String s) { _v = s; }
}
class B extends A {
  @override
  A operator +(A o) => this;
  @override
  bool operator ==(Object o) => false;
  @override
  String operator [](int i) => 'b';
  @override
  void operator []=(int i, String s) {}
  void superAdd(A o) { var r = super + o; }
  void superEq(A o) { var r = super == o; }
  void superIndex() { var r = super[0]; }
  void superIndexSet() { super[0] = 'v'; }
  void superIndexCompound() { super[0] += 'v'; }
  void thisAdd(A o) { var r = this + o; }
}
class C extends B {
  @override
  A operator +(A o) => this;
  @override
  bool operator ==(Object o) => false;
  @override
  String operator [](int i) => 'c';
  @override
  void operator []=(int i, String s) {}
}
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
	// the functions that the calls of the property x in a function may reach (a static call: the function; a dynamic call: its candidates)
	reach := func(fn string) string {
		got := map[string]bool{}
		for _, f := range prog.Funcs {
			if f.ID != ir.FuncID("package:sp/lib.dart#"+fn) {
				continue
			}
			for _, b := range f.Blocks {
				for _, in := range b.Instrs {
					if in.Op != ir.OpCall || in.Callee == nil || !strings.HasSuffix(in.Callee.Symbol, ".+") && !strings.HasSuffix(in.Callee.Symbol, ".==") && !strings.HasSuffix(in.Callee.Symbol, ".[]") && !strings.HasSuffix(in.Callee.Symbol, ".[]=") {
						continue
					}
					ids := in.Callee.Candidates
					if in.Callee.Func != "" {
						ids = append([]ir.FuncID{in.Callee.Func}, ids...)
					}
					for _, id := range ids {
						got[string(id)[strings.LastIndex(string(id), "#")+1:]] = true
					}
				}
			}
		}
		var names []string
		for n := range got {
			names = append(names, n)
		}
		sort.Strings(names)
		return strings.Join(names, " ")
	}
	for fn, want := range map[string]string{
		"B.superAdd":           "A.+",
		"B.superEq":            "A.==",
		"B.superIndex":         "A.[]",
		"B.superIndexSet":      "A.[]=",
		"B.superIndexCompound": "A.[] A.[]=",
		"B.thisAdd":            "B.+ C.+", // this + o is a dynamic call: B's own, and the one of its subclass
	} {
		if got := reach(fn); got != want {
			t.Errorf("%s reaches [%s], want [%s]", fn, got, want)
		}
	}
}
