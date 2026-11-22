package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The packages of the zoo (docs/DESIGN-package-models.md, 6.8): a cell that calls an external package lives in a module of its
// own, bench/zoo-go-pkg/<package> (Go) and bench/zoo-rust-pkg/<package> (Rust), with a skeleton of the package in it (skel/).
// The tests below find those directories (nothing lists them), so that a pull request that adds a package touches no shared
// file: not a test, not ci.yml, not a go.mod or a Cargo.toml of the zoo.
//
//   - A package is its own program: it is analysed alone (rules: its rules.yaml, and the one of the zoo of the language, for
//     zoo/mark and zoo-mark) and run alone (the three runs of the zoo). The whole-zoo checks (bench/zoo-go, bench/zoo-rust:
//     TestFuncValues*, the parity, the pure and callback harnesses ...) never read a package: the code of a skeleton (a
//     dependency, Func.Dep) is not in what they check. Only TestBuiltinRowsAreHitByTheZoo reads the packages, and it counts the
//     rows that the zoo hits, dependency or not (a cell hits the row of an external package by calling the skeleton).
//   - The harness of a package is not in the repository: Go builds it with an overlay (the same cmd/observe as the zoo's, and
//     an imports.go of the directories under cells/), Rust writes a crate in a temporary directory (the cells of src, run).
//   - The cells are named F/pkg/<package>/<cell>; the marks are in bench/zoo/marks.d/<lang>/pkg/<package>/.

func zooBenchDir(t testing.TB) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// zooPkgDirs returns the directories of the packages of the zoo of a language ("go", "rust"): the directories of
// bench/zoo-<lang>-pkg that have the manifest of the language, in the order of the names.
func zooPkgDirs(t testing.TB, lang string) []string {
	t.Helper()
	root := filepath.Join(zooBenchDir(t), "zoo-"+lang+"-pkg")
	manifest := map[string]string{"go": "go.mod", "rust": "Cargo.toml"}[lang]
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), manifest)); err != nil {
			t.Errorf("%s/%s has no %s: a package of the zoo is a module of its own", filepath.Base(root), e.Name(), manifest)
			continue
		}
		dirs = append(dirs, filepath.Join(root, e.Name()))
	}
	return dirs
}

var zooGoModuleRe = regexp.MustCompile(`(?m)^module\s+(\S+)`)

// ---- Go

