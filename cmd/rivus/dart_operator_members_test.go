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

// The name of an operator is the name of the function: `<=` is not `<`, `[]=` is not `[]`, and `==` is not nothing.
// The extractor's table of the methods of a class used to take every "=" off the name, so a call of one operator
// could reach the body of another (`a <= b` the body of `<`, `a[i]` the body of `[]=`). Each form must reach the
// operators that it calls, and only those: `a[i] += v` calls `[]` and then `[]=`.
func TestDartOperatorsAreNotMixed(t *testing.T) {
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
	write("pubspec.yaml", "name: ops\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("lib/lib.dart", `class A {
  String _v = '';
  String operator [](int i) => _v;
  void operator []=(int i, String s) { _v = s; }
  bool operator <(A o) => true;
  bool operator <=(A o) => true;
  bool operator >(A o) => true;
  bool operator >=(A o) => true;
  @override
  bool operator ==(Object o) => true;
  @override
  int get hashCode => 0;
}
class B extends A {
  @override
  bool operator <=(A o) => false;
}
void index(A a) { var r = a[0]; }
void indexSet(A a) { a[0] = 'v'; }
void indexCompound(A a) { a[0] += 'v'; }
void indexOrAssign(A a) { a[0] ??= 'v'; }
void lt(A a, A b) { var r = a < b; }
void le(A a, A b) { var r = a <= b; }
void gt(A a, A b) { var r = a > b; }
void ge(A a, A b) { var r = a >= b; }
void eq(A a, A b) { var r = a == b; }
void leOnSub(B a, A b) { var r = a <= b; }
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
	// the operators that the calls of a function may reach (a static call: the function; a dynamic call: its candidates)
	reach := func(fn string) string {
		got := map[string]bool{}
		for _, f := range prog.Funcs {
			if f.ID != ir.FuncID("package:ops/lib.dart#"+fn) {
				continue
			}
			for _, b := range f.Blocks {
				for _, in := range b.Instrs {
					if in.Op != ir.OpCall || in.Callee == nil {
						continue
					}
					ids := in.Callee.Candidates
					if in.Callee.Func != "" {
						ids = append([]ir.FuncID{in.Callee.Func}, ids...)
					}
					for _, id := range ids {
						s := string(id)
						if i := strings.LastIndex(s, "#A."); i >= 0 {
							got["A."+s[i+3:]] = true
						} else if i := strings.LastIndex(s, "#B."); i >= 0 {
							got["B."+s[i+3:]] = true
						}
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
		"index":         "A.[]",
		"indexSet":      "A.[]=",
		"indexCompound": "A.[] A.[]=",
		"indexOrAssign": "A.[] A.[]=",
		"lt":            "A.<",
		"le":            "A.<= B.<=", // B overrides it: a call on an A may run either
		"gt":            "A.>",
		"ge":            "A.>=",
		"eq":            "A.==",
		"leOnSub":       "B.<=",
	} {
		if got := reach(fn); got != want {
			t.Errorf("%s reaches [%s], want [%s]", fn, got, want)
		}
	}
}
