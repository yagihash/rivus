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
	"sync"
	"testing"
)

// The zoo (bench/zoo-go, docs/DESIGN-zoo.md): small cases of one shape each, three to a cell (a vulnerable
// one, a fixed one, a decoy), whose annotations are held to what the cases do when they are run:
//
//   - TestZooExecGo builds the cases with -tags zooexec and -cover, runs them three times (variant 0,
//     variant 1, variant 0 in the reverse order) and checks them: the reverse run sees the same, the
//     variants agree on what reached, every statement of the cells ran;
//   - TestZooTruthGo holds the annotations (@sink, @nosink, @carry, @manual, ...) to what was seen;
//   - TestZooGo runs rivus on the cells and holds its findings to the same annotations, with the
//     ratchet of the curated projects (@miss, @fp), and prints the headline numbers.
//
// The annotation of a case is never changed to what rivus says: a flow that rivus does not find is a
// @miss with a @ref, a report that must not be there is an @fp with a @ref.

type zooHit struct {
	Case      string   `json:"case"`
	File      string   `json:"file"`
	Line      int      `json:"line"`
	Class     string   `json:"class"`
	Seen      []string `json:"seen"`
	Reached   []string `json:"reached"`
	Srcs      []string `json:"srcs"`
	Recv      string   `json:"recv"`
	Masked    string   `json:"masked"`
	Ambiguous bool     `json:"ambiguous"`
}

type zooRun struct {
	Variant int      `json:"variant"`
	Reverse bool     `json:"reverse"`
	Cases   []string `json:"cases"`
	Hits    []zooHit `json:"hits"`
}

// zooObsHit is a hit of observed.json (the shape that every language writes).
type zooObsHit struct {
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Class   string   `json:"class"`
	Seen    []string `json:"seen"`
	Reached []string `json:"reached"`
	Carry   []string `json:"carry"`
}

type zooObserved struct {
	Cases     []string
	Hits      map[string][]zooObsHit
	Uncovered []string // the statements of the cells that did not run
	Problems  []string // the runs that do not agree
}

var (
	zooOnce sync.Once
	zooObs  zooObserved
	zooErr  error
)

