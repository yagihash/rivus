package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
)

// The curated benchmarks (bench/<lang>-curated) are small projects in the shape of
// real ones, with the flows that must be found and the ones that must not be
// written into the source, in the way of the samples (see checkSampleFlows):
//
//	x := c.Query("q")  // @source(tag)
//	db.Exec(x)         // @sink(tag)
//	db.Exec("c")       // @sink(none)
//
// Two marks say what rivus does not do yet, so that the benchmark can be run
// as a ratchet. They are checked from both sides: when the engine changes and the
// mark is no longer true, the test fails and the mark is to be removed.
//
//	x := <-ch          // @source(tag) @miss(silent)             (a flow that is there, and is not found)
//	x := <-ch          // @source(tag) @miss(unresolved_callee)  (the same, and the engine says why)
//	db.Exec(y)         // @sink(none) @fp                        (a sink that must stay quiet, and is reported)
//
// A @miss says whether the engine left a trace of what it lost: "silent" means that the function
// of the source and the function of the sink hold no uncertainty (see missTraces: a
// dynamic_dispatch with one candidate says nothing about the lost flow, and counts as no trace);
// a kind (unresolved_callee, unmodeled_library_call, dynamic_dispatch, depth_limit; several
// separated by a comma) means that an uncertainty of that kind is in one of the two functions.
// Both are checked: a trace that is gone, and a trace that has come ("silent" no longer true:
// an improvement, write the kind) fail the test. A fix that adds the trace of a lost flow
// shows in this way.
//
// "@noisy(N)" goes on a sink line (with its @sink mark) that N sources of other tags
// reach, wrongly, and that no @nosink names: the callbacks that are called by a
// helper whose candidates are all the closures of one type (Rust, Dart) do this to
// every closure that holds a sink. The flows are counted, not named; the test fails
// when the count changes (fewer: write the new number, it is an improvement).
//
// "@nosink(tag)" says that the source of the tag must not reach this line (the
// line has a flow of its own, from another source: two keys of one map, say).
// It takes "@fp" in the same way.
var curatedNoisy = regexp.MustCompile(`@noisy\((\d+)\)`)
var curatedAnnot = regexp.MustCompile(`@(source|sink|nosink)\(([A-Za-z0-9_-]+)\)(\s+@(miss(?:\(([a-z_,]+)\))?|fp))?`)

// curatedAllowUnconverged is set by a test that holds a run that does not converge (TestScaleSynth: a chain longer than
// the passes the engine makes); its expected file says so.
var curatedAllowUnconverged bool

type curatedAt struct {
	file string
	line int
}

// curatedCounts is what a curated project holds.
type curatedCounts struct {
	detect, quiet, miss, fp, noisy int
	// fpTrace and fpSilent split the sinks that must stay quiet and are reported (@fp): a finding at the sink carries a
	// container_element_imprecision (an over-approximation that says so), or none does (a false report with no trace)
	fpTrace, fpSilent int
	// missSilent and missTrace split miss: the engine left no trace of the flow it lost,
	// or an uncertainty of a kind says so.
	missSilent, missTrace int
	// offSource is the number of findings that have no step in the function of their source
	// (see offSourcePath), and what they are: a flow that is to be found ("detect"), one of
	// another source into a noisy sink ("noisy"), a reported flow that must not be there
	// ("fp"), or another ("other").
	offSource int
	offKinds  map[string]int
	// uncertainties of the run, for the tests that look at a kind
	uncertainties []*taint.Uncertainty
}

// hasUncertainty reports whether the run recorded an uncertainty of the kind, in a file whose
// path ends in file, with a detail that starts with detail.
func (c curatedCounts) hasUncertainty(kind, file, detail string) bool {
	for _, u := range c.uncertainties {
		if u.Kind == kind && strings.HasSuffix(u.Position.File, file) && strings.HasPrefix(u.Detail, detail) {
			return true
		}
	}
	return false
}

// checkCurated analyses the Go project in dir (a Go module with rules.yaml) and
// compares the findings with the annotations of its .go files.
func checkCurated(t *testing.T, dir string) curatedCounts {
	t.Helper()
	return checkAnnotated(t, dir, annotOpts{pattern: "./..."})
}

