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
	"sync"
	"testing"
)

// The Rust zoo (bench/zoo-rust, docs/DESIGN-zoo.md): the same as the Go one (zoo_test.go) and the Dart one, with
// the Rust tools:
//
//   - bench/zoo-rust is the workspace that rivus analyses (the cells, and the declarations of zoo-mark). It has
//     no dependency at all: the extractor builds what it reads (build.rs, proc macros), so the zoo is code of our
//     own and std only;
//   - bench/zoo-rust/exec is the harness: a workspace of its own, with the recording zoo-mark (feature `exec`),
//     built with -C instrument-coverage by the pinned nightly (the one with llvm-tools);
//   - TestZooExecRust runs the cases three times (variant 0, variant 1, variant 0 in the reverse order) and checks
//     them; TestZooTruthRust holds the annotations to what was seen; TestZooRust runs rivus (the Rust extractor,
//     then the analysis) and holds its findings to the annotations; TestZooMarkIndependentRust holds that the
//     findings do not depend on the implementation of zoo-mark.
//
// They need the Rust extractor (RIVUS_RUST) and the toolchain of extractor/rust/rust-toolchain.toml.

var (
	zooRustOnce sync.Once
	zooRustObs  zooObserved
	zooRustErr  error
)

func zooRustRoot(t testing.TB) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "zoo-rust"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// zooRustChannel is the toolchain of the extractor (the one with the llvm tools for the coverage).
func zooRustChannel(t testing.TB) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "extractor", "rust", "rust-toolchain.toml"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^channel\s*=\s*"([^"]+)"`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no channel in extractor/rust/rust-toolchain.toml")
	}
	return string(m[1])
}