func zooRoot(t testing.TB) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "zoo-go"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func zooCommand(dir string, env []string, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

// zooObserve builds and runs the cases once per test binary.
func zooObserve(t testing.TB) zooObserved {
	t.Helper()
	dir := zooRoot(t)
	zooOnce.Do(func() { zooObs, zooErr = zooObserveOnce(dir, filepath.Join(filepath.Dir(binary), "zoo"), "zoo/cells/...", "") })
	if zooErr != nil {
		t.Fatal(zooErr)
	}
	return zooObs
}

// zooObserveOnce builds ./cmd/observe of the module in dir (with the overlay file, when there is one: a package of the zoo has
// no cmd/observe of its own, see zoo_pkg_test.go) and runs it three times. work is where the runs write; coverpkg the cells.
func zooObserveOnce(dir, work, coverpkg, overlay string) (zooObserved, error) {
	var obs zooObserved
	if err := os.MkdirAll(work, 0o755); err != nil {
		return obs, err
	}
	exe := filepath.Join(work, "observe")
	build := []string{"build", "-tags", "zooexec", "-cover", "-coverpkg=" + coverpkg, "-o", exe}
	if overlay != "" {
		build = append(build, "-overlay", overlay)
	}
	if out, err := zooCommand(dir, nil, "go", append(build, "./cmd/observe")...).CombinedOutput(); err != nil {
		return obs, fmt.Errorf("building the harness: %v\n%s", err, out)
	}
	type runSpec struct {
		name    string
		variant string
		reverse bool
	}
	runs := []runSpec{{"v0", "0", false}, {"v1", "1", false}, {"rev", "0", true}}
	got := map[string]zooRun{}
	var covDirs []string
	for _, r := range runs {
		cov := filepath.Join(work, "cov-"+r.name)
		os.RemoveAll(cov)
		if err := os.MkdirAll(cov, 0o755); err != nil {
			return obs, err
		}
		covDirs = append(covDirs, cov)
		outFile := filepath.Join(work, r.name+".json")
		args := []string{"-out", outFile}
		if r.reverse {
			args = append(args, "-reverse")
		}
		if out, err := zooCommand(dir, []string{"ZOO_VARIANT=" + r.variant, "GOCOVERDIR=" + cov}, exe, args...).CombinedOutput(); err != nil {
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
	}

	// the statements of the cells that did not run, in any of the three runs
	prof := filepath.Join(work, "cover.txt")
	if out, err := zooCommand(dir, nil, "go", "tool", "covdata", "textfmt", "-i="+strings.Join(covDirs, ","), "-o="+prof).CombinedOutput(); err != nil {
		return obs, fmt.Errorf("covdata: %v\n%s", err, out)
	}
	f, err := os.Open(prof)
	if err != nil {
		return obs, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "/cells/") || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[2] == "0" {
			name := fields[0]
			if i := strings.Index(name, "/cells/"); i >= 0 {
				name = name[i+1:]
			}
			obs.Uncovered = append(obs.Uncovered, name)
		}
	}
	sort.Strings(obs.Uncovered)
	return zooCombine(obs, got)
}

// zooCombine reads the three runs (variant 0, variant 1, variant 0 in the reverse order; the harness of every
// language writes the same shape) and makes the observation: what reached each sink in variant 0, the carries
// (from the difference with variant 1), and the problems (the runs that do not agree). It is shared by the
// languages.
func zooCombine(obs zooObserved, got map[string]zooRun) (zooObserved, error) {
	v0, v1, rev := got["v0"], got["v1"], got["rev"]
	key := func(h zooHit, n int) string { return fmt.Sprintf("%s|%s:%d|%d", h.Case, h.File, h.Line, n) }
	index := func(r zooRun) (map[string]zooHit, []string) {
		m := map[string]zooHit{}
		var keys []string
		count := map[string]int{}
		for _, h := range r.Hits {
			base := fmt.Sprintf("%s|%s:%d", h.Case, h.File, h.Line)
			k := key(h, count[base])
			count[base]++
			m[k] = h
			keys = append(keys, k)
		}
		return m, keys
	}
	m0, keys0 := index(v0)
	m1, _ := index(v1)
	mr, _ := index(rev)
	if len(m0) != len(mr) || len(m0) != len(m1) {
		obs.Problems = append(obs.Problems, fmt.Sprintf("the runs have a different number of hits: variant 0 %d, variant 1 %d, reverse %d", len(m0), len(m1), len(mr)))
	}
	obs.Cases = v0.Cases
	obs.Hits = map[string][]zooObsHit{}
	for _, c := range v0.Cases {
		obs.Hits[c] = nil
	}
	for _, k := range keys0 {
		h := m0[k]
		if h.Ambiguous {
			obs.Problems = append(obs.Problems, fmt.Sprintf("%s: the text reached the sink at %s:%d and the case has several sources (name the tag in the text, or use one source)", h.Case, h.File, h.Line))
		}
		o := zooObsHit{File: h.File, Line: h.Line, Class: h.Class, Seen: h.Seen, Reached: h.Reached}
		if r, ok := mr[k]; !ok || r.Recv != h.Recv || strings.Join(r.Reached, ",") != strings.Join(h.Reached, ",") {
			obs.Problems = append(obs.Problems, fmt.Sprintf("%s: the reverse run saw something else at %s:%d (the cases depend on their order)", h.Case, h.File, h.Line))
		}
		if o1, ok := m1[k]; !ok {
			obs.Problems = append(obs.Problems, fmt.Sprintf("%s: variant 1 has no hit at %s:%d", h.Case, h.File, h.Line))
		} else {
			if strings.Join(o1.Reached, ",") != strings.Join(h.Reached, ",") {
				obs.Problems = append(obs.Problems, fmt.Sprintf("%s: what reached %s:%d depends on the variant (%v, %v)", h.Case, h.File, h.Line, h.Reached, o1.Reached))
			}
			if len(h.Reached) == 0 && len(o1.Reached) == 0 && h.Class != "Any" && o1.Masked != h.Masked {
				// the text depends on the danger of the marker, and the danger did not reach: a carry
				o.Carry = h.Seen
				if len(o.Carry) == 0 {
					o.Carry = h.Srcs
				}
			}
		}
		obs.Hits[h.Case] = append(obs.Hits[h.Case], o)
	}
	if p := os.Getenv("ZOO_OBSERVED_OUT"); p != "" {
		b, _ := json.MarshalIndent(obs.Hits, "", " ")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return obs, err
		}
	}
	return obs, nil
}

