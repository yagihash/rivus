package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The Dart zoo (bench/zoo-dart, docs/DESIGN-zoo.md): the same as the Go one (zoo_test.go), with the Dart
// tools:
//
//   - bench/zoo-dart/cells is the package that rivus analyses (the cells, and the declarations of zoo_mark);
//   - bench/zoo-dart/exec is the harness: the recording zoo_mark in its place (dependency_overrides) and
//     `dart test --coverage`;
//   - TestZooExecDart runs the cases three times (variant 0, variant 1, variant 0 in the reverse order) and
//     checks them; TestZooTruthDart holds the annotations to what was seen; TestZooDart runs rivus (the Dart
//     extractor, then the analysis) and holds its findings to the annotations; TestZooMarkIndependentDart holds
//     that the findings do not depend on the implementation of zoo_mark.
//
// They need the Dart SDK (RIVUS_DART), as TestCuratedDart does.

var (
	zooDartOnce sync.Once
	zooDartObs  zooObserved
	zooDartErr  error
)

func zooDartRoot(t testing.TB) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "zoo-dart"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// zooDartCmd is the Dart command (RIVUS_DART, for example "dart" or "mise exec dart@3.13.6 -- dart") with args.
func zooDartCmd(t *testing.T, dir string, env []string, args ...string) *exec.Cmd {
	t.Helper()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	words := strings.Fields(dart)
	cmd := exec.Command(words[0], append(append([]string{}, words[1:]...), args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

// zooDartPubGet resolves the packages of dir when that has not been done.
func zooDartPubGet(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".dart_tool", "package_config.json")); err == nil {
		return
	}
	if out, err := zooDartCmd(t, dir, nil, "pub", "get").CombinedOutput(); err != nil {
		t.Fatalf("pub get in %s: %v\n%s", dir, err, out)
	}
}

// zooDartObserve runs the cases once per test binary.
func zooDartObserve(t *testing.T) zooObserved {
	t.Helper()
	root := zooDartRoot(t)
	if os.Getenv("RIVUS_DART") == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	zooDartOnce.Do(func() { zooDartObs, zooDartErr = zooDartObserveOnce(t, root) })
	if zooDartErr != nil {
		t.Fatal(zooDartErr)
	}
	return zooDartObs
}

func zooDartObserveOnce(t *testing.T, root string) (zooObserved, error) {
	var obs zooObserved
	exe := filepath.Join(root, "exec")
	zooDartPubGet(t, exe)
	work := filepath.Join(filepath.Dir(binary), "zoo-dart")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return obs, err
	}
	type runSpec struct {
		name    string
		variant string
		reverse string
	}
	runs := []runSpec{{"v0", "0", "0"}, {"v1", "1", "0"}, {"rev", "0", "1"}}
	got := map[string]zooRun{}
	covered := map[string]bool{} // "lib/x.dart:12" -> some run ran the line
	known := map[string]bool{}
	for _, r := range runs {
		cov := filepath.Join(work, "cov-"+r.name)
		os.RemoveAll(cov)
		outFile := filepath.Join(work, r.name+".json")
		os.Remove(outFile)
		env := []string{"ZOO_VARIANT=" + r.variant, "ZOO_REVERSE=" + r.reverse, "ZOO_OUT=" + outFile}
		if out, err := zooDartCmd(t, exe, env, "test", "--coverage="+cov, "test/zoo_test.dart").CombinedOutput(); err != nil {
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

		// the lines of the cells that ran
		cb, err := os.ReadFile(filepath.Join(cov, "test", "zoo_test.dart.vm.json"))
		if err != nil {
			return obs, err
		}
		var rep struct {
			Coverage []struct {
				Source string
				Hits   []int
			}
		}
		if err := json.Unmarshal(cb, &rep); err != nil {
			return obs, err
		}
		for _, e := range rep.Coverage {
			if !strings.HasPrefix(e.Source, "package:zoo_cells/") {
				continue
			}
			file := "lib/" + strings.TrimPrefix(e.Source, "package:zoo_cells/")
			for i := 0; i+1 < len(e.Hits); i += 2 {
				k := fmt.Sprintf("%s:%d", file, e.Hits[i])
				known[k] = true
				if e.Hits[i+1] > 0 {
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

// TestZooExecDart runs the self-check of zoo_mark (the constants and the probes of bench/zoo/spec.json, in both
// variants) and the three runs of the cases.
func TestZooExecDart(t *testing.T) {
	root := zooDartRoot(t)
	exe := filepath.Join(root, "exec")
	if os.Getenv("RIVUS_DART") == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	zooDartPubGet(t, exe)
	for _, v := range []string{"0", "1"} {
		out, err := zooDartCmd(t, exe, []string{"ZOO_VARIANT=" + v}, "test", "test/spec_test.dart").CombinedOutput()
		if err != nil {
			t.Errorf("the spec of zoo_mark, variant %s: %v\n%s", v, err, out)
		}
	}
	obs := zooDartObserve(t)
	for _, p := range obs.Problems {
		t.Error(p)
	}
	for _, u := range obs.Uncovered {
		t.Errorf("a statement of the cells did not run: %s", u)
	}
	n := 0
	for _, h := range obs.Hits {
		n += len(h)
	}
	t.Logf("zoo-dart: %d cases run, %d hits", len(obs.Cases), n)
}

// zooDartScan reads the cells of the Dart zoo (a cell is a file of cells/lib).
func zooDartScan(t *testing.T, root string) ([]zooLine, []*zooCell) {
	t.Helper()
	return zooScanIn(t, filepath.Join(root, "cells"), "lib", ".dart", func(rel string) string { return rel })
}

// TestZooTruthDart holds the annotations of the Dart cells to what the cases did when they were run.
func TestZooTruthDart(t *testing.T) {
	root := zooDartRoot(t)
	obs := zooDartObserve(t)
	lines, cells := zooDartScan(t, root)
	zooCheckCells(t, cells, obs)
	// every file of the cells is registered by the harness
	cases, err := os.ReadFile(filepath.Join(root, "exec", "lib", "cases.dart"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cells {
		if !strings.Contains(string(cases), "'package:zoo_cells/"+strings.TrimPrefix(c.dir, "lib/")+"'") {
			t.Errorf("%s is not registered by exec/lib/cases.dart", c.dir)
		}
	}
	zooHoldLines(t, lines, cells, obs)
}

// zooDartStream runs the Dart extractor on the directory: the IR.
func zooDartStream(t *testing.T, dir string) []byte {
	t.Helper()
	zooDartPubGet(t, dir)
	extractor, err := filepath.Abs(filepath.Join("..", "..", "extractor", "dart"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := zooDartCmd(t, extractor, nil, "run", "bin/rivus_dart.dart", dir)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v\n%s", err, stderr.String())
	}
	return stream
}

// zooDartAlone analyses each cell by itself, as a subtest (green alone and red in the whole is a mix with the
// others; red alone is the cell).
func zooDartAlone(t *testing.T, root string, cells []*zooCell) {
	t.Helper()
	for _, c := range cells {
		c := c
		t.Run("alone/"+strings.TrimPrefix(c.name, "F/"), func(t *testing.T) {
			one := t.TempDir()
			for _, f := range []string{"pubspec.yaml", "rules.yaml"} {
				b, err := os.ReadFile(filepath.Join(root, "cells", f))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(one, f), b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, d := range []string{"packages", c.dir} {
				if out, err := exec.Command("cp", "-R", filepath.Join(root, "cells", d), mkdirParent(t, filepath.Join(one, d))).CombinedOutput(); err != nil {
					t.Fatalf("cp %s: %v\n%s", d, err, out)
				}
			}
			os.RemoveAll(filepath.Join(one, "packages", "zoo_mark", ".dart_tool"))
			checkCuratedIR(t, one, zooDartStream(t, one), ".dart")
		})
	}
}

// TestZooDart runs rivus on the Dart cells and holds the findings to the annotations (the ratchet of the curated
// projects), then prints the headline numbers.
func TestZooDart(t *testing.T) {
	root := zooDartRoot(t)
	obs := zooDartObserve(t)
	lines, cells := zooDartScan(t, root)
	cellsDir := filepath.Join(root, "cells")
	c := checkCuratedIR(t, cellsDir, zooDartStream(t, cellsDir), ".dart")
	if t.Failed() || os.Getenv("ZOO_ALONE") == "1" { // ZOO_ALONE=1 asks for it
		t.Log("the zoo failed as a whole: each cell is analysed alone below (green alone: mixed up with the others; red alone: the cell)")
		zooDartAlone(t, root, cells)
	}
	zooHeadline(t, "zoo-dart", lines, cells, obs, c)
	logSilentMisses(t, "zoo-dart", c)
}

// TestZooMarkIndependentDart holds the promise that the analysis reads only the declarations of zoo_mark: with
// the recording implementation in its place, and the registration of the cases in a table (the harness's own
// lib/cases.dart, which holds every case as a function value), the findings, their reachability and their
// confidence are the same.
func TestZooMarkIndependentDart(t *testing.T) {
	root := zooDartRoot(t)
	if os.Getenv("RIVUS_DART") == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	cellsDir := filepath.Join(root, "cells")
	findings := func(dir string) []string {
		stream := zooDartStream(t, dir)
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
	plain := findings(cellsDir)

	// the same cells, the recording zoo_mark in the package of the declarations, and the harness's table
	other := t.TempDir()
	for _, f := range []string{"pubspec.yaml", "rules.yaml"} {
		b, err := os.ReadFile(filepath.Join(cellsDir, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(other, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{"packages", "lib"} {
		if out, err := exec.Command("cp", "-R", filepath.Join(cellsDir, d), mkdirParent(t, filepath.Join(other, d))).CombinedOutput(); err != nil {
			t.Fatalf("cp %s: %v\n%s", d, err, out)
		}
	}
	os.RemoveAll(filepath.Join(other, "packages", "zoo_mark", ".dart_tool"))
	impl, err := os.ReadFile(filepath.Join(root, "exec", "zoo_mark", "lib", "zoo_mark.dart"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "packages", "zoo_mark", "lib", "zoo_mark.dart"), impl, 0o644); err != nil {
		t.Fatal(err)
	}
	table, err := os.ReadFile(filepath.Join(root, "exec", "lib", "cases.dart"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "lib", "cases.dart"), table, 0o644); err != nil {
		t.Fatal(err)
	}
	rec := findings(other)
	if strings.Join(plain, "\n") != strings.Join(rec, "\n") {
		t.Errorf("the findings change with the implementation of zoo_mark:\n plain: %v\n recording: %v", plain, rec)
	}
	if len(plain) == 0 {
		t.Error("no findings")
	}
	t.Logf("zoo-dart: %d findings, the same with the implementation of zoo_mark and the registration of the cases", len(plain))
}
