package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A Dart package with no package_config.json (`dart pub get` was not run, or failed) is said: its imports do not resolve and
// its functions are not named by package: URIs, so the rules that name them do not match, and nothing was found with nothing
// said. In a tree of packages only the package that has none is. A package config that is there and old (a dependency that
// the pubspec.yaml has since gained is not in it) is said too, with another reason; and the import that does not resolve is a
// missing_source as well.
func TestDartPackageWithoutPackageConfigIsSaid(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	pubGet := func(dir string) {
		get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get")...)
		get.Dir = dir
		if out, err := get.CombinedOutput(); err != nil {
			t.Fatalf("dart pub get in %s: %v\n%s", dir, err, out)
		}
	}
	pkg := func(dir, name, extraDeps string) {
		if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: "+name+"\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n"+extraDeps), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "lib", "a.dart"), []byte("void f() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	type result struct {
		Summary struct {
			Unread *struct {
				Top []struct {
					Package string `json:"package"`
					Why     string `json:"why"`
				} `json:"top"`
			} `json:"unread"`
		} `json:"summary"`
		Uncertainties []struct {
			Kind   string `json:"kind"`
			Detail string `json:"detail"`
		} `json:"uncertainties"`
	}
	var lastStderr string
	// the standard error says it in a sentence of its own (what to run), not as a type error with "0 functions"
	saidOnStderr := func(what string) {
		t.Helper()
		if !strings.Contains(lastStderr, "dart pub get") || strings.Contains(lastStderr, "a type error in them") {
			t.Errorf("%s: the standard error does not say to run dart pub get (or says it is a type error):\n%s", what, lastStderr)
		}
	}
	run := func(dir string) result {
		cmd := exec.Command(binary, "analyze", "-lang", "dart", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
		var se bytes.Buffer
		cmd.Stderr = &se
		out, err := cmd.Output()
		lastStderr = se.String()
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		var r result
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	unread := func(r result) []string {
		var out []string
		if r.Summary.Unread != nil {
			for _, p := range r.Summary.Unread.Top {
				if strings.HasPrefix(p.Why, "package_config.json is not there") {
					out = append(out, p.Package)
				}
			}
		}
		return out
	}

	// no package_config.json at all
	one := t.TempDir()
	pkg(one, "one", "")
	if got := unread(run(one)); len(got) != 1 || got[0] != "." {
		t.Errorf("a package without a package config: unread %v", got)
	}
	saidOnStderr("a package without a package config")
	// a tree of two packages: only the one that was not pub-got
	tree := t.TempDir()
	pkg(filepath.Join(tree, "a"), "a", "")
	pkg(filepath.Join(tree, "b"), "b", "")
	pubGet(filepath.Join(tree, "a"))
	if got := unread(run(tree)); len(got) != 1 || got[0] != "b" {
		t.Errorf("a tree of two packages, one pub-got: unread %v", got)
	}
	// a package config that is there, and old: the dependency was added after
	old := t.TempDir()
	pkg(old, "old", "")
	pubGet(old)
	if err := os.WriteFile(filepath.Join(old, "pubspec.yaml"), []byte("name: old\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\ndependencies:\n  path: ^1.9.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "lib", "a.dart"), []byte("import 'package:path/path.dart' as p;\nString f() => p.join('a', 'b');\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := run(old)
	if got := unread(r); len(got) != 0 {
		t.Errorf("a package config that is there is not 'not there': %v", got)
	}
	oldSaid := false
	if r.Summary.Unread != nil {
		for _, p := range r.Summary.Unread.Top {
			oldSaid = oldSaid || (p.Package == "." && strings.HasPrefix(p.Why, "package_config.json is old: pubspec.yaml declares path"))
		}
	}
	if !oldSaid {
		t.Errorf("an old package config (a dependency added after pub get): not said: %+v", r.Summary.Unread)
	}
	saidOnStderr("an old package config")
	// after pub get again: not old
	pubGet(old)
	if r2 := run(old); r2.Summary.Unread != nil {
		for _, p := range r2.Summary.Unread.Top {
			if strings.HasPrefix(p.Why, "package_config.json") {
				t.Errorf("a package config that is up to date is said to be: %+v", p)
			}
		}
	}
	missing := false
	for _, u := range r.Uncertainties {
		missing = missing || (u.Kind == "missing_source" && strings.Contains(u.Detail, "package:path/path.dart"))
	}
	if !missing {
		t.Errorf("an old package config: no missing_source for the import that does not resolve: %+v", r.Uncertainties)
	}
}