// TestZooExecGo runs the self-check of mark (the constants and the probes of bench/zoo/spec.json,
// in both variants) and the three runs of the cases.
func TestZooExecGo(t *testing.T) {
	dir := zooRoot(t)
	for _, v := range []string{"0", "1"} {
		out, err := zooCommand(dir, []string{"ZOO_VARIANT=" + v}, "go", "test", "-count=1", "-tags", "zooexec", "./mark").CombinedOutput()
		if err != nil {
			t.Errorf("the spec of mark, variant %s: %v\n%s", v, err, out)
		}
	}
	obs := zooObserve(t)
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
	t.Logf("zoo-go: %d cases run, %d hits", len(obs.Cases), n)
}

// ---- the annotations of the cells

var zooTagRole = regexp.MustCompile(`-(vuln|fixed|decoy)[0-9]*(-[a-z0-9]+)?$`)

var (
	zooSinkRe   = regexp.MustCompile(`@sink\(([A-Za-z0-9_-]+)\)`)
	zooNosinkRe = regexp.MustCompile(`@nosink\(([A-Za-z0-9_-]+)\)`)
	zooCarryRe  = regexp.MustCompile(`@carry\(([a-z0-9-]+),\s*(encode|digest|numeric)\)`)
	zooManualRe = regexp.MustCompile(`@manual\(([^)]+)\)`)
	zooRefRe    = regexp.MustCompile(`@ref\(([^)]+)\)`)
	zooCellRe   = regexp.MustCompile(`@cell\((F/[a-z0-9/-]+)\)`)
	zooShapeRe  = regexp.MustCompile(`@shape:\s*(\S.*)`)
	zooRoleRe   = regexp.MustCompile(`@(vuln|fixed|decoy)\b`)
	zooMissRe   = regexp.MustCompile(`@miss(?:\(([a-z_,]+)\))?`)
	zooFpRe     = regexp.MustCompile(`@fp\b`)
	zooNaRe     = regexp.MustCompile(`@na\(([^)]+)\)`)
	// the marks of a Go cell that the other language does not have (docs/DESIGN-zoo.md, "@na and @todo"):
	// @na(F/<family>/<shape>: why), the form does not exist in the language; @todo(F/<family>/<shape>: PARITY: why), the
	// form can be written and the cell is not written yet. Both pass the parity check; the table counts them apart.
	zooParityNaRe   = regexp.MustCompile(`@na\((F/[a-z0-9/-]+):\s*([^)]+)\)`)
	zooParityTodoRe = regexp.MustCompile(`@todo\((F/[a-z0-9/-]+):\s*PARITY:\s*([^)]+)\)`)
)

// zooParityMarks reads the @na and @todo marks of a file (a cell name -> the reason).
func zooParityMarks(src string) (na, todo map[string]string) {
	na, todo = map[string]string{}, map[string]string{}
	for _, m := range zooParityNaRe.FindAllStringSubmatch(src, -1) {
		na[m[1]] = m[2]
	}
	for _, m := range zooParityTodoRe.FindAllStringSubmatch(src, -1) {
		todo[m[1]] = m[2]
	}
	return
}