func zooRustCargo(t testing.TB, dir string, env []string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("cargo", append([]string{"+" + zooRustChannel(t)}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

// zooRustLLVM finds a tool of the llvm-tools component of the toolchain (llvm-cov, llvm-profdata).
func zooRustLLVM(t testing.TB, tool string) string {
	t.Helper()
	ch := zooRustChannel(t)
	out, err := exec.Command("rustc", "+"+ch, "--print", "sysroot").Output()
	if err != nil {
		t.Fatalf("rustc --print sysroot: %v", err)
	}
	sysroot := strings.TrimSpace(string(out))
	hits, _ := filepath.Glob(filepath.Join(sysroot, "lib", "rustlib", "*", "bin", tool))
	if len(hits) == 0 {
		t.Fatalf("%s is not in %s (rustup component add llvm-tools)", tool, sysroot)
	}
	return hits[0]
}

// zooRustObserve builds and runs the cases once per test binary.
func zooRustObserve(t *testing.T) zooObserved {
	t.Helper()
	if os.Getenv("RIVUS_RUST") == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	root := zooRustRoot(t)
	zooRustOnce.Do(func() { zooRustObs, zooRustErr = zooRustObserveOnce(t, root) })
	if zooRustErr != nil {
		t.Fatal(zooRustErr)
	}
	return zooRustObs
}

func zooRustObserveOnce(t *testing.T, root string) (zooObserved, error) {
	var obs zooObserved
	exe := filepath.Join(root, "exec")
	work := filepath.Join(filepath.Dir(binary), "zoo-rust")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return obs, err
	}
	target := filepath.Join(work, "target")
	build := zooRustCargo(t, exe, []string{"RUSTFLAGS=-C instrument-coverage", "CARGO_TARGET_DIR=" + target},
		"build", "--locked", "--offline", "--quiet", "--bin", "zoo-observe")
	if out, err := build.CombinedOutput(); err != nil {
		return obs, fmt.Errorf("building the harness: %v\n%s", err, out)
	}
	bin := filepath.Join(target, "debug", "zoo-observe")
	runs := []struct{ name, variant, reverse string }{{"v0", "0", "0"}, {"v1", "1", "0"}, {"rev", "0", "1"}}
	got := map[string]zooRun{}
	var raws []string
	for _, r := range runs {
		cov := filepath.Join(work, "cov-"+r.name)
		os.RemoveAll(cov)
		if err := os.MkdirAll(cov, 0o755); err != nil {
			return obs, err
		}
		outFile := filepath.Join(work, r.name+".json")
		os.Remove(outFile)
		cmd := exec.Command(bin)
		cmd.Dir = exe
		cmd.Env = append(os.Environ(), "ZOO_VARIANT="+r.variant, "ZOO_REVERSE="+r.reverse, "ZOO_OUT="+outFile,
			"LLVM_PROFILE_FILE="+filepath.Join(cov, "zoo-%p-%m.profraw"))
		if out, err := cmd.CombinedOutput(); err != nil {
			return obs, fmt.Errorf("running the harness (%s): %v\n%s", r.name, err, out)
		}
		b, err := os.ReadFile(outFile)
		if err != nil {
			return obs, err
		}
		var run zooRun
		if err := json.Unmarshal(b, &run); err != nil {
			return obs, err
		}
		got[r.name] = run
		m, _ := filepath.Glob(filepath.Join(cov, "*.profraw"))
		raws = append(raws, m...)
	}

	// the regions of the cells that did not run, in any of the three runs
	prof := filepath.Join(work, "zoo.profdata")
	args := append([]string{"merge", "-sparse", "-o", prof}, raws...)
	if out, err := exec.Command(zooRustLLVM(t, "llvm-profdata"), args...).CombinedOutput(); err != nil {
		return obs, fmt.Errorf("llvm-profdata: %v\n%s", err, out)
	}
	cmd := exec.Command(zooRustLLVM(t, "llvm-cov"), "export", "-instr-profile="+prof, bin)
	cmd.Stderr = os.Stderr
	cj, err := cmd.Output()
	if err != nil {
		return obs, fmt.Errorf("llvm-cov: %v", err)
	}
	var rep struct {
		Data []struct {
			Functions []struct {
				Filenames []string
				Regions   [][]int
			}
		}
	}
	if err := json.Unmarshal(cj, &rep); err != nil {
		return obs, err
	}
	known := map[string]bool{}
	covered := map[string]bool{}
	for _, d := range rep.Data {
		for _, f := range d.Functions {
			for _, r := range f.Regions {
				// [line start, column start, line end, column end, count, file id, expanded file id, kind]; kind 0 is code
				if len(r) < 8 || r[7] != 0 || r[5] != r[6] || r[5] >= len(f.Filenames) {
					continue
				}
				name := filepath.ToSlash(f.Filenames[r[5]])
				i := strings.LastIndex(name, "cells/src/")
				if i < 0 {
					continue
				}
				k := fmt.Sprintf("%s:%d:%d", name[i:], r[0], r[1])
				known[k] = true
				if r[4] > 0 {
					covered[k] = true
				}
			}
		}
	}
	for k := range known {
		if !covered[k] {
			obs.Uncovered = append(obs.Uncovered, k)
		}
	}
	sort.Strings(obs.Uncovered)
	return zooCombine(obs, got)
}

// TestZooExecRust runs the self-check of zoo-mark (the constants and the probes of bench/zoo/spec.json, in both
// variants: `cargo test` of the harness) and the three runs of the cases.
func TestZooExecRust(t *testing.T) {
	if os.Getenv("RIVUS_RUST") == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	exe := filepath.Join(zooRustRoot(t), "exec")
	for _, v := range []string{"0", "1"} {
		out, err := zooRustCargo(t, exe, []string{"ZOO_VARIANT=" + v, "CARGO_TARGET_DIR=" + filepath.Join(filepath.Dir(binary), "zoo-rust", "target-test")},
			"test", "--locked", "--offline", "--quiet").CombinedOutput()
		if err != nil {
			t.Errorf("cargo test of the harness, variant %s: %v\n%s", v, err, out)
		}
	}
	obs := zooRustObserve(t)
	for _, p := range obs.Problems {
		t.Error(p)
	}
	for _, u := range obs.Uncovered {
		t.Errorf("a region of the cells did not run: %s", u)
	}
	n := 0
	for _, h := range obs.Hits {
		n += len(h)
	}
	t.Logf("zoo-rust: %d cases run, %d hits", len(obs.Cases), n)
}

// zooRustScan reads the cells of the Rust zoo (a cell is a file of cells/src).
func zooRustScan(t *testing.T, root string) ([]zooLine, []*zooCell) {
	t.Helper()
	return zooScanIn(t, root, "cells/src", ".rs", func(rel string) string { return rel })
}

// TestZooTruthRust holds the annotations of the Rust cells to what the cases did when they were run.
func TestZooTruthRust(t *testing.T) {
	root := zooRustRoot(t)
	obs := zooRustObserve(t)
	lines, cells := zooRustScan(t, root)
	zooCheckCells(t, cells, obs)
	// every file of the cells is registered by the harness
	table, err := os.ReadFile(filepath.Join(root, "exec", "src", "cases.rs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cells {
		mod := strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(c.dir, "cells/src/"), ".rs"), "/", "::")
		if !strings.Contains(string(table), "c::"+mod+"::register()") {
			t.Errorf("%s is not registered by exec/src/cases.rs", c.dir)
		}
	}
	// and every module of lib.rs has a file with a cell (a module that is not a cell would run no case)
	zooHoldLines(t, lines, cells, obs)
}

// zooRustStream runs the Rust extractor on the directory: the IR.
func zooRustStream(t *testing.T, dir string) []byte {
	t.Helper()
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, dir)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v\n%s", err, stderr.String())
	}
	return stream
}

