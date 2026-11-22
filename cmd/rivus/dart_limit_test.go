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

// A call of a function value that has more candidates than rivus keeps (ADR-050: 32) is not followed. The functions that
// pass a function to such a helper are told: the helper's summary says which parameter was called over the limit, and the
// caller has a depth_limit (candidate_limit) note whatever the argument looks like (a closure made there, a variable),
// also two helpers up. A helper under the limit, and the parameter that is not called, have none.
func TestCandidateLimitIsSaidWhereTheFunctionIsPassed(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("String transform(String s, String Function(String) f) => f(s);\n")
	b.WriteString("String inner(String s, String Function(String) f) => f(s);\n")
	b.WriteString("String outer(String s, String Function(String) f) => inner(s, f);\n")
	b.WriteString("String viaVar(String s, String Function(String) f) => f(s);\n")
	b.WriteString("String small(String s, String Function(String) f) => f(s);\n")
	b.WriteString("class Box { Box(this.f); final String Function(String) f; }\n")
	b.WriteString("String viaBox(String s, Box b) => b.f(s);\n")
	for i := 0; i < 34; i++ {
		fmt.Fprintf(&b, "void d%d() { transform('x', (x) => x + '%d'); }\n", i, i)
		fmt.Fprintf(&b, "void w%d() { outer('x', (x) => x + '%d'); }\n", i, i)
		fmt.Fprintf(&b, "void v%d() { final g = (String x) => x + '%d'; viaVar('x', g); }\n", i, i)
		fmt.Fprintf(&b, "void b%d() { viaBox('x', Box((x) => x + '%d')); }\n", i, i)
	}
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&b, "void s%d() { small('x', (x) => x + '%d'); }\n", i, i)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib", "a.dart"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: lm\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get")...)
	get.Dir = dir
	if out, err := get.CombinedOutput(); err != nil {
		t.Fatalf("dart pub get: %v\n%s", err, out)
	}
	cmd := exec.Command(binary, "analyze", "-lang", "dart", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var r struct {
		Uncertainties []struct {
			Kind   string `json:"kind"`
			Func   string `json:"func"`
			Detail string `json:"detail"`
		} `json:"uncertainties"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	told := map[string]string{} // the function -> the detail
	for _, u := range r.Uncertainties {
		if u.Kind == "depth_limit" && strings.HasPrefix(u.Detail, "candidate_limit: what is passed as ") {
			told[u.Func[strings.LastIndex(u.Func, "#")+1:]] = u.Detail
		}
	}
	for _, p := range []string{"d", "w", "v"} {
		for i := 0; i < 34; i++ {
			name := fmt.Sprintf("%s%d", p, i)
			d, ok := told[name]
			if !ok {
				t.Errorf("%s passes a function to a helper that is over the candidate limit, and has no note of it", name)
			} else if !strings.Contains(d, "argument 1 ") {
				t.Errorf("%s: the note is about another parameter than the called one (argument 1): %s", name, d)
			}
		}
	}
	for i := 0; i < 3; i++ {
		if _, ok := told[fmt.Sprintf("s%d", i)]; ok {
			t.Errorf("s%d passes a function to a helper that is under the limit, and has a note", i)
		}
	}
	for name, d := range told {
		if strings.Contains(d, "argument 0 ") {
			t.Errorf("%s: a note about the parameter that is not called: %s", name, d)
		}
	}
	// a function held in a field of what is passed (b.f(s)): such a value is not a candidate at all (no function is found:
	// unresolved_callee in the helper, not candidate_limit), so there is no limit to say, and the callers are not told (the
	// form of #79; counted here, not asserted)
	boxed := 0
	for i := 0; i < 34; i++ {
		if _, ok := told[fmt.Sprintf("b%d", i)]; ok {
			boxed++
		}
	}
	t.Logf("the function held in a field of the argument: %d of 34 callers are told", boxed)
}

// A call of a function value with no candidate (nothing passed to the helper is a function that rivus can see: it is held
// in a field or an element of what is passed): the callers that pass something there are told, as for the candidate
// limit; also two helpers up. A helper whose callers pass functions that are found (callIt), and the parameter that is not
// called, have no such note.
func TestUnresolvedCallIsSaidWhereTheFunctionIsPassed(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `class Box { Box(this.f); final String Function(String) f; }
String viaBox(String s, Box b) => b.f(s);
String viaList(String s, List<String Function(String)> fs) => fs[0](s);
String outerBox(String s, Box b) => viaBox(s, b);
String callIt(String s, String Function(String) f) => f(s);
void fb() { viaBox('x', Box((x) => x + 'b')); }
void fl() { viaList('x', [(x) => x + 'l']); }
void fo() { outerBox('x', Box((x) => x + 'o')); }
void ok0() { callIt('x', (x) => x + '0'); }
void ok1() { callIt('x', (x) => x + '1'); }
`
	if err := os.WriteFile(filepath.Join(dir, "lib", "a.dart"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: ur\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get")...)
	get.Dir = dir
	if out, err := get.CombinedOutput(); err != nil {
		t.Fatalf("dart pub get: %v\n%s", err, out)
	}
	cmd := exec.Command(binary, "analyze", "-lang", "dart", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var r struct {
		Uncertainties []struct {
			Kind   string `json:"kind"`
			Func   string `json:"func"`
			Detail string `json:"detail"`
		} `json:"uncertainties"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	told := map[string]string{}
	for _, u := range r.Uncertainties {
		if u.Kind == "unresolved_callee" && strings.HasPrefix(u.Detail, "the function that is passed as ") {
			told[u.Func[strings.LastIndex(u.Func, "#")+1:]] = u.Detail
		}
	}
	for _, name := range []string{"fb", "fl", "fo"} {
		d, ok := told[name]
		if !ok {
			t.Errorf("%s passes a function to a helper that calls it with no candidate, and has no note of it", name)
		} else if !strings.Contains(d, "argument 1 ") {
			t.Errorf("%s: the note is about another parameter than the called one (argument 1): %s", name, d)
		}
	}
	for _, name := range []string{"ok0", "ok1"} {
		if _, ok := told[name]; ok {
			t.Errorf("%s passes functions that are found, and has a note that they are not known", name)
		}
	}
}