// annotOpts are what the annotated projects other than the curated ones change (the zoo, bench/zoo-go):
// the package pattern to analyse, a run that does not converge on purpose (a flow that the engine
// loses by its limit: the @miss says so), and the uncertainties of the whole program (no function)
// counted as the trace of a @miss.
type annotOpts struct {
	pattern          string
	allowUnconverged bool
	programWide      bool
}

// checkAnnotated is checkCurated for a project with its own options.
func checkAnnotated(t *testing.T, dir string, o annotOpts) curatedCounts {
	t.Helper()
	cmd := exec.Command(binary, "analyze", "-sites", "-rules", "rules.yaml", o.pattern)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("analyze: %v\n%s", err, stderr)
	}
	return checkCuratedReportOpts(t, dir, out, ".go", o)
}

// checkCuratedIR is checkCurated for a project that an extractor reads (Dart,
// Rust): stream is the IR the extractor wrote for dir, and the annotations are
// in the files with the extension ext.
func checkCuratedIR(t *testing.T, dir string, stream []byte, ext string) curatedCounts {
	t.Helper()
	irFile := filepath.Join(t.TempDir(), "curated.jsonl")
	if err := os.WriteFile(irFile, stream, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "analyze", "-sites", "-ir", irFile, "-rules", filepath.Join(dir, "rules.yaml"))
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("analyze: %v\n%s", err, stderr)
	}
	return checkCuratedReport(t, dir, out, ext)
}

// checkCuratedReport compares the report out with the annotations of the files
// of dir that end in ext.
func checkCuratedReport(t *testing.T, dir string, out []byte, ext string) curatedCounts {
	t.Helper()
	return checkCuratedReportOpts(t, dir, out, ext, annotOpts{})
}

