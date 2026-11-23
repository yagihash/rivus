package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

const cbUnresolvedRules = `ruleset: t/cb@1
sources:
  - kind: callback_param
    symbol: example.com/cases/cases/cbparam.Route
    operand: 1
    from: 1
    labels: [user_input]
`

// The registrations whose handler was not found are in summary.callback_param_unresolved (count and places), sorted, and there is
// none without a callback_param row (Go).
func TestCallbackParamUnresolvedInSummaryGo(t *testing.T) {
	t.Parallel()
	rules := filepath.Join(t.TempDir(), "cb.yaml")
	if err := os.WriteFile(rules, []byte(cbUnresolvedRules), 0o644); err != nil {
		t.Fatal(err)
	}
	var rep report.Report
	rivusJSON(t, &rep, "analyze", "-rules", rules, "./cases/cbparam")
	c := rep.Summary.CallbackParamUnresolved
	if c == nil || c.Count != 2 || len(c.Sites) != 2 {
		t.Fatalf("want the 2 calls (Registers, Chosen): %+v", c)
	}
	for i, s := range c.Sites {
		if !strings.HasSuffix(s.Symbol, "cbparam.Route") || s.Position.Line == 0 || s.Func == "" {
			t.Errorf("site %d: %+v", i, s)
		}
	}
	if !strings.HasSuffix(c.Sites[0].Func, "cbparam.Registers") || !strings.HasSuffix(c.Sites[1].Func, "cbparam.Chosen") {
		t.Errorf("not sorted by place: %+v", c.Sites)
	}
	n := 0
	for _, u := range rep.Uncertainties {
		if u.Kind == "unresolved_callee" && strings.HasPrefix(u.Detail, "callback_param_unresolved:") {
			n++
		}
	}
	if n != 2 {
		t.Errorf("the notes: %d", n)
	}
	var none report.Report
	rivusJSON(t, &none, "analyze", "./cases/cbparam")
	if none.Summary.CallbackParamUnresolved != nil {
		t.Errorf("no row, and the item is there: %+v", none.Summary.CallbackParamUnresolved)
	}
}

// The same for Dart: the helper that registers what it is given is counted; a function written at the call is not.
func TestCallbackParamUnresolvedInSummaryDart(t *testing.T) {
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
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pubspec.yaml", "name: dx\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n")
	write("rules.yaml", "ruleset: t/dx@1\nsources:\n  - kind: callback_param\n    symbol: package:dx/dx.dart#Router.get\n    operand: 1\n    from: 1\n    labels: [user_input]\n")
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("lib/dx.dart", `class Router { void get(String r, Function h) {} }
void direct(Router r) { r.get('/a', (int q, String id) {}); }
void helper(Router r, Function h) { r.get('/b', h); }
`)
	get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get")...)
	get.Dir = dir
	if out, err := get.CombinedOutput(); err != nil {
		t.Fatalf("dart pub get: %v\n%s", err, out)
	}
	run := func(rules bool) report.Report {
		args := []string{"analyze"}
		if rules {
			args = append(args, "-rules", "rules.yaml")
		}
		cmd := exec.Command(binary, append(args, "./...")...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		var rep report.Report
		if err := json.Unmarshal(out, &rep); err != nil {
			t.Fatal(err)
		}
		return rep
	}
	c := run(true).Summary.CallbackParamUnresolved
	if c == nil || c.Count != 1 || !strings.HasSuffix(c.Sites[0].Func, "#helper") {
		t.Errorf("want the call in helper only: %+v", c)
	}
	if run(false).Summary.CallbackParamUnresolved != nil {
		t.Error("no row, and the item is there")
	}
}