// zooGoPkgOverlay writes the files of the harness of a package (cmd/observe: main.go of the zoo's, and an imports.go of the
// directories of cells/ that hold Go files) to a temporary directory, and returns the overlay file that puts them in the
// package: the directory has no cmd/observe, and nothing is generated into the repository.
func zooGoPkgOverlay(t *testing.T, dir, module string) string {
	t.Helper()
	tmp := t.TempDir()
	main, err := os.ReadFile(filepath.Join(zooRoot(t), "cmd", "observe", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	err = filepath.Walk(filepath.Join(dir, "cells"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".go") {
			rel, _ := filepath.Rel(dir, filepath.Dir(p))
			d := filepath.ToSlash(rel)
			if len(dirs) == 0 || dirs[len(dirs)-1] != d {
				dirs = append(dirs, d)
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var imp strings.Builder
	imp.WriteString("//go:build zooexec\n\npackage main\n\nimport (\n")
	for _, d := range dirs {
		fmt.Fprintf(&imp, "\t_ %q\n", module+"/"+d)
	}
	imp.WriteString(")\n")
	files := map[string]string{"main.go": string(main), "imports.go": imp.String()}
	replace := map[string]string{}
	for name, text := range files {
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		replace[filepath.Join(dir, "cmd", "observe", name)] = p
	}
	b, _ := json.Marshal(map[string]any{"Replace": replace})
	ov := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(ov, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return ov
}

// TestZooPkgGo runs each package of the Go zoo as a zoo of its own: the three runs (every case ran, every statement of the cells
// ran), the annotations held to the runs (the truth), and rivus on the cells held to the annotations.
func TestZooPkgGo(t *testing.T) {
	dirs := zooPkgDirs(t, "go")
	if len(dirs) == 0 {
		t.Skip("no package in bench/zoo-go-pkg")
	}
	rules := filepath.Join(zooRoot(t), "rules.yaml")
	for _, dir := range dirs {
		dir := dir
		t.Run(filepath.Base(dir), func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err != nil {
				t.Fatal(err)
			}
			m := zooGoModuleRe.FindSubmatch(b)
			if m == nil {
				t.Fatalf("no module line in %s/go.mod", dir)
			}
			module := string(m[1])
			work := filepath.Join(filepath.Dir(binary), "zoo-pkg", "go", filepath.Base(dir))
			obs, err := zooObserveOnce(dir, work, module+"/cells/...", zooGoPkgOverlay(t, dir, module))
			if err != nil {
				t.Fatal(err)
			}
			lines, cells := zooScan(t, dir)
			if len(cells) == 0 {
				t.Fatal("no cell (@cell) in cells/")
			}
			zooCheckCells(t, cells, obs)
			for _, c := range cells {
				if !strings.HasPrefix(c.name, "F/pkg/"+strings.ReplaceAll(filepath.Base(dir), "_", "-")+"/") {
					t.Errorf("%s: a cell of the package %s is named F/pkg/%s/<cell>", c.name, filepath.Base(dir), strings.ReplaceAll(filepath.Base(dir), "_", "-"))
				}
			}
			zooHoldLines(t, lines, cells, obs)
			c := checkAnnotated(t, dir, annotOpts{pattern: "./cells/...", extraRules: []string{rules}})
			zooHeadline(t, "zoo-go-pkg/"+filepath.Base(dir), lines, cells, obs, c)
		})
	}
}

// ---- Rust

// zooRustPkgCopy copies a package of the Rust zoo to a temporary directory, with zoo-mark next to it: the sandbox under which
// the extractor reads a crate does not read a path dependency outside the directory it was given, so the path of zoo-mark in
// Cargo.toml (../../zoo-rust/zoo-mark: it works in the repository) is rewritten to the copy.
func zooRustPkgCopy(t *testing.T, dir string) string {
	t.Helper()
	dst := t.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "target" {
			continue
		}
		if out, err := exec.Command("cp", "-R", filepath.Join(dir, e.Name()), filepath.Join(dst, e.Name())).CombinedOutput(); err != nil {
			t.Fatalf("cp %s: %v\n%s", e.Name(), err, out)
		}
	}
	if out, err := exec.Command("cp", "-R", filepath.Join(zooRustRoot(t), "zoo-mark"), filepath.Join(dst, "zoo-mark")).CombinedOutput(); err != nil {
		t.Fatalf("cp zoo-mark: %v\n%s", err, out)
	}
	os.RemoveAll(filepath.Join(dst, "zoo-mark", "target"))
	// every Cargo.toml of the copy (the crate of the package, and the skeleton crates under it) that has zoo-mark as a path
	// dependency (../../zoo-rust/zoo-mark in the repository: it works there) points to the copy of zoo-mark
	found := false
	err = filepath.Walk(dst, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Base(p) != "Cargo.toml" || strings.HasPrefix(p, filepath.Join(dst, "zoo-mark")+string(filepath.Separator)) {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out := zooRustMarkPathRe.ReplaceAllStringFunc(string(b), func(string) string {
			found = true
			rel, _ := filepath.Rel(filepath.Dir(p), filepath.Join(dst, "zoo-mark"))
			return `path = "` + filepath.ToSlash(rel) + `"`
		})
		if out == string(b) {
			return nil
		}
		return os.WriteFile(p, []byte(out), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("no Cargo.toml of the package names zoo-mark as path = \"../../zoo-rust/zoo-mark\"")
	}
	return dst
}

var zooRustMarkPathRe = regexp.MustCompile(`path\s*=\s*"(?:\.\./)+zoo-rust/zoo-mark"`)

var zooRustPkgNameRe = regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`)

// zooRustPkgHarness writes the harness crate of a package to a temporary directory: it runs the register() of every file of
// cells/src (the cases of the cells), as exec/ of the zoo does, with the recording zoo-mark.
func zooRustPkgHarness(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	m := zooRustPkgNameRe.FindSubmatch(b) // the first name is the package's
	if m == nil {
		t.Fatalf("no package name in %s/Cargo.toml", dir)
	}
	crate := strings.ReplaceAll(string(m[1]), "-", "_")
	var mods []string
	src := filepath.Join(dir, "cells", "src")
	filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".rs") && filepath.Base(p) != "lib.rs" {
			rel, _ := filepath.Rel(src, p)
			mods = append(mods, strings.ReplaceAll(strings.TrimSuffix(filepath.ToSlash(rel), ".rs"), "/", "::"))
		}
		return nil
	})
	sort.Strings(mods)
	var reg strings.Builder
	for _, m := range mods {
		fmt.Fprintf(&reg, "    c::%s::register();\n", m)
	}
	main := fmt.Sprintf(`use std::panic::{catch_unwind, AssertUnwindSafe};
use %s as c;

fn main() {
    let reverse = std::env::var("ZOO_REVERSE").as_deref() == Ok("1");
%s    let mut ids = zoo_mark::case_ids();
    if reverse {
        ids.reverse();
    }
    let mut problems = Vec::new();
    for id in ids {
        let before = zoo_mark::hit_count();
        if catch_unwind(AssertUnwindSafe(|| zoo_mark::run_case(&id))).is_err() {
            problems.push(format!("{id}: the case panicked"));
        } else if zoo_mark::hit_count() <= before {
            problems.push(format!("{id}: the case has recorded no call of a sink"));
        }
    }
    if let Ok(out) = std::env::var("ZOO_OUT") {
        if let Err(e) = std::fs::write(&out, zoo_mark::observation_json(reverse)) {
            eprintln!("cannot write {out}: {e}");
            std::process::exit(2);
        }
    }
    for p in &problems {
        eprintln!("{p}");
    }
    if !problems.is_empty() {
        std::process::exit(1);
    }
}
`, crate, reg.String())
	tmp := t.TempDir()
	toml := fmt.Sprintf(`[package]
name = "zoo-pkg-observe"
version = "0.0.0"
edition = "2021"
publish = false

[[bin]]
name = "zoo-pkg-observe"
path = "main.rs"

[dependencies]
zoo-mark = { path = %q, features = ["exec"] }
%s = { path = %q }

[workspace]
`, filepath.Join(zooRustRoot(t), "zoo-mark"), string(m[1]), dir)
	for name, text := range map[string]string{"Cargo.toml": toml, "main.rs": main} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := zooRustCargo(t, tmp, nil, "generate-lockfile", "--offline", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("cargo generate-lockfile: %v\n%s", err, out)
	}
	return tmp
}

// TestZooPkgRust is TestZooPkgGo for the packages of the Rust zoo (where the extractor and the isolation are).
func TestZooPkgRust(t *testing.T) {
	if os.Getenv("RIVUS_RUST") == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	dirs := zooPkgDirs(t, "rust")
	if len(dirs) == 0 {
		t.Skip("no package in bench/zoo-rust-pkg")
	}
	rules := filepath.Join(zooRustRoot(t), "rules.yaml")
	for _, dir := range dirs {
		dir := dir
		t.Run(filepath.Base(dir), func(t *testing.T) {
			harness := zooRustPkgHarness(t, dir)
			work := filepath.Join(filepath.Dir(binary), "zoo-pkg", "rust", filepath.Base(dir))
			if err := os.MkdirAll(work, 0o755); err != nil {
				t.Fatal(err)
			}
			obs, err := zooRustObserveAt(t, harness, work, filepath.Join(filepath.Dir(binary), "zoo-pkg", "rust-target"), "zoo-pkg-observe")
			if err != nil {
				t.Fatal(err)
			}
			lines, cells := zooScanIn(t, dir, "cells/src", ".rs", func(rel string) string { return rel })
			if len(cells) == 0 {
				t.Fatal("no cell (@cell) in cells/src")
			}
			zooCheckCells(t, cells, obs)
			for _, c := range cells {
				if !strings.HasPrefix(c.name, "F/pkg/"+strings.ReplaceAll(filepath.Base(dir), "_", "-")+"/") {
					t.Errorf("%s: a cell of the package %s is named F/pkg/%s/<cell>", c.name, filepath.Base(dir), strings.ReplaceAll(filepath.Base(dir), "_", "-"))
				}
			}
			zooHoldLines(t, lines, cells, obs)
			cp := zooRustPkgCopy(t, dir)
			c := checkCuratedIROpts(t, cp, zooRustStream(t, cp), ".rs", annotOpts{extraRules: []string{rules}, ignoreSinkUnder: "skel/"})
			zooHeadline(t, "zoo-rust-pkg/"+filepath.Base(dir), lines, cells, obs, c)
		})
	}
}