// zooParityLine logs the count of the cells that the language lacks, in two kinds, for bench/parity (the table
// reads this line): "zoo-<lang>: parity marks: @na N [names] | @todo N [names]".
func zooParityLine(t testing.TB, lang string, na, todo []string) {
	t.Helper()
	sort.Strings(na)
	sort.Strings(todo)
	t.Logf("%s: parity marks: @na %d [%s] | @todo %d [%s]", lang, len(na), strings.Join(na, " "), len(todo), strings.Join(todo, " "))
}

// zooLine is what the annotations of one line say.
type zooLine struct {
	file       string
	line       int
	sinks      []string // tags, or "none"
	nosinks    []string
	carries    [][2]string
	manual     string
	hasMiss    bool
	hasFp      bool
	hasRef     bool
	refs       []string // the names in @ref(...)
	sourceTags []string
	missKinds  []string
}

type zooCell struct {
	name   string // F/call/direct
	dir    string
	shape  string
	roles  map[string]int
	manual int
	na     []string // @na(reason): the form does not exist in the other language, and why
}

// zooScan reads the cells of the Go zoo: the annotated lines and the cells (a cell is a directory).
func zooScan(t testing.TB, dir string) ([]zooLine, []*zooCell) {
	t.Helper()
	return zooScanIn(t, dir, "cells", ".go", func(rel string) string { return filepath.ToSlash(filepath.Dir(rel)) })
}

