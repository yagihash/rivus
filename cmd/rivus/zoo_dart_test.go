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
	zooDartGenerateCases(t, dir)
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
	// the config is made, and not older than the pubspec (a skeleton package that was added)
	if cfg, err := os.Stat(filepath.Join(dir, ".dart_tool", "package_config.json")); err == nil {
		if ps, err := os.Stat(filepath.Join(dir, "pubspec.yaml")); err != nil || !ps.ModTime().After(cfg.ModTime()) {
			return
		}
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
	// every file of the cells is registered by the harness (exec/lib/cases.dart is generated from the files: zooDartRoot)
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
	cmd := zooDartCmd(t, extractor, nil, "run", "bin/rivus_dart.dart", "--dep-dir", "packages", dir) // the skeletons of cells/packages are read as dependencies (a path dependency is under the project, and would be read as the project's own)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v\n%s", err, stderr.String())
	}
	return stream
}

// zooDartCopy copies the given cells of the Dart zoo (and what they need: pubspec.yaml, rules.yaml, packages) to a new
// directory, to be analysed as a program of its own.
func zooDartCopy(t *testing.T, root string, cells []*zooCell) string {
	t.Helper()
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
	for _, d := range append([]string{"packages"}, zooCellDirs(cells)...) {
		if out, err := exec.Command("cp", "-R", filepath.Join(root, "cells", d), mkdirParent(t, filepath.Join(one, d))).CombinedOutput(); err != nil {
			t.Fatalf("cp %s: %v\n%s", d, err, out)
		}
	}
	os.RemoveAll(filepath.Join(one, "packages", "zoo_mark", ".dart_tool"))
	return one
}

