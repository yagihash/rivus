package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
)

const ssLib = `package lib

func Source() string { return "" }
func Sink(s string)  {}
func XSS(s string)   {}
`

// The rules of the user: their own source and sinks, and nothing of the built-in sources and sinks.
const ssRules = `
ruleset: mine@1
sources:
  - symbol: example.com/tmp/lib.Source
    labels: [user_input]
sinks:
  - symbol: example.com/tmp/lib.Sink
    category: mine
  - symbol: example.com/tmp/lib.XSS
    category: xss
`

const ssMain = `package main

import (
	"html"
	"net/http"
	"os/exec"
	"path/filepath"

	"example.com/tmp/lib"
)

// through filepath.Join (a propagators row of the built-in models) to a sink of the user's
func Joined() {
	a := lib.Source()
	lib.Sink(filepath.Join("/x", a))
}

// to a built-in sink
func Command() {
	a := lib.Source()
	exec.Command(a)
}

// html.EscapeString (a built-in sanitizers row) neutralises xss
func Escaped() {
	a := lib.Source()
	lib.XSS(html.EscapeString(a))
}

// a built-in source (the type of a parameter)
func Handler(w http.ResponseWriter, r *http.Request) {
	lib.Sink(r.URL.Path)
}
`

func ssWorkspace(t *testing.T) *workspace {
	w := newWorkspace(t)
	w.write("lib/lib.go", ssLib)
	w.write("rules.yaml", ssRules)
	w.write("main/main.go", ssMain)
	return w
}

func ssAnalyze(t *testing.T, w *workspace, args ...string) report.Report {
	t.Helper()
	var rep report.Report
	if err := json.Unmarshal(w.run(append([]string{"analyze"}, append(args, "-rules", "rules.yaml", "./main")...)...), &rep); err != nil {
		t.Fatal(err)
	}
	return rep
}

func ssSinks(rep report.Report) map[string]int {
	m := map[string]int{}
	for _, f := range rep.Findings {
		m[f.Sink.Symbol]++
	}
	return m
}

// -no-builtin-sources-sinks leaves out the built-in sources and sinks and keeps the rest: the flow through
// filepath.Join (propagators) reaches the sink of the user, the built-in sink and the built-in source are gone, and the
// built-in sanitizers row still works for a sink of the user's.
func TestNoBuiltinSourcesSinks(t *testing.T) {
	t.Parallel()
	w := ssWorkspace(t)

	// without the flag: the built-in sink and the built-in source are in
	all := ssSinks(ssAnalyze(t, w))
	if all["os/exec.Command"] == 0 {
		t.Fatalf("without the flag the built-in sink is not reached: %v", all)
	}

	rep := ssAnalyze(t, w, "-no-builtin-sources-sinks")
	got := ssSinks(rep)
	if len(rep.Findings) != 1 || got["example.com/tmp/lib.Sink"] != 1 {
		t.Fatalf("want one finding, at the sink of the user (through filepath.Join): %v", got)
	}
	if f := rep.Findings[0]; f.Source.Symbol != "example.com/tmp/lib.Source" {
		t.Errorf("source %q", f.Source.Symbol)
	}
	if got["os/exec.Command"] != 0 {
		t.Errorf("a built-in sink is still found: %v", got)
	}
	// the sanitizers row of the built-in models works on the sink of the user: none at lib.XSS
	if got["example.com/tmp/lib.XSS"] != 0 {
		t.Errorf("html.EscapeString does not neutralise xss: %v", got)
	}
	// the built-in rule sets are named as read without sources and sinks
	var marked int
	for _, n := range rep.Rules {
		if strings.HasPrefix(n, "rivus/") {
			if !strings.HasSuffix(n, " (sources and sinks left out)") {
				t.Errorf("rule set %q is not marked", n)
			}
			marked++
		}
	}
	if marked == 0 {
		t.Errorf("no built-in rule set in %v", rep.Rules)
	}
	// without the flag no name is marked
	for _, n := range ssAnalyze(t, w).Rules {
		if strings.Contains(n, "left out") {
			t.Errorf("rule set %q is marked without the flag", n)
		}
	}
}

// With -no-builtin-rules too, that one wins: no built-in rule set is read, marked or not.
func TestNoBuiltinSourcesSinksWithNoBuiltinRules(t *testing.T) {
	t.Parallel()
	w := ssWorkspace(t)
	rep := ssAnalyze(t, w, "-no-builtin-sources-sinks", "-no-builtin-rules")
	for _, n := range rep.Rules {
		if strings.HasPrefix(n, "rivus/") {
			t.Errorf("a built-in rule set is read: %q", n)
		}
	}
}

// sites lists no built-in source with the flag, and one without it.
func TestNoBuiltinSourcesSinksSites(t *testing.T) {
	t.Parallel()
	w := ssWorkspace(t)
	count := func(args ...string) int {
		var out struct {
			Sites []struct {
				Role   string `json:"role"`
				Symbol string `json:"symbol"`
			} `json:"sites"`
		}
		if err := json.Unmarshal(w.run(append([]string{"sites", "-role", "source"}, append(args, "./main")...)...), &out); err != nil {
			t.Fatal(err)
		}
		return len(out.Sites)
	}
	if n := count(); n == 0 {
		t.Fatalf("the built-in source (the request of a handler) is not listed without the flag")
	}
	if n := count("-no-builtin-sources-sinks"); n != 0 {
		t.Errorf("%d sources with the flag and no rules of the user", n)
	}
	if n := count("-no-builtin-sources-sinks", "-rules", "rules.yaml"); n != 3 {
		t.Errorf("want the three calls of the source of the user, got %d", n)
	}
}

// -categories knows the categories of the sinks that are loaded: with the built-in sinks left out, a built-in category is
// unknown unless a sink of the user has it.
func TestNoBuiltinSourcesSinksCategories(t *testing.T) {
	t.Parallel()
	w := ssWorkspace(t)
	run := func(args ...string) (string, error) {
		cmd := exec.Command(binary, append([]string{"analyze", "-no-builtin-sources-sinks", "-rules", "rules.yaml"}, append(args, "./main")...)...)
		cmd.Dir = w.dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("-categories", "sql_injection"); err == nil || !strings.Contains(out, "unknown category") {
		t.Errorf("a built-in category is known: %v\n%s", err, out)
	}
	// xss is a category of a sink of the user's
	if out, err := run("-categories", "xss"); err != nil {
		t.Errorf("a category of the user's sink is unknown: %v\n%s", err, out)
	}
	// the same flag without it knows sql_injection
	cmd := exec.Command(binary, "analyze", "-rules", "rules.yaml", "-categories", "sql_injection", "./main")
	cmd.Dir = w.dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("without the flag: %v\n%s", err, out)
	}
}