// zooScanIn reads the files of the zoo of a language (sub: the directory of the cells, ext: the extension), the
// annotated lines and the cells. unit says which unit holds a cell (the package of Go, the file of Dart): the
// directory or the file of rel (the path from dir).
func zooScanIn(t testing.TB, dir, sub, ext string, unit func(rel string) string) ([]zooLine, []*zooCell) {
	t.Helper()
	var lines []zooLine
	cells := map[string]*zooCell{}
	byDir := map[string][]*zooCell{}
	var goDirs []string
	err := filepath.Walk(filepath.Join(dir, sub), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ext) || filepath.Base(path) == "lib.rs" { // (lib.rs of the Rust zoo only lists the cells)
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		d := unit(rel)
		seenDir := false
		for _, g := range goDirs {
			seenDir = seenDir || g == d
		}
		if !seenDir {
			goDirs = append(goDirs, d)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			text := sc.Text()
			if m := zooCellRe.FindStringSubmatch(text); m != nil {
				c := cells[m[1]]
				if c == nil {
					c = &zooCell{name: m[1], dir: d, roles: map[string]int{}}
					cells[m[1]] = c
				}
				byDir[d] = append(byDir[d], c)
			}
			if m := zooShapeRe.FindStringSubmatch(text); m != nil {
				for _, c := range byDir[d] {
					c.shape = m[1]
				}
			}
			for _, m := range zooRoleRe.FindAllStringSubmatch(text, -1) {
				for _, c := range byDir[d] {
					c.roles[m[1]]++
				}
			}
			zl := zooLine{file: rel, line: n}
			for _, m := range zooSinkRe.FindAllStringSubmatch(text, -1) {
				zl.sinks = append(zl.sinks, m[1])
			}
			for _, m := range zooNosinkRe.FindAllStringSubmatch(text, -1) {
				zl.nosinks = append(zl.nosinks, m[1])
			}
			for _, m := range zooCarryRe.FindAllStringSubmatch(text, -1) {
				zl.carries = append(zl.carries, [2]string{m[1], m[2]})
			}
			if m := zooManualRe.FindStringSubmatch(text); m != nil {
				zl.manual = m[1]
				for _, c := range byDir[d] {
					c.manual++
				}
			}
			for _, m := range curatedAnnot.FindAllStringSubmatch(text, -1) {
				if m[1] == "source" {
					zl.sourceTags = append(zl.sourceTags, m[2])
				}
			}
			if m := zooMissRe.FindStringSubmatch(text); m != nil {
				zl.hasMiss = true
				if m[1] != "" {
					zl.missKinds = strings.Split(m[1], ",")
				}
			}
			zl.hasFp = zooFpRe.MatchString(text)
			zl.hasRef = zooRefRe.MatchString(text)
			for _, m := range zooRefRe.FindAllStringSubmatch(text, -1) {
				zl.refs = append(zl.refs, strings.Split(m[1], ",")...)
			}
			for _, m := range zooNaRe.FindAllStringSubmatch(text, -1) {
				for _, c := range byDir[d] {
					c.na = append(c.na, m[1])
				}
			}
			if len(zl.sinks)+len(zl.nosinks)+len(zl.carries)+len(zl.sourceTags) > 0 || zl.manual != "" || zl.hasMiss || zl.hasFp {
				lines = append(lines, zl)
			}
		}
		return sc.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range goDirs {
		if len(byDir[d]) == 0 {
			t.Errorf("%s has no @cell", d)
		}
	}
	var out []*zooCell
	for _, c := range cells {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return lines, out
}

func zooContains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// TestZooTruthGo holds the annotations of the cells to what the cases did when they were run.
func TestZooTruthGo(t *testing.T) {
	dir := zooRoot(t)
	obs := zooObserve(t)
	lines, cells := zooScan(t, dir)
	zooCheckCells(t, cells, obs)
	// every directory of the cells is imported by the harness
	imports, err := os.ReadFile(filepath.Join(dir, "cmd", "observe", "imports.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cells {
		if !strings.Contains(string(imports), `"zoo/`+c.dir+`"`) {
			t.Errorf("zoo/%s is not imported by cmd/observe/imports.go", c.dir)
		}
	}
	zooHoldLines(t, lines, cells, obs)
}

// zooCheckCells checks the cells (a shape, the three roles) and that the cases that were run are the ones of
// the cells. It is shared by the languages.
func zooCheckCells(t *testing.T, cells []*zooCell, obs zooObserved) {
	t.Helper()
	// the cells: three cases each, and the cases are the ones that were run
	wantCases := map[string]bool{}
	for _, c := range cells {
		if c.shape == "" {
			t.Errorf("%s has no @shape", c.name)
		}
		for _, role := range []string{"vuln", "fixed", "decoy"} {
			if c.roles[role] < 1 {
				t.Errorf("%s: no role @%s", c.name, role)
			}
			wantCases[c.name+"/"+role] = true
		}
	}
	for id := range wantCases {
		if !zooContains(obs.Cases, id) {
			t.Errorf("the case %s was not run (is its cell in the list of the harness?)", id)
		}
	}
	// a cell may have more cases of the vulnerable role (a chain of 3, 13 and 30 stages): vuln<N>
	extra := regexp.MustCompile(`^(F/[a-z0-9/-]+)/vuln[0-9]+$`)
	for _, id := range obs.Cases {
		if m := extra.FindStringSubmatch(id); m != nil && wantCases[m[1]+"/vuln"] {
			continue
		}
		if !wantCases[id] {
			t.Errorf("the case %s was run and no cell has it", id)
		}
	}
}

// zooHoldLines holds the annotated lines of the cells to the hits of the run (the heart of the truth test; the
// languages share it).
func zooHoldLines(t *testing.T, lines []zooLine, cells []*zooCell, obs zooObserved) {
	t.Helper()
	// the hits of each line, with the case that made them
	type hitAt struct {
		role string
		h    zooObsHit
	}
	at := map[string][]hitAt{}
	for id, hs := range obs.Hits {
		role := id[strings.LastIndex(id, "/")+1:]
		for _, h := range hs {
			k := fmt.Sprintf("%s:%d", h.File, h.Line)
			at[k] = append(at[k], hitAt{role, h})
		}
	}
	annotated := map[string]bool{}
	manuals := 0
	for _, l := range lines {
		k := fmt.Sprintf("%s:%d", l.file, l.line)
		if (l.hasMiss || l.hasFp) && !l.hasRef {
			t.Errorf("%s: a @miss or @fp needs a @ref(...) (the item of the review, an issue, a CWE)", k)
		}
		if l.manual != "" {
			manuals++
		}
		if len(l.sinks)+len(l.nosinks)+len(l.carries) > 0 {
			annotated[k] = true
		}
		hs := at[k]
		reachedTags := map[string]bool{}
		carryTags := map[string]bool{}
		for _, a := range hs {
			for _, r := range a.h.Reached {
				reachedTags[r] = true
			}
			for _, r := range a.h.Carry {
				carryTags[r] = true
			}
		}
		if len(hs) == 0 && len(l.sinks)+len(l.nosinks)+len(l.carries) > 0 && l.manual == "" {
			t.Errorf("%s: the line is annotated and no case reached it when run (a @sink(none) that never ran proves nothing)", k)
			continue
		}
		for _, tag := range l.sinks {
			switch {
			case l.manual != "":
			case tag == "none":
				if len(reachedTags) > 0 || len(carryTags) > 0 {
					t.Errorf("%s: @sink(none), and the run has %v reaching it and %v carried to it", k, keysOf(reachedTags), keysOf(carryTags))
				}
			case !reachedTags[tag]:
				t.Errorf("%s: @sink(%s), and the danger of the tag did not reach it when run (reached: %v)", k, tag, keysOf(reachedTags))
			}
		}
		for _, tag := range l.nosinks {
			if reachedTags[tag] || carryTags[tag] {
				t.Errorf("%s: @nosink(%s), and the run has the tag reaching or carried to it", k, tag)
			}
		}
		for _, c := range l.carries {
			if !carryTags[c[0]] && l.manual == "" {
				t.Errorf("%s: @carry(%s, %s), and the run has no carry of the tag (reached: %v)", k, c[0], c[1], keysOf(reachedTags))
			}
		}
	}
	// a flow that reached a line with no annotation for it: the annotation is missing
	annotatedTags := map[string]map[string]bool{}
	for _, l := range lines {
		k := fmt.Sprintf("%s:%d", l.file, l.line)
		if annotatedTags[k] == nil {
			annotatedTags[k] = map[string]bool{}
		}
		for _, s := range l.sinks {
			annotatedTags[k][s] = true
		}
		for _, c := range l.carries {
			annotatedTags[k][c[0]] = true
		}
	}
	for k, hs := range at {
		for _, a := range hs {
			for _, r := range a.h.Reached {
				if !annotatedTags[k][r] {
					t.Errorf("%s: the run has %s reaching it, and the line has no @sink(%s)", k, r, r)
				}
			}
			for _, r := range a.h.Carry {
				if !annotatedTags[k][r] && !annotatedTags[k]["none"] {
					t.Errorf("%s: the run has %s carried to it, and the line has no @carry(%s, ...)", k, r, r)
				}
			}
		}
	}
	// the hand annotations: 3% of the cases, one to a cell
	for _, c := range cells {
		if c.manual > 1 {
			t.Errorf("%s has %d @manual (one to a cell)", c.name, c.manual)
		}
	}
	if float64(manuals) > 0.03*float64(len(obs.Cases)) {
		t.Errorf("%d @manual of %d cases: more than 3%%", manuals, len(obs.Cases))
	}
	for _, p := range obs.Problems {
		t.Error(p)
	}
	for _, u := range obs.Uncovered {
		t.Errorf("a statement of the cells did not run: %s", u)
	}
}

func keysOf(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestZooGo runs rivus on the cells and holds the findings to the annotations (the ratchet of the
// curated projects), then prints the headline numbers.
func TestZooGo(t *testing.T) {
	dir := zooRoot(t)
	obs := zooObserve(t)
	lines, cells := zooScan(t, dir)
	// the cells that stop by the pass limit on purpose are a program of their own (zooSplit): the others must converge
	rest, recCells := zooSplit(lines, cells)
	c := checkAnnotated(t, zooCopy(t, dir, zooCellDirs(rest)), annotOpts{pattern: "./cells/..."})
	if len(recCells) > 0 {
		zooCheckRecNames(t, recCells)
		c.add(checkAnnotated(t, zooCopy(t, dir, zooCellDirs(recCells)), annotOpts{pattern: "./cells/...", allowUnconverged: true, programWide: true}))
	}
	if t.Failed() || os.Getenv("ZOO_ALONE") == "1" { // ZOO_ALONE=1 asks for it
		// the cells are analysed one by one: green alone and red in the whole is a mix with the others
		t.Log("the zoo failed as a whole: each cell is analysed alone below (green alone: mixed up with the others; red alone: the cell)")
		zooAlone(t, dir, cells)
	}

	zooHeadline(t, "zoo-go", lines, cells, obs, c)
}

// zooHeadline prints the headline numbers (docs/DESIGN-zoo.md, 6.3) of the zoo of a language: from the
// annotations, once they agree with the run, and from what rivus found.
func zooHeadline(t *testing.T, name string, lines []zooLine, cells []*zooCell, obs zooObserved, c curatedCounts) {
	t.Helper()
	// the headline (docs/DESIGN-zoo.md, 6.3): from the annotations, once they agree with the run
	roleOfTag := func(tag string) string {
		if m := zooTagRole.FindStringSubmatch(tag); m != nil {
			return m[1]
		}
		return ""
	}
	vulnTags, missVuln := 0, 0
	missSilent, missTrace, missNorule := 0, 0, 0
	fpFixed, fpDecoy := 0, 0
	for _, l := range lines {
		for _, tag := range l.sourceTags {
			if roleOfTag(tag) != "vuln" {
				continue
			}
			vulnTags++
			if l.hasMiss {
				missVuln++
				switch {
				case len(l.missKinds) == 1 && l.missKinds[0] == "silent":
					missSilent++
				case len(l.missKinds) == 1 && l.missKinds[0] == "norule":
					missNorule++
				default:
					missTrace++
				}
			}
		}
		if l.hasFp {
			k := fmt.Sprintf("%s:%d", l.file, l.line)
			for id, hs := range obs.Hits {
				for _, h := range hs {
					if fmt.Sprintf("%s:%d", h.File, h.Line) == k {
						if strings.HasSuffix(id, "/fixed") {
							fpFixed++
						} else if strings.HasSuffix(id, "/decoy") {
							fpDecoy++
						}
					}
				}
			}
		}
	}
	manuals := 0
	for _, cell := range cells {
		manuals += cell.manual
	}
	carry := map[string][2]int{}
	for _, l := range lines {
		for _, cr := range l.carries {
			v := carry[cr[1]]
			v[0]++
			carry[cr[1]] = v
		}
	}
	nCases := len(obs.Cases)
	t.Logf("%s: cells %d / cases %d | found %d / %d | @miss %d (trace %d, silent %d, no rule %d) | fixed reported wrongly %d | decoys reported wrongly %d / %d (with the trace container_element_imprecision %d, without %d) | carry %v | manual %d (%.1f%%) | run and annotation disagree %d | statements not run %d",
		name, len(cells), nCases, vulnTags-missVuln, vulnTags, missVuln, missTrace, missSilent, missNorule, fpFixed, fpDecoy, len(cells), c.fpTrace, c.fpSilent, carry,
		manuals, 100*float64(manuals)/float64(max(nCases, 1)), len(obs.Problems), len(obs.Uncovered))
	t.Logf("%s: rivus: %d flows found, %d sinks quiet, %d flows not found (@miss), %d reported wrongly (@fp)", name, c.detect, c.quiet, c.miss, c.fp)
	if c.detect+c.miss < 1 {
		t.Errorf("no flow in the zoo: %+v", c)
	}
}
