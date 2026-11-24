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

// A call through an interface (or an abstract class) may reach the method that a class takes from a mixin or from a
// superclass without writing it. The order that Dart looks in is the class itself, the mixins (the last one first),
// then the superclass; `implements` takes no body. A body that no class of the interface takes is not a candidate.
func TestDartInheritedImplementationsAreCandidates(t *testing.T) {
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
	write("pubspec.yaml", "name: inh\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("lib/lib.dart", `abstract class Store { void put(String s); }
mixin Saver { void put(String s) {} }
class DiskStore extends Object with Saver implements Store {}
void viaMixin(Store s) { s.put('v'); }

abstract class StoreB { void put(String s); }
class BaseB { void put(String s) {} }
class MiddleB extends BaseB {}
class LeafB extends MiddleB implements StoreB {}
void viaSuper(StoreB s) { s.put('v'); }

abstract class StoreC { void put(String s); }
mixin FirstC { void put(String s) {} }
mixin LastC { void put(String s) {} }
class OrderC extends Object with FirstC, LastC implements StoreC {}
void viaOrder(StoreC s) { s.put('v'); }

abstract class StoreD { void put(String s); }
class BaseD { void put(String s) {} }
mixin SaverD on BaseD { @override void put(String s) {} }
class OnD extends BaseD with SaverD implements StoreD {}
void viaOn(StoreD s) { s.put('v'); }

abstract class StoreE { void put(String s); }
mixin SaverE { void put(String s) {} }
class UserE extends Object with SaverE {}
class OverE extends Object with SaverE implements StoreE { @override void put(String s) {} }
void viaOverride(StoreE s) { s.put('v'); }

abstract class StoreF { void put(String s); }
class HasBodyF { void put(String s) {} }
class OnlyF implements HasBodyF, StoreF { @override void put(String s) {} }
void viaImplements(StoreF s) { s.put('v'); }

abstract class StoreG { void put(String s); }
class BaseG { void put(String s) {} }
class SubG extends BaseG implements StoreG { @override void put(String s) {} }
class PlainG extends BaseG implements StoreG {}
void viaSubclass(StoreG s) { s.put('v'); }
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
	// the methods that the calls of a function may reach, as Class.method
	reach := func(fn string) string {
		got := map[string]bool{}
		for _, f := range prog.Funcs {
			if f.ID != ir.FuncID("package:inh/lib.dart#"+fn) {
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
						got[s[strings.LastIndex(s, "#")+1:]] = true
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
		"viaMixin":      "Saver.put",          // DiskStore does not write it and takes it from the mixin
		"viaSuper":      "BaseB.put",          // two classes up
		"viaOrder":      "LastC.put",          // the last mixin wins, FirstC.put is not reached
		"viaOn":         "SaverD.put",         // the mixin wins over the superclass it is on, BaseD.put is not reached
		"viaOverride":   "OverE.put",          // the class writes its own, SaverE.put is not reached (UserE is not a Store)
		"viaImplements": "OnlyF.put",          // `implements` takes no body: HasBodyF.put is not reached
		"viaSubclass":   "BaseG.put SubG.put", // PlainG takes BaseG.put, SubG writes its own
	} {
		if got := reach(fn); got != want {
			t.Errorf("%s reaches [%s], want [%s]", fn, got, want)
		}
	}
}