func checkCuratedReportOpts(t *testing.T, dir string, out []byte, ext string, o annotOpts) curatedCounts {
	t.Helper()
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.LoadErrors) != 0 {
		t.Fatalf("load errors: %v", rep.LoadErrors)
	}

	// A flow that was found because a closure was taken to be called (assumed_call, C-7) is not
	// without uncertainty on its path: the finding says so, for every finding of every project.
	assumed := 0
	for _, fd := range rep.Findings {
		for _, u := range fd.Uncertainties {
			if u.Kind == taint.KindAssumedCall {
				assumed++
				if fd.Confidence == taint.ConfidenceClear {
					t.Errorf("the finding %s:%d -> %s:%d has an assumed_call and its confidence is %s", fd.Source.Position.File, fd.Source.Position.Line, fd.Sink.Position.File, fd.Sink.Position.Line, fd.Confidence)
				}
				break
			}
		}
	}
	t.Logf("%d findings have an assumed_call on their path", assumed)

	src, sinks := map[string]curatedAt{}, map[string][]curatedAt{}
	miss := map[string]bool{}
	missKinds := map[string][]string{} // tag -> the kinds of its @miss(...)
	var none []curatedAt
	fp := map[curatedAt]bool{}
	type notFlow struct {
		tag string
		at  curatedAt
		fp  bool
	}
	var nots []notFlow
	noisyWant := map[curatedAt]int{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".dart_tool", "target", ".rivus":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ext) {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			here := curatedAt{rel, n}
			if m := curatedNoisy.FindStringSubmatch(sc.Text()); m != nil {
				v, _ := strconv.Atoi(m[1])
				noisyWant[here] = v
			}
			for _, m := range curatedAnnot.FindAllStringSubmatch(sc.Text(), -1) {
				switch {
				case m[1] == "nosink":
					nots = append(nots, notFlow{m[2], here, m[4] == "fp"})
				case m[1] == "source":
					if _, dup := src[m[2]]; dup {
						t.Errorf("the tag %s has two sources", m[2])
					}
					src[m[2]] = here
					if strings.HasPrefix(m[4], "miss") {
						miss[m[2]] = true
						if m[5] == "" {
							t.Errorf("the @miss of %s at %s:%d needs the kind of its trace: @miss(silent) or @miss(<kind>)", m[2], rel, n)
						} else {
							missKinds[m[2]] = strings.Split(m[5], ",")
						}
					}
				case m[2] == "none":
					none = append(none, here)
					if m[4] == "fp" {
						fp[here] = true
					}
				default:
					sinks[m[2]] = append(sinks[m[2]], here)
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		t.Fatal(err)
	}

	pair := func(a, b curatedAt) string { return fmt.Sprintf("%s:%d -> %s:%d", a.file, a.line, b.file, b.line) }
	want := map[string]string{} // pair -> tag
	ends := map[string][2]curatedAt{}
	for tag, s := range src {
		if len(sinks[tag]) == 0 {
			t.Errorf("the tag %s has a source and no sink", tag)
		}
		for _, k := range sinks[tag] {
			want[pair(s, k)] = tag
			ends[pair(s, k)] = [2]curatedAt{s, k}
		}
	}
	for tag := range sinks {
		if _, ok := src[tag]; !ok {
			t.Errorf("the tag %s has a sink and no source", tag)
		}
	}
	got := map[string]bool{}
	skip := map[string]bool{}
	hitNone := map[curatedAt]bool{}
	for _, fd := range rep.Findings {
		s := curatedAt{fd.Source.Position.File, fd.Source.Position.Line}
		k := curatedAt{fd.Sink.Position.File, fd.Sink.Position.Line}
		got[pair(s, k)] = true
		// the sources of the same kind that reach the sink too are folded into the finding of the
		// one with the best path (which is chosen by the paths, not by the file): they are found
		for _, o := range fd.OtherSources {
			got[pair(curatedAt{o.Position.File, o.Position.Line}, k)] = true
		}
		for _, n := range none {
			if k == n {
				hitNone[n] = true
			}
		}
	}

	var c curatedCounts
	c.uncertainties = rep.Uncertainties
	var problems []string
	for _, nf := range nots {
		s, ok := src[nf.tag]
		if !ok {
			problems = append(problems, "@nosink names the unknown tag "+nf.tag)
			continue
		}
		reported := got[pair(s, nf.at)]
		switch {
		case nf.fp && !reported:
			problems = append(problems, fmt.Sprintf("quiet now (remove @fp): %s [%s] -> %s:%d", s.file, nf.tag, nf.at.file, nf.at.line))
		case nf.fp:
			c.fp++
		case reported:
			problems = append(problems, fmt.Sprintf("unexpected flow (must stay quiet): %s:%d -> %s:%d [%s]", s.file, s.line, nf.at.file, nf.at.line, nf.tag))
		default:
			c.quiet++
		}
		skip[pair(s, nf.at)] = true
	}
	for k, tag := range want {
		switch {
		case miss[tag] && got[k]:
			problems = append(problems, fmt.Sprintf("found now (remove @miss): %s [%s]", k, tag))
		case miss[tag]:
			c.miss++
			problems = append(problems, checkMissTrace(t, rep, k, tag, missKinds[tag], ends[k], &c, o.programWide)...)
		case !got[k]:
			problems = append(problems, fmt.Sprintf("missing flow: %s [%s]", k, tag))
		default:
			c.detect++
		}
	}
	c.offKinds = map[string]int{}
	for _, fd := range offSourcePath(&rep) {
		s := curatedAt{fd.Source.Position.File, fd.Source.Position.Line}
		k := curatedAt{fd.Sink.Position.File, fd.Sink.Position.Line}
		kind := "other"
		_, wanted := want[pair(s, k)]
		_, noisy := noisyWant[k]
		switch {
		case wanted:
			kind = "detect"
		case hitNone[k] || skip[pair(s, k)]:
			kind = "fp"
		case noisy:
			kind = "noisy"
		}
		c.offKinds[kind]++
		c.offSource++
		if os.Getenv("RIVUS_CURATED_TRACES") == "1" {
			t.Logf("OFFSOURCE %s %s (source function %s)", kind, pair(s, k), fd.Source.Func)
		}
	}
	noisyGot := map[curatedAt]int{}
	for _, fd := range rep.Findings {
		// a flow into a quiet sink is told below, with the sink
		k := fmt.Sprintf("%s:%d -> %s:%d", fd.Source.Position.File, fd.Source.Position.Line, fd.Sink.Position.File, fd.Sink.Position.Line)
		if hitNone[curatedAt{fd.Sink.Position.File, fd.Sink.Position.Line}] {
			continue
		}
		if _, ok := want[k]; !ok && !skip[k] {
			at := curatedAt{fd.Sink.Position.File, fd.Sink.Position.Line}
			if _, noisy := noisyWant[at]; noisy {
				noisyGot[at]++
				c.noisy++
				continue
			}
			problems = append(problems, "unexpected flow: "+k)
		}
	}
	for at, w := range noisyWant {
		if noisyGot[at] != w {
			problems = append(problems, fmt.Sprintf("@noisy(%d) at %s:%d, and %d other flows reach it now", w, at.file, at.line, noisyGot[at]))
		}
	}
	for _, n := range none {
		switch {
		case fp[n] && !hitNone[n]:
			problems = append(problems, fmt.Sprintf("quiet now (remove @fp): %s:%d", n.file, n.line))
		case fp[n]:
			c.fp++
			traced := false
			for _, fd := range rep.Findings {
				if (curatedAt{fd.Sink.Position.File, fd.Sink.Position.Line}) != n {
					continue
				}
				for _, u := range fd.Uncertainties {
					traced = traced || u.Kind == taint.KindContainerImprecision
				}
			}
			if traced {
				c.fpTrace++
			} else {
				c.fpSilent++
			}
		case hitNone[n]:
			// reported above as an unexpected flow; say it is a sink that must stay quiet
			problems = append(problems, fmt.Sprintf("a flow reaches the sink at %s:%d, which must stay quiet", n.file, n.line))
		default:
			c.quiet++
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
	// unexpected flows into a quiet sink are reported twice, by design: the first
	// names the pair, the second the sink.
	if !rep.Stats.Converged && !curatedAllowUnconverged && !o.allowUnconverged {
		t.Errorf("the analysis did not converge: %+v", rep.Stats)
	}
	return c
}

// logSilentMisses logs the share of the @miss that left no trace: "silent misses: S / M (P%)".
// It is the measure of how much of what the engine loses it also says nothing about (the
// definition and the numbers are in bench/README.md); the work that adds traces (DART-15,
// CORE-D, C-7) is to bring it down. It is a log, not a ceiling: the per-@miss check is above.
func logSilentMisses(t *testing.T, name string, c curatedCounts) {
	t.Helper()
	pct := 0.0
	if c.miss > 0 {
		pct = 100 * float64(c.missSilent) / float64(c.miss)
	}
	t.Logf("%s silent misses: %d / %d (%.0f%%)", name, c.missSilent, c.miss, pct)
}

// assumedCalls lists the functions that made a closure whose captured values were applied there
// (assumed_call, C-7), by the end of their ID (the function that made it).
func (c curatedCounts) assumedCalls() map[string]bool {
	out := map[string]bool{}
	for _, u := range c.uncertainties {
		if u.Kind == taint.KindAssumedCall {
			out[u.Func] = true
		}
	}
	return out
}

// checkAssumedCall holds what C-7 promises: a closure that is given away has the assumed_call at the
// place it was made; a closure that is only called there (the call, go, defer) has none.
func checkAssumedCall(t *testing.T, c curatedCounts, pkg string, given, seen []string) {
	t.Helper()
	have := c.assumedCalls()
	for _, f := range given {
		if !have[pkg+f] {
			t.Errorf("%s%s: no assumed_call for the closure that is given away", pkg, f)
		}
	}
	for _, f := range seen {
		if have[pkg+f] {
			t.Errorf("%s%s: assumed_call for a closure that is only called where it is made (it would be applied twice)", pkg, f)
		}
	}
}

// TestCuratedGo is the accuracy benchmark for Go (bench/go-curated): it runs with
// every "go test" and takes seconds.
func TestCuratedGo(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	c := checkCurated(t, dir)
	t.Logf("go-curated: %d flows to find, %d sinks to keep quiet, %d flows not found yet (@miss: %d silent, %d with a trace), %d reported wrongly (@fp), %d flows of other sources into noisy sinks (@noisy)", c.detect, c.quiet, c.miss, c.missSilent, c.missTrace, c.fp, c.noisy)
	logSilentMisses(t, "go-curated", c)
	checkAssumedCall(t, c, "curated/api.",
		[]string{"capRunA", "capRunB", "capWrite", "capNested", "capLoop", "capTwo", "mkRun", "capLate", "capSeenAndPassed", "capResult"},
		[]string{"capInline", "capGo", "capDefer"})
	checkOffSource(t, c, maxGoOffSource)
	if c.detect < 15 || c.quiet < 8 {
		t.Errorf("the project has shrunk: %+v", c)
	}
}

// dartCuratedStream runs the Dart extractor on bench/dart-curated: the IR and the directory.
func dartCuratedStream(t *testing.T) ([]byte, string) {
	t.Helper()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "dart-curated"))
	if err != nil {
		t.Fatal(err)
	}
	extractor, err := filepath.Abs(filepath.Join("..", "..", "extractor", "dart"))
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	// the package has no dependency outside the repository: pub resolves it without the network
	if _, err := os.Stat(filepath.Join(dir, ".dart_tool", "package_config.json")); err != nil {
		get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get", "--offline")...)
		get.Dir = dir
		if out, err := get.CombinedOutput(); err != nil {
			t.Fatalf("pub get: %v\n%s", err, out)
		}
	}
	cmd := exec.Command(words[0], append(append([]string{}, words[1:]...), "run", "bin/rivus_dart.dart", dir)...)
	cmd.Dir = extractor
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v\n%s", err, stderr.String())
	}
	return stream, dir
}

