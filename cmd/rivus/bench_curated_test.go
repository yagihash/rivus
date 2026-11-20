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
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
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
//	x := <-ch          // @source(tag) @miss   (a flow that is there, and is not found)
//	db.Exec(y)         // @sink(none) @fp      (a sink that must stay quiet, and is reported)
//
// "@nosink(tag)" says that the source of the tag must not reach this line (the
// line has a flow of its own, from another source: two keys of one map, say).
// It takes "@fp" in the same way.
var curatedAnnot = regexp.MustCompile(`@(source|sink|nosink)\(([A-Za-z0-9_-]+)\)(\s+@(miss|fp))?`)

type curatedAt struct {
	file string
	line int
}

// curatedCounts is what a curated project holds.
type curatedCounts struct{ detect, quiet, miss, fp int }

// checkCurated analyses the project in dir (a Go module with rules.yaml) and
// compares the findings with the annotations of its .go files.
func checkCurated(t *testing.T, dir string) curatedCounts {
	t.Helper()
	cmd := exec.Command(binary, "analyze", "-rules", "rules.yaml", "./...")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("analyze: %v\n%s", err, stderr)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.LoadErrors) != 0 {
		t.Fatalf("load errors: %v", rep.LoadErrors)
	}

	src, sinks := map[string]curatedAt{}, map[string][]curatedAt{}
	miss := map[string]bool{}
	var none []curatedAt
	fp := map[curatedAt]bool{}
	type notFlow struct {
		tag string
		at  curatedAt
		fp  bool
	}
	var nots []notFlow
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
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
			for _, m := range curatedAnnot.FindAllStringSubmatch(sc.Text(), -1) {
				switch {
				case m[1] == "nosink":
					nots = append(nots, notFlow{m[2], here, m[4] == "fp"})
				case m[1] == "source":
					if _, dup := src[m[2]]; dup {
						t.Errorf("the tag %s has two sources", m[2])
					}
					src[m[2]] = here
					if m[4] == "miss" {
						miss[m[2]] = true
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
	for tag, s := range src {
		if len(sinks[tag]) == 0 {
			t.Errorf("the tag %s has a source and no sink", tag)
		}
		for _, k := range sinks[tag] {
			want[pair(s, k)] = tag
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
		for _, n := range none {
			if k == n {
				hitNone[n] = true
			}
		}
	}

	var c curatedCounts
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
		case !got[k]:
			problems = append(problems, fmt.Sprintf("missing flow: %s [%s]", k, tag))
		default:
			c.detect++
		}
	}
	for _, fd := range rep.Findings {
		// a flow into a quiet sink is told below, with the sink
		k := fmt.Sprintf("%s:%d -> %s:%d", fd.Source.Position.File, fd.Source.Position.Line, fd.Sink.Position.File, fd.Sink.Position.Line)
		if hitNone[curatedAt{fd.Sink.Position.File, fd.Sink.Position.Line}] {
			continue
		}
		if _, ok := want[k]; !ok && !skip[k] {
			problems = append(problems, "unexpected flow: "+k)
		}
	}
	for _, n := range none {
		switch {
		case fp[n] && !hitNone[n]:
			problems = append(problems, fmt.Sprintf("quiet now (remove @fp): %s:%d", n.file, n.line))
		case fp[n]:
			c.fp++
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
	if !rep.Stats.Converged {
		t.Errorf("the analysis did not converge: %+v", rep.Stats)
	}
	return c
}

// TestCuratedGo is the accuracy benchmark for Go (bench/go-curated): it runs with
// every "go test" and takes seconds.
func TestCuratedGo(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "go-curated"))
	if err != nil {
		t.Fatal(err)
	}
	c := checkCurated(t, dir)
	t.Logf("go-curated: %d flows to find, %d sinks to keep quiet, %d flows not found yet (@miss), %d reported wrongly (@fp)", c.detect, c.quiet, c.miss, c.fp)
	if c.detect < 15 || c.quiet < 8 {
		t.Errorf("the project has shrunk: %+v", c)
	}
}
