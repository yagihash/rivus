package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Function values that are held in a cell (docs/issues/210, stage 1). A closure that is put in a field, a container, a
// top-level variable or a list and called through it is followed: the summary of each function that the cell holds is
// applied at the call, and the call says that the candidates were added (unresolved_callee, func_cells), also on the
// finding that goes through it. The cell is told by its root: a function put in the object of one allocation is not called
// through the object of another. A closure that is given to a sink is no finding. More than 32 functions in a cell are all
// followed (a flow that was added while the cell held fewer cannot be taken back, so folding would make the result depend on
// the order of the iteration), and the call says so (candidate_limit). The result is the same with one worker and with many.
func TestDartFuncCells(t *testing.T) {
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
	write("pubspec.yaml", "name: fc\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("rules.yaml", `ruleset: t/fc@1
sources:
  - symbol: package:fc/lib.dart#source
    labels: [user_input]
sinks:
  - symbol: package:fc/lib.dart#sink
    args: [0]
    category: cmd
`)
	var b strings.Builder
	b.WriteString(`String source() => 'x';
void sink(Object o) {}
class H { void Function(String)? a; void Function(String)? b; }
void field() { final h = H(); h.a = (x) { sink(x); }; h.a!(source()); }
void otherInstance() { final h = H(); final g = H(); h.a = (x) { sink(x); }; g.a = (x) {}; g.a!(source()); }
void otherField() { final h = H(); h.a = (x) { sink(x); }; h.b = (x) {}; h.b!(source()); }
void map() { final m = <String, void Function(String)>{'k': (x) { sink(x); }}; m['k']!(source()); }
void Function(String)? gFn;
void setG() { gFn = (x) { sink(x); }; }
void callG(String v) { gFn!(v); }
void global() { setG(); callG(source()); }
void list() { final fs = <void Function(String)>[]; fs.add((x) { sink(x); }); for (final f in fs) { f(source()); } }
void closureToSink() { final f = (String x) {}; sink(f); }
void tearOff() { final h = H(); h.a = takes; h.a!(source()); }
void takes(String s) { sink(s); }
void many() {
  final fs = <void Function(String)>[];
`)
	for i := 0; i < 34; i++ {
		fmt.Fprintf(&b, "  fs.add((x) { sink(x + '%d'); });\n", i)
	}
	b.WriteString("  fs[0](source());\n}\n")
	write("lib/lib.dart", b.String())
	get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get")...)
	get.Dir = dir
	if out, err := get.CombinedOutput(); err != nil {
		t.Fatalf("dart pub get: %v\n%s", err, out)
	}
	run := func(workers string) []byte {
		cmd := exec.Command(binary, "analyze", "-workers", workers, "-rules", "rules.yaml", "./...")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		return out
	}
	out := run("0")
	for _, w := range []string{"1", "8"} {
		if other := run(w); !sameReport(t, out, other) {
			t.Errorf("-workers %s gives another result than the default", w)
		}
	}
	var r struct {
		Findings []struct {
			Source        struct{ Func string } `json:"source"`
			Sink          struct{ Func string } `json:"sink"`
			Uncertainties []struct {
				Kind   string `json:"kind"`
				Detail string `json:"detail"`
			} `json:"uncertainties"`
		} `json:"findings"`
		Uncertainties []struct {
			Kind   string `json:"kind"`
			Func   string `json:"func"`
			Detail string `json:"detail"`
		} `json:"uncertainties"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	short := func(s string) string { return s[strings.LastIndex(s, "#")+1:] }
	found := map[string]bool{}
	for _, f := range r.Findings {
		fn := short(f.Source.Func)
		found[fn] = true
		said := false
		for _, u := range f.Uncertainties {
			said = said || u.Kind == "unresolved_callee" && strings.HasPrefix(u.Detail, "func_cells:")
		}
		if !said {
			t.Errorf("%s: the finding goes through a function value that was found in a cell, and does not say so (%v)", fn, f.Uncertainties)
		}
	}
	for _, want := range []string{"field", "map", "global", "list", "tearOff", "many"} {
		if !found[want] {
			t.Errorf("no finding from %s (have %v)", want, found)
		}
	}
	for _, not := range []string{"otherInstance", "otherField", "closureToSink"} {
		if found[not] {
			t.Errorf("finding from %s: it must stay quiet", not)
		}
	}
	limit := false
	for _, u := range r.Uncertainties {
		if u.Kind == "depth_limit" && short(u.Func) == "many" && strings.HasPrefix(u.Detail, "candidate_limit: the cell of the function value") {
			limit = true
		}
	}
	if !limit {
		t.Errorf("many: a cell with more than 32 functions: the call does not say it (candidate_limit)")
	}
}

// sameReport says whether two reports hold the same findings and uncertainties, in the same order.
func sameReport(t *testing.T, a, b []byte) bool {
	t.Helper()
	pick := func(x []byte) string {
		var r struct {
			Findings      json.RawMessage `json:"findings"`
			Uncertainties json.RawMessage `json:"uncertainties"`
		}
		if err := json.Unmarshal(x, &r); err != nil {
			t.Fatal(err)
		}
		return string(r.Findings) + string(r.Uncertainties)
	}
	return pick(a) == pick(b)
}
