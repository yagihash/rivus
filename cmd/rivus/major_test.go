package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// A row written with /v* (any major of a module, model.MajorWildcard) is for the module of each major that the program has:
// the same row meets lib/v2 and lib/v3, and not lib/vault (a path that is not a v and digits) or lib (no major in its path).
func TestMajorWildcardRowMeetsEveryMajor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mods := map[string]string{"v2": "example.invalid/lib/v2", "v3": "example.invalid/lib/v3", "vault": "example.invalid/lib/vault", "plain": "example.invalid/lib"}
	for dir, path := range mods {
		write(t, filepath.Join(root, dir, "go.mod"), "module "+path+"\n\ngo 1.21\n")
		write(t, filepath.Join(root, dir, "pkg", "pkg.go"), "package pkg\n\nfunc F(s string) {}\n\nfunc Other(s string) {}\n")
	}
	app := filepath.Join(root, "app")
	write(t, filepath.Join(app, "go.mod"), `module example.com/app

go 1.21

require (
	example.invalid/lib v0.0.0
	example.invalid/lib/v2 v2.0.0
	example.invalid/lib/v3 v3.0.0
	example.invalid/lib/vault v0.0.0
)

replace (
	example.invalid/lib => ../plain
	example.invalid/lib/v2 => ../v2
	example.invalid/lib/v3 => ../v3
	example.invalid/lib/vault => ../vault
)
`)
	write(t, filepath.Join(app, "main.go"), `package main

import (
	"net/http"

	plain "example.invalid/lib/pkg"
	v2 "example.invalid/lib/v2/pkg"
	v3 "example.invalid/lib/v3/pkg"
	vault "example.invalid/lib/vault/pkg"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		v2.F(q)
		v3.F(q)
		vault.F(q)
		plain.F(q)
		v2.Other(q)
	})
}
`)
	rules := brRules(t, app, "major.yaml", `ruleset: t@1
sinks:
  - symbol: example.invalid/lib/v*/pkg.F
    args: [0]
    category: command_injection
`)
	got := brAnalyze(t, app, "-rules", rules)
	if len(got.Findings) != 2 {
		t.Fatalf("findings: %d, want 2 (v2.F and v3.F)\n%s", len(got.Findings), got.raw)
	}
	text := string(got.raw)
	for _, want := range []string{"example.invalid/lib/v2/pkg.F", "example.invalid/lib/v3/pkg.F"} {
		if !strings.Contains(text, want) {
			t.Errorf("no finding at %s", want)
		}
	}
	for _, not := range []string{"lib/vault/pkg.F", "example.invalid/lib/pkg.F", "pkg.Other"} {
		for _, f := range got.Findings {
			if strings.Contains(f.ID, not) {
				t.Errorf("a finding at %s", not)
			}
		}
	}

	// the row without /v* is as it was: it names one module
	rules = brRules(t, app, "exact.yaml", `ruleset: t@1
sinks:
  - symbol: example.invalid/lib/v2/pkg.F
    args: [0]
    category: command_injection
`)
	if got := brAnalyze(t, app, "-rules", rules); len(got.Findings) != 1 {
		t.Errorf("a row of v2: %d findings, want 1", len(got.Findings))
	}
}