// TestCuratedDart is the accuracy benchmark for Dart (bench/dart-curated). It needs
// the Dart SDK: RIVUS_DART names the command that runs it, for example "dart" or
// "mise exec dart@3.13.6 -- dart".
func TestCuratedDart(t *testing.T) {
	stream, dir := dartCuratedStream(t)
	c := checkCuratedIR(t, dir, stream, ".dart")
	// limits.dart: nine closures to one helper. The functions are known and rivus leaves them out
	// because of its own limit: it says so, and does not say the callee is unknown (ADR-050).
	if !c.hasUncertainty("depth_limit", "limits.dart", "candidate_limit") {
		t.Error("limits.dart: no depth_limit (candidate_limit) for the helper that nine closures are given to")
	}
	if !c.hasUncertainty("depth_limit", "limits.dart", "caller_depth_limit") {
		t.Error("limits.dart: no depth_limit (caller_depth_limit) for the helper that is five calls below the closure")
	}
	t.Logf("dart-curated: %d flows to find, %d sinks to keep quiet, %d flows not found yet (@miss: %d silent, %d with a trace), %d reported wrongly (@fp), %d flows of other sources into noisy sinks (@noisy)", c.detect, c.quiet, c.miss, c.missSilent, c.missTrace, c.fp, c.noisy)
	logSilentMisses(t, "dart-curated", c)
	checkAssumedCall(t, c, "package:curated/callbacks.dart#",
		[]string{"button_a", "button_b", "timer_run", "seen_and_passed", "cap_result"},
		[]string{"inline_only", "inline_literal"})
	checkOffSource(t, c, maxDartOffSource)
	if c.detect < 70 || c.quiet < 35 {
		t.Errorf("the project has shrunk: %+v", c)
	}
}

