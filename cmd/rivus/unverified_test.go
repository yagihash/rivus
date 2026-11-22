package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// unverifiedProxy is a module proxy on disk (GOPROXY=file://) with example.invalid/lib/v2 at v2.3.0 and a module with no record,
// example.invalid/other at v1.0.0: the version of a module comes from go.mod, and a replacement by a directory has none.
func unverifiedProxy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	add := func(mod, ver, pkgSrc string) {
		dir := filepath.Join(root, mod, "@v")
		gomod := "module " + mod + "\n\ngo 1.21\n"
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for name, body := range map[string]string{"go.mod": gomod, "pkg/pkg.go": pkgSrc} {
			w, err := zw.Create(mod + "@" + ver + "/" + name)
			if err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(body))
		}
		zw.Close()
		write(t, filepath.Join(dir, ver+".info"), `{"Version":"`+ver+`","Time":"2024-01-01T00:00:00Z"}`)
		write(t, filepath.Join(dir, ver+".mod"), gomod)
		write(t, filepath.Join(dir, "list"), ver+"\n")
		if err := os.WriteFile(filepath.Join(dir, ver+".zip"), buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	add("example.invalid/lib/v2", "v2.3.0", "package pkg\n\nfunc F(s string) {}\n")
	add("example.invalid/other", "v1.0.0", "package pkg\n\nfunc G(s string) {}\n")
	return root
}

func unverifiedApp(t *testing.T) (app string, env []string) {
	t.Helper()
	proxy := unverifiedProxy(t)
	app = filepath.Join(t.TempDir(), "app")
	write(t, filepath.Join(app, "go.mod"), "module example.com/app\n\ngo 1.21\n\nrequire (\n\texample.invalid/lib/v2 v2.3.0\n\texample.invalid/other v1.0.0\n)\n")
	write(t, filepath.Join(app, "main.go"), `package main

import (
	lib "example.invalid/lib/v2/pkg"
	other "example.invalid/other/pkg"
)

func main() { lib.F("x"); other.G("x") }
`)
	// the module cache is read only: it is made writable before the temporary directory is removed
	cache := filepath.Join(t.TempDir(), "modcache")
	t.Cleanup(func() {
		filepath.Walk(cache, func(p string, info os.FileInfo, err error) error {
			if err == nil {
				os.Chmod(p, 0o777)
			}
			return nil
		})
	})
	env = []string{"GOPROXY=file://" + proxy, "GOSUMDB=off", "GOFLAGS=-mod=mod", "GOMODCACHE=" + cache}
	return app, env
}

func unverifiedOf(t *testing.T, app string, env []string, rules string) []map[string]any {
	t.Helper()
	out := runIn(t, binary, app, env, "analyze", "-rules", rules, "./...")
	var r struct {
		Summary struct {
			Unverified []map[string]any `json:"unverified_versions"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return r.Summary.Unverified
}

// A module that a model has a verified_versions record for, used at a version outside the records, is said in summary.unverified_versions
// (also when there is no finding). In the range, with no record, or with a record of another module: nothing is said.
func TestUnverifiedVersionsInSummary(t *testing.T) {
	app, env := unverifiedApp(t)
	rows := "sinks:\n  - symbol: example.invalid/lib/v*/pkg.F\n    args: [0]\n    category: command_injection\n"
	rec := func(pkg, versions string) string {
		return "ruleset: t@1\nverified_versions:\n  - package: " + pkg + "\n    versions: \"" + versions + "\"\n    checked: 2026-11-23\n" + rows
	}
	// in the range of a /v* record: nothing
	if got := unverifiedOf(t, app, env, brRules(t, app, "in.yaml", rec("example.invalid/lib/v*", ">=2.0.0 <3.0.0"))); len(got) != 0 {
		t.Errorf("in the range: %v", got)
	}
	// outside: one entry, with the used version, the ranges, and where the version comes from
	got := unverifiedOf(t, app, env, brRules(t, app, "out.yaml", rec("example.invalid/lib/v*", ">=93.0.0 <94.0.0")))
	want := []map[string]any{{"package": "example.invalid/lib/v2", "used": "v2.3.0", "verified": []any{">=93.0.0 <94.0.0"}, "source": "go.mod"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("outside the range: %v, want %v", got, want)
	}
	// a record that names the module itself (no /v*), outside the range
	if got := unverifiedOf(t, app, env, brRules(t, app, "exact.yaml", rec("example.invalid/lib/v2", ">=2.4.0 <3.0.0"))); len(got) != 1 {
		t.Errorf("an exact record outside the range: %v", got)
	}
	// no record for the module: nothing, whatever the version (most built-in files have none)
	if got := unverifiedOf(t, app, env, brRules(t, app, "none.yaml", "ruleset: t@1\n"+rows)); len(got) != 0 {
		t.Errorf("no record: %v", got)
	}
	// a record of another module: nothing
	if got := unverifiedOf(t, app, env, brRules(t, app, "other.yaml", rec("example.invalid/nothing/v*", ">=93.0.0 <94.0.0"))); len(got) != 0 {
		t.Errorf("a record of another module: %v", got)
	}
}