// zooDartAlone analyses each cell by itself, as a subtest (green alone and red in the whole is a mix with the
// others; red alone is the cell).
func zooDartAlone(t *testing.T, root string, cells []*zooCell) {
	t.Helper()
	for _, c := range cells {
		c := c
		t.Run("alone/"+strings.TrimPrefix(c.name, "F/"), func(t *testing.T) {
			one := zooDartCopy(t, root, []*zooCell{c})
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
	// a chain of 13 and 30 stages stops by the pass limit (the Go and Rust zoos have the same): those cells are a program of
	// their own (zooSplit), and the others must converge
	rest, recCells := zooSplit(lines, cells)
	restDir := zooDartCopy(t, root, rest)
	c := checkCuratedIROpts(t, restDir, zooDartStream(t, restDir), ".dart", annotOpts{})
	if len(recCells) > 0 {
		zooCheckRecNames(t, recCells)
		recDir := zooDartCopy(t, root, recCells)
		c.add(checkCuratedIROpts(t, recDir, zooDartStream(t, recDir), ".dart", annotOpts{allowUnconverged: true, programWide: true}))
	}
	_ = cellsDir
	if t.Failed() {
		t.Log("the zoo failed as a whole: ZOO_ALONE=1 analyses each cell alone (green alone: mixed up with the others; red alone: the cell; the Dart extractor takes about 10 s for each)")
	}
	if os.Getenv("ZOO_ALONE") == "1" { // asked for: the extractor runs once for each cell
		zooDartAlone(t, root, cells)
	}
	zooHeadline(t, "zoo-dart", lines, cells, obs, c)
	zooParityDart(t, lines, cells)
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
		err = notFailed(err) // exit code 4: the zoo has cells that stop by the pass limit on purpose (F/state/global-chain); TestZoo* hold that the others converge
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
	t.Logf("zoo-dart: %d findings, the same with the implementation of zoo_mark and the registration of the cases", len(plain))
}

// zooVulnMisses counts the @miss of the vulnerable cases of a cell (and the @ref of them).
func zooVulnMisses(lines []zooLine, c *zooCell) (miss int, refs [][]string) {
	for _, l := range lines {
		if !(l.file == c.dir || strings.HasPrefix(l.file, c.dir+"/")) {
			continue
		}
		for _, tag := range l.sourceTags {
			if m := zooTagRole.FindStringSubmatch(tag); m == nil || m[1] != "vuln" {
				continue
			}
			if l.hasMiss {
				miss++
				refs = append(refs, l.refs)
			}
		}
	}
	return
}

// zooParityDart compares the cells of the Dart zoo with the ones of the Go zoo (the common forms have the same name,
// docs/DESIGN-zoo.md 4.2.2): a common cell in which Go finds the flow and Dart has a @miss says @ref(..., PARITY);
// a Dart cell that Go does not have says why in @na(go: reason) (no such form in Go) or says where Go measures it in
// @elsewhere(go: <a cell of zoo-go-pkg or a case of bench/gcp>), which is checked to be there; a Go cell that Dart does not have is in
// bench/zoo-dart/na.txt (@na(F/<family>/<shape>: reason), a form that Dart has no counterpart of) or is still to be
// written (stage 3), which is logged.
func zooParityDart(t *testing.T, lines []zooLine, cells []*zooCell) {
	t.Helper()
	goLines, goCells := zooScan(t, zooRoot(t))
	vulnMisses := zooVulnMisses
	goCell := map[string]*zooCell{}
	for _, c := range goCells {
		goCell[c.name] = c
	}
	common, parity, onlyDart, onlyDartPending := 0, 0, 0, 0
	var goNa, goTodo, goElsewhere, goElsewhereMiss []string // the cells that only Dart has: Go's side of the parity marks
	targets := zooElsewhereTargets(t)
	dartHas := map[string]bool{}
	dartCell := map[string]*zooCell{}
	for _, c := range cells {
		dartHas[c.name] = true
		dartCell[c.name] = c
		gc := goCell[c.name]
		for _, e := range zooElsewhereErrors(c, targets, gc != nil) {
			t.Error(e)
		}
		if gc == nil {
			if len(c.elsewhere) > 0 {
				goElsewhere = append(goElsewhere, strings.TrimPrefix(c.name, "F/"))
				if ref, ok := zooElsewhereRef(c); ok && targets[ref] {
					goElsewhereMiss = append(goElsewhereMiss, strings.TrimPrefix(c.name, "F/"))
				}
			} else if len(c.na) == 0 {
				t.Logf("%s: a common form, and zoo-go has no cell of this name yet", c.name)
				onlyDartPending++
				goTodo = append(goTodo, strings.TrimPrefix(c.name, "F/"))
			} else {
				onlyDart++
				goNa = append(goNa, strings.TrimPrefix(c.name, "F/"))
			}
			continue
		}
		if len(c.na) > 0 {
			t.Errorf("%s: has @na(%s), and zoo-go has a cell of this name", c.name, c.na[0])
		}
		common++
		gm, _ := vulnMisses(goLines, gc)
		m, refs := vulnMisses(lines, c)
		if gm == 0 && m > 0 {
			parity++
			for _, r := range refs {
				if !zooContains(r, "PARITY") {
					t.Errorf("%s: a @miss of a common cell that zoo-go finds needs @ref(..., PARITY)", c.name)
				}
			}
		}
	}
	nab, err := os.ReadFile(filepath.Join(zooDartRoot(t), "na.txt"))
	if err != nil {
		t.Fatal(err)
	}
	na, todo := zooParityMarks(string(nab))
	var naNames, todoNames []string
	notYet, notApplicable := 0, 0
	for _, c := range goCells {
		_, isNa := na[c.name]
		_, isTodo := todo[c.name]
		switch {
		case dartHas[c.name]:
			if isNa || isTodo {
				t.Errorf("%s: the Dart zoo has the cell, and na.txt says @na or @todo of it", c.name)
			}
		case isNa && isTodo:
			t.Errorf("%s is both @na and @todo in na.txt", c.name)
		case isNa:
			notApplicable++
			naNames = append(naNames, strings.TrimPrefix(c.name, "F/"))
		default: // @todo, or no mark (stage 3, not written yet): both are "not yet"
			notYet++
			todoNames = append(todoNames, strings.TrimPrefix(c.name, "F/"))
		}
	}
	zooParityLine(t, "zoo-dart", naNames, todoNames, nil, nil)
	markedDart := 0
	for _, n := range todoNames {
		if _, ok := todo["F/"+n]; ok {
			markedDart++
		}
	}
	t.Logf("zoo-dart: @todo %d, marked in na.txt %d, not classified %d", len(todoNames), markedDart, len(todoNames)-markedDart)
	// the other direction: the cells that Dart has and Go does not (@na(go: reason): no such form in Go; @elsewhere(go: where):
	// measured in another place; a common form
	// that zoo-go has not written yet is "not yet")
	zooParityLine(t, "zoo-go", goNa, goTodo, goElsewhere, goElsewhereMiss)
	// bench/zoo-go/parity-todo.txt: the marked part of goTodo (the names are counted once, in goTodo)
	gnb, err := os.ReadFile(filepath.Join(zooRoot(t), "parity-todo.txt"))
	if err != nil {
		t.Fatal(err)
	}
	goHas := map[string]bool{}
	for n := range goCell {
		goHas[n] = true
	}
	marked, errs := zooGoMarkErrors(string(gnb), dartCell, goHas)
	for _, e := range errs {
		t.Error(e)
	}
	t.Logf("zoo-go: @todo %d, marked in parity-todo.txt %d, not classified %d", len(goTodo), len(marked), len(goTodo)-len(marked))
	// the five classes of the cells that Dart has, against Go (they add up to the cells of the Dart zoo)
	t.Logf("zoo-go: parity classes: common %d | @na %d | @elsewhere %d | @todo %d | not classified %d | the cells of the Dart zoo %d", common, len(goNa), len(goElsewhere), len(marked), len(goTodo)-len(marked), len(cells))
	t.Logf("zoo-dart: common cells %d | found in zoo-go and not in Dart %d (@ref(PARITY)) | only in Dart: %d with @na(go: reason), %d common and not yet in zoo-go | cells of zoo-go that Dart has no counterpart of (na.txt) %d, not written yet (stage 3) %d", common, parity, onlyDart, onlyDartPending, notApplicable, notYet)
}

// zooGoMarkErrors checks bench/zoo-go/parity-todo.txt, the Go side of the parity marks of the cells that only Dart has:
// @todo(F/<family>/<shape>: PARITY: why) names a Dart cell that has no @na(go: ..) and that zoo-go has no cell of
// (the count of the unmarked ones is "not classified"); @na is not written here (a Dart cell has @na(go: ..)).
func zooGoMarkErrors(src string, dart map[string]*zooCell, goHas map[string]bool) (marked map[string]string, errs []string) {
	na, marked := zooParityMarks(src)
	for n := range na {
		errs = append(errs, fmt.Sprintf("%s: @na in zoo-go/parity-todo.txt (write @na(go: reason) in the Dart cell)", n))
	}
	names := make([]string, 0, len(marked))
	for n := range marked {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		switch c := dart[n]; {
		case c == nil:
			errs = append(errs, fmt.Sprintf("%s: @todo in zoo-go/parity-todo.txt, and the Dart zoo has no cell of this name", n))
		case len(c.na) > 0:
			errs = append(errs, fmt.Sprintf("%s: @todo in zoo-go/parity-todo.txt, and the Dart cell has @na(%s)", n, c.na[0]))
		case len(c.elsewhere) > 0:
			errs = append(errs, fmt.Sprintf("%s: @todo in zoo-go/parity-todo.txt, and the Dart cell has @elsewhere(%s)", n, c.elsewhere[0]))
		case goHas[n]:
			errs = append(errs, fmt.Sprintf("%s: @todo in zoo-go/parity-todo.txt, and zoo-go has a cell of this name", n))
		}
	}
	return
}