// rustCuratedStream runs the Rust extractor on bench/rust-curated: the IR and the directory.
func rustCuratedStream(t *testing.T) ([]byte, string) {
	t.Helper()
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "rust-curated"))
	if err != nil {
		t.Fatal(err)
	}
	stream, err := exec.Command(bin, dir).Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v", err)
	}
	return stream, dir
}

// TestCuratedRust is the accuracy benchmark for Rust (bench/rust-curated). It needs
// the extractor binary: RIVUS_RUST (extractor/rust: cargo build).
func TestCuratedRust(t *testing.T) {
	stream, dir := rustCuratedStream(t)
	c := checkCuratedIR(t, dir, stream, ".rs")
	t.Logf("rust-curated: %d flows to find, %d sinks to keep quiet, %d flows not found yet (@miss: %d silent, %d with a trace), %d reported wrongly (@fp), %d flows of other sources into noisy sinks (@noisy)", c.detect, c.quiet, c.miss, c.missSilent, c.missTrace, c.fp, c.noisy)
	logSilentMisses(t, "rust-curated", c)
	checkOffSource(t, c, maxRustOffSource)
	if c.detect < 100 || c.quiet < 38 {
		t.Errorf("the project has shrunk: %+v", c)
	}
}

// missTraces returns the kinds of the uncertainties that the engine left in the functions of the
// source and of the sink of a flow that it did not find: what `rivus resolve` says about a flow
// that is gone ("absent_now"). The uncertainties of the whole run (no function) do not count; nor
// do a dynamic_dispatch with exactly one candidate (it is told at every method call, and says
// nothing of the callback that was lost: advisor's rule, bench/MISSES.md) and numeric_stop
// (numbers do not carry an injection: it is no explanation of a miss).
func missTraces(rep *report.Report, from, to curatedAt, programWide bool) []string {
	funcs := map[string]bool{}
	for _, s := range rep.Sites {
		p := curatedAt{s.Position.File, s.Position.Line}
		if p == from || p == to {
			funcs[s.Func] = true
		}
	}
	seen := map[string]bool{}
	var kinds []string
	for _, u := range rep.Uncertainties {
		if u.Func == "" && programWide && u.Kind == taint.KindRecursionApproximation {
			// an uncertainty of the whole program (the limit of the number of passes): it says that
			// a flow may be lost, and not which. Only a @miss that names the kind counts it.
		} else if u.Func == "" || !funcs[u.Func] {
			continue
		}
		if u.Kind == taint.KindNumericStop || (u.Kind == taint.KindDynamicDispatch && len(u.Candidates) == 1) {
			continue
		}
		if !seen[u.Kind] {
			seen[u.Kind] = true
			kinds = append(kinds, u.Kind)
		}
	}
	sort.Strings(kinds)
	return kinds
}