// zooRustCopy copies the workspace that rivus analyses (not the harness, not target/) to a new directory.
func zooRustCopy(t *testing.T, root string) string {
	t.Helper()
	dst := t.TempDir()
	for _, f := range []string{"Cargo.toml", "Cargo.lock", "rules.yaml", "zoo-mark", "cells", "dep"} {
		if out, err := exec.Command("cp", "-R", filepath.Join(root, f), filepath.Join(dst, f)).CombinedOutput(); err != nil {
			t.Fatalf("cp %s: %v\n%s", f, err, out)
		}
	}
	os.RemoveAll(filepath.Join(dst, "zoo-mark", "target"))
	return dst
}

// zooRustAlone analyses each cell by itself, as a subtest (green alone and red in the whole is a mix with the
// others; red alone is the cell).
func zooRustAlone(t *testing.T, root string, cells []*zooCell) {
	t.Helper()
	for _, c := range cells {
		c := c
		t.Run("alone/"+strings.TrimPrefix(c.name, "F/"), func(t *testing.T) {
			one := zooRustCopy(t, root)
			// only this cell: lib.rs lists the one module
			mod := strings.TrimSuffix(strings.TrimPrefix(c.dir, "cells/src/"), ".rs")
			parts := strings.Split(mod, "/")
			lib := "pub mod " + parts[0] + " { pub mod " + parts[1] + "; }\n"
			if err := os.WriteFile(filepath.Join(one, "cells", "src", "lib.rs"), []byte(lib), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, d := range zooRustModFiles(t, filepath.Join(one, "cells", "src")) {
				if filepath.ToSlash(strings.TrimPrefix(d, filepath.Join(one, "cells", "src")+"/")) != mod+".rs" {
					os.Remove(d)
				}
			}
			checkCuratedIR(t, one, zooRustStream(t, one), ".rs")
		})
	}
}

func zooRustModFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".rs") && filepath.Base(p) != "lib.rs" {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// TestZooRust runs rivus on the Rust cells and holds the findings to the annotations (the ratchet of the curated
// projects), then prints the headline numbers.
func TestZooRust(t *testing.T) {
	root := zooRustRoot(t)
	obs := zooRustObserve(t)
	lines, cells := zooRustScan(t, root)
	rec := false // a chain of 13 and 30 stages stops by the pass limit (the Go zoo has the same)
	for _, l := range lines {
		for _, k := range l.missKinds {
			rec = rec || k == "recursion_approximation"
		}
	}
	c := checkCuratedIROpts(t, root, zooRustStream(t, root), ".rs", annotOpts{allowUnconverged: rec, programWide: rec})
	if t.Failed() {
		t.Log("the zoo failed as a whole: ZOO_ALONE=1 analyses each cell alone (green alone: mixed up with the others; red alone: the cell)")
	}
	if os.Getenv("ZOO_ALONE") == "1" { // asked for: the extractor runs once for each cell
		zooRustAlone(t, root, cells)
	}
	zooHeadline(t, "zoo-rust", lines, cells, obs, c)
	logSilentMisses(t, "zoo-rust", c)
	zooParity(t, lines, cells)
}

// zooParity compares the cells that the Go zoo and the Rust zoo share (the same name, and the same tag for the
// vulnerable case): of the flows that rivus finds in Go, the ones that it does not find in Rust. Each of them has
// @ref(PARITY) in the Rust cell (the review of the precision gap between the languages looks for that name).
func zooParity(t *testing.T, rust []zooLine, rustCells []*zooCell) {
	t.Helper()
	goRoot := zooRoot(t)
	goLines, goCells := zooScan(t, goRoot)
	goHas := map[string]bool{}
	for _, c := range goCells {
		goHas[c.name] = true
	}
	common := 0
	rustHas := map[string]bool{}
	for _, c := range rustCells {
		rustHas[c.name] = true
		if goHas[c.name] {
			common++
		}
	}
	// a shape that Rust does not have is told in cells/src/lib.rs: @na(F/<family>/<shape>: the reason)
	lib, err := os.ReadFile(filepath.Join(zooRustRoot(t), "cells", "src", "lib.rs"))
	if err != nil {
		t.Fatal(err)
	}
	na, todo := zooParityMarks(string(lib))
	var naNames, todoNames []string
	for _, c := range goCells {
		_, isNa := na[c.name]
		_, isTodo := todo[c.name]
		switch {
		case rustHas[c.name]:
			if isNa || isTodo {
				t.Errorf("%s: the Rust zoo has the cell, and lib.rs says @na or @todo of it", c.name)
			}
		case isNa && isTodo:
			t.Errorf("%s is both @na and @todo in cells/src/lib.rs", c.name)
		case isNa:
			naNames = append(naNames, strings.TrimPrefix(c.name, "F/"))
		case isTodo:
			todoNames = append(todoNames, strings.TrimPrefix(c.name, "F/"))
		default:
			t.Errorf("%s is a cell of the Go zoo: the Rust zoo has it, or says @na(%s: the reason) (the form does not exist in Rust) or @todo(%s: PARITY: the reason) (it can be written, not yet) in cells/src/lib.rs", c.name, c.name, c.name)
		}
	}
	zooParityLine(t, "zoo-rust", naNames, todoNames)
	goMiss := map[string]bool{}
	goTags := map[string]bool{}
	goTagsAll := map[string]bool{} // every tag of the Go zoo (a source tag)
	for _, l := range goLines {
		for _, tag := range l.sourceTags {
			goTagsAll[tag] = true
		}
		for _, tag := range l.sourceTags {
			if zooTagRole.FindStringSubmatch(tag) == nil || zooTagRole.FindStringSubmatch(tag)[1] != "vuln" {
				continue
			}
			goTags[tag] = true
			if l.hasMiss {
				goMiss[tag] = true
			}
		}
	}
	// the decoys and the fixed cases that rivus reports wrongly in Rust and not in Go (the same tag)
	goFp := map[string]bool{}
	for _, l := range goLines {
		if l.hasFp {
			for _, tag := range l.nosinks {
				goFp[tag] = true
			}
		}
	}
	var fpGap []string
	for _, l := range rust {
		if l.hasFp {
			for _, tag := range l.nosinks {
				if !goFp[tag] && goTagsAll[tag] {
					fpGap = append(fpGap, tag)
					if !zooContains(l.refs, "PARITY") {
						t.Errorf("%s:%d: %s is quiet in the Go zoo and reported in the Rust one: the @fp needs @ref(PARITY)", l.file, l.line, tag)
					}
				}
			}
		}
	}
	sort.Strings(fpGap)
	var gap, bothMiss, both []string
	foundRust := 0
	rustVuln := 0
	for _, l := range rust {
		for _, tag := range l.sourceTags {
			if m := zooTagRole.FindStringSubmatch(tag); m == nil || m[1] != "vuln" {
				continue
			}
			rustVuln++
			if !l.hasMiss {
				foundRust++
			}
			if !goTags[tag] {
				continue
			}
			switch {
			case !goMiss[tag] && l.hasMiss:
				gap = append(gap, tag)
				if !zooContains(l.refs, "PARITY") {
					t.Errorf("%s:%d: %s is found in the Go zoo and not in the Rust one: the @miss needs @ref(PARITY)", l.file, l.line, tag)
				}
			case goMiss[tag] && l.hasMiss:
				bothMiss = append(bothMiss, tag)
			default:
				both = append(both, tag)
			}
		}
	}
	sort.Strings(gap)
	t.Logf("zoo-rust: parity with zoo-go: common cells %d | common vulnerable flows %d (found in both %d, missed in both %d) | found in Go and not in Rust %d %v | decoys reported in Rust and not in Go %d %v", common, len(gap)+len(bothMiss)+len(both), len(both), len(bothMiss), len(gap), gap, len(fpGap), fpGap)
	t.Logf("zoo-rust: vulnerable flows found %d / %d", foundRust, rustVuln)
}

// TestZooMarkIndependentRust holds the promise that the analysis reads only the declarations of zoo-mark: with the
// recording implementation in its place (the feature `exec` on by default), and the registration of the cases in a
// table (the harness's own exec/src/cases.rs, which holds every case as a function value), the findings, their
// reachability and their confidence are the same.
func TestZooMarkIndependentRust(t *testing.T) {
	if os.Getenv("RIVUS_RUST") == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	root := zooRustRoot(t)
	findings := func(dir string) []string {
		stream := zooRustStream(t, dir)
		irFile := filepath.Join(t.TempDir(), "zoo.jsonl")
		if err := os.WriteFile(irFile, stream, 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "analyze", "-sites", "-ir", irFile, "-rules", filepath.Join(dir, "rules.yaml"))
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		return zooParseFindings(t, out)
	}
	plain := findings(root)

	other := zooRustCopy(t, root)
	manifest := filepath.Join(other, "zoo-mark", "Cargo.toml")
	b, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(strings.Replace(string(b), "[features]\n", "[features]\ndefault = [\"exec\"]\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	table, err := os.ReadFile(filepath.Join(root, "exec", "src", "cases.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "cells", "src", "cases.rs"), []byte(strings.Replace(string(table), "use zoo_cells as c;", "use crate as c;", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	libPath := filepath.Join(other, "cells", "src", "lib.rs")
	lib, err := os.ReadFile(libPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(libPath, append(lib, []byte("pub mod cases;\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := findings(other)
	if strings.Join(plain, "\n") != strings.Join(rec, "\n") {
		in := func(l []string, s string) bool {
			for _, x := range l {
				if x == s {
					return true
				}
			}
			return false
		}
		for _, f := range plain {
			if !in(rec, f) {
				t.Errorf("only without the recording implementation: %s", f)
			}
		}
		for _, f := range rec {
			if !in(plain, f) {
				t.Errorf("only with the recording implementation: %s", f)
			}
		}
	}
	if len(plain) == 0 {
		t.Error("no findings")
	}
	t.Logf("zoo-rust: %d findings, the same with the implementation of zoo-mark and the registration of the cases", len(plain))
}