// checkMissTrace compares the trace of a @miss with its annotation, and counts it in c.
func checkMissTrace(t *testing.T, rep report.Report, pairName, tag string, want []string, end [2]curatedAt, c *curatedCounts, programWide bool) []string {
	silent := len(want) == 1 && want[0] == "silent"
	got := missTraces(&rep, end[0], end[1], programWide && !silent)
	if os.Getenv("RIVUS_CURATED_TRACES") == "1" {
		t.Logf("MISSTRACE %s:%d %s %v", end[0].file, end[0].line, tag, got)
		funcs := map[string]bool{}
		for _, st := range rep.Sites {
			if p := (curatedAt{st.Position.File, st.Position.Line}); p == end[0] || p == end[1] {
				funcs[st.Func] = true
			}
		}
		for _, u := range rep.Uncertainties {
			if funcs[u.Func] {
				t.Logf("MISSRAW %s %s %s:%d cand=%d %s", tag, u.Kind, u.Position.File, u.Position.Line, len(u.Candidates), u.Detail)
			}
		}
	}
	if len(want) == 0 {
		return nil // the annotation has no kind: told where it is read
	}
	var problems []string
	switch {
	case silent:
		c.missSilent++
		if len(got) > 0 {
			problems = append(problems, fmt.Sprintf("a trace has come (improvement: write @miss(%s) instead of @miss(silent)): %s [%s]", strings.Join(got, ","), pairName, tag))
		}
	default:
		c.missTrace++
		have := map[string]bool{}
		for _, k := range got {
			have[k] = true
		}
		for _, k := range want {
			if !have[k] {
				problems = append(problems, fmt.Sprintf("the trace %s of the @miss is gone (the uncertainties of its functions: %v): %s [%s]", k, got, pairName, tag))
			}
		}
	}
	return problems
}

// The findings of the curated projects that have no step in the function of their source
// (offSourcePath, BENCH-2), today. These are ceilings: they must go down, never up, and
// they are lowered when CORE-B is fixed (to 0). What they are is in the log (OFFSOURCE, with
// RIVUS_CURATED_TRACES=1) and in bench/MISSES.md.
const (
	maxGoOffSource   = 0
	maxDartOffSource = 0
	maxRustOffSource = 0
)

// checkOffSource holds the findings without a step in their source function to the ceiling.
func checkOffSource(t *testing.T, c curatedCounts, max int) {
	t.Helper()
	t.Logf("%d findings have no step in the function of their source (ceiling %d): %v", c.offSource, max, c.offKinds)
	if c.offSource > max {
		t.Errorf("%d findings have a path without a step in their source function: more than the %d of today: %v", c.offSource, max, c.offKinds)
	}
	if c.offSource < max {
		t.Errorf("only %d findings have a path without a step in their source function, and the ceiling is %d: lower it", c.offSource, max)
	}
}
