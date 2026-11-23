package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// A module that calls a dependency (example.com/third, by replace; example.com/third/sub and /other are packages of it)
// that has a function with a body that matters (Clean returns a constant), a function that calls a function it is given
// (Each), one that calls a method of a value it is given (Serve), and one that has a sink (Exec).
//
// The project is made once for the whole run (brShared) and handed to every test that only reads it, so that the runs that several tests
// share (the same arguments over the same project) can share their result too (brAnalyze); bodyReplacedProjectCopy is a project of the
// test's own, for a test that writes into it.
func bodyReplacedProject(t *testing.T) string {
	t.Helper()
	brShared.projectOnce.Do(func() {
		brShared.project = filepath.Join(filepath.Dir(binary), "br-project")
		writeBodyReplacedProject(t, brShared.project)
	})
	return filepath.Join(brShared.project, "app")
}

func bodyReplacedProjectCopy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeBodyReplacedProject(t, root)
	return filepath.Join(root, "app")
}

// brShared holds what the tests of this file share in one run of the tests: the project, the rule files by their content (the same text is
// the same path, so declared_in is the same too), and the result of an analysis by the project and the arguments.
var brShared struct {
	projectOnce sync.Once
	project     string
	mu          sync.Mutex
	rules       map[string]string
	runs        map[string]*brRun
}

type brRun struct {
	once sync.Once
	r    *brReport
}

func writeBodyReplacedProject(t *testing.T, root string) {
	t.Helper()
	app, third := filepath.Join(root, "app"), filepath.Join(root, "third")
	write(t, filepath.Join(third, "go.mod"), "module example.com/third\n\ngo 1.21\n")
	write(t, filepath.Join(third, "third.go"), `package third

import "os/exec"

func Run(s string) string { return s + "!" }

func Clean(s string) string { return "ok" }

func Each(s string, f func(string) string) string { return f(s) }

type Handler interface{ Handle(s string) string }

func Serve(h Handler, s string) string { return h.Handle(s) }

func Exec(s string) { exec.Command(s).Run() }
`)
	write(t, filepath.Join(third, "sub", "sub.go"), "package sub\n\nfunc Id(s string) string { return s }\n")
	write(t, filepath.Join(third, "other", "other.go"), "package other\n\nfunc Id(s string) string { return s }\n")
	write(t, filepath.Join(app, "go.mod"), "module example.com/app\n\ngo 1.21\n\nrequire example.com/third v0.0.0\n\nreplace example.com/third => ../third\n")
	write(t, filepath.Join(app, "main.go"), `package main

import (
	"net/http"
	"os/exec"

	"example.com/third"
	"example.com/third/other"
	"example.com/third/sub"
)

type H struct{}

func (H) Handle(s string) string { exec.Command(s).Run(); return s }

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		exec.Command(third.Run(q)).Run()
		exec.Command(third.Clean(q)).Run()
		exec.Command(third.Each(q, func(x string) string { exec.Command(x).Run(); return x })).Run()
		exec.Command(third.Serve(H{}, q)).Run()
		exec.Command(sub.Id(q)).Run()
		exec.Command(other.Id(q)).Run()
		third.Exec(q)
	})
}
`)
}

type brReport struct {
	Findings []struct {
		ID            string `json:"id"`
		Uncertainties []struct {
			Kind   string `json:"kind"`
			Detail string `json:"detail"`
			Symbol string `json:"symbol"`
		} `json:"uncertainties"`
	} `json:"findings"`
	Summary struct {
		Replaced []struct {
			Package    string `json:"package"`
			DeclaredIn string `json:"declared_in"`
			Reason     string `json:"reason"`
			Functions  int    `json:"functions"`
			Declared   int    `json:"functions_declared"`
		} `json:"body_replaced_packages"`
		Ignored int `json:"body_replaced_ignored"`
	} `json:"summary"`
	Stats struct {
		Functions int `json:"functions"`
	} `json:"stats"`
	Packages []string `json:"packages"`
	raw      []byte
}

func (r *brReport) ids() []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.ID)
	}
	sort.Strings(out)
	return out
}

// kinds are the notes that the findings carry, by kind, with the symbols.
func (r *brReport) notes(kind string) []string {
	seen := map[string]bool{}
	for _, f := range r.Findings {
		for _, u := range f.Uncertainties {
			if u.Kind == kind {
				seen[u.Symbol+": "+u.Detail] = true
			}
		}
	}
	var out []string
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// brAnalyze runs `rivus analyze` over dir and decodes the report. The same dir and arguments give the result of the first run to every
// caller (the reports are only read); a test whose point is a repeated run, or the run of a project that it writes into, uses brAnalyzeFresh.
func brAnalyze(t *testing.T, dir string, args ...string) *brReport {
	t.Helper()
	key := dir + "\x00" + strings.Join(args, "\x00")
	brShared.mu.Lock()
	if brShared.runs == nil {
		brShared.runs = map[string]*brRun{}
	}
	run := brShared.runs[key]
	if run == nil {
		run = &brRun{}
		brShared.runs[key] = run
	}
	brShared.mu.Unlock()
	run.once.Do(func() { run.r = brAnalyzeFresh(t, dir, args...) })
	if run.r == nil { // the first caller failed the test (t.Fatalf in its goroutine)
		t.Fatalf("the shared run of %v failed in another test", args)
	}
	return run.r
}

func brAnalyzeFresh(t *testing.T, dir string, args ...string) *brReport {
	t.Helper()
	out := runIn(t, binary, dir, nil, append([]string{"analyze"}, append(args, "./...")...)...)
	var r brReport
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	r.raw = out
	return &r
}

// brRules writes a rule file; the same text is the same file (named by the hash of the text), so the runs that name it are the same run.
func brRules(t *testing.T, dir, name, body string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(body))
	key := hex.EncodeToString(sum[:8])
	brShared.mu.Lock()
	defer brShared.mu.Unlock()
	if brShared.rules == nil {
		brShared.rules = map[string]string{}
	}
	if p, ok := brShared.rules[key+name]; ok {
		return p
	}
	p := filepath.Join(filepath.Dir(binary), "br-rules", key, name)
	write(t, p, body)
	brShared.rules[key+name] = p
	return p
}

const brDeclare = `
body_replaced_packages:
  - package: example.com/third
    reason: checked against the body with -read-declared-bodies
`

// The declared package is not read. A call of a function that the model has a row for is the model; a function without
// a row is unmodeled_library_call (a finding that the body, which returns a constant, would not have). With the rows that say
// what the body does, the findings are those of the run that reads the body.
func TestBodyReplacedCallsAreTheRowOrUnmodeled(t *testing.T) {
	t.Parallel()
	app := bodyReplacedProject(t)
	read := brAnalyze(t, app)
	if len(read.Summary.Replaced) != 0 || read.Summary.Ignored != 0 {
		t.Fatalf("without a declaration: %+v", read.Summary)
	}

	// no row for Clean (its body returns a constant): what goes in is taken to come out, and the call is said to be unmodeled
	rules := brRules(t, app, "decl.yaml", `ruleset: t@1`+brDeclare+`
propagators:
  - symbol: example.com/third.Run
    flows: [{from: arg0, to: result}]
`)
	got := brAnalyze(t, app, "-rules", rules)
	if len(got.Summary.Replaced) != 1 {
		t.Fatalf("summary.body_replaced_packages: %+v", got.Summary.Replaced)
	}
	if p := got.Summary.Replaced[0]; p.Package != "example.com/third" || p.Declared != 5 || p.Functions != 0 || p.DeclaredIn != rules || !strings.Contains(p.Reason, "checked") {
		t.Errorf("summary.body_replaced_packages: %+v", p)
	}
	if got.Stats.Functions >= read.Stats.Functions {
		t.Errorf("the functions of the package are left out: %d, %d when read", got.Stats.Functions, read.Stats.Functions)
	}
	if len(got.ids()) != len(read.ids())+1 {
		t.Errorf("findings: %d, %d when read: the one more is the call of Clean", len(got.ids()), len(read.ids()))
	}
	for _, id := range read.ids() {
		found := false
		for _, g := range got.ids() {
			found = found || g == id
		}
		if !found {
			t.Errorf("the finding %s of the run that reads the body is lost", id)
		}
	}
	notes := got.notes("unmodeled_library_call")
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "example.com/third.Clean: the body of the package example.com/third is not read") {
		t.Errorf("Clean has no row: unmodeled_library_call says so:\n%s", joined)
	}
	if strings.Contains(joined, "third.Run:") {
		t.Errorf("Run has a row: it is the model, not unmodeled:\n%s", joined)
	}

	// with the row that says what the body does, the findings are the ones of the run that reads the body
	rules = brRules(t, app, "decl.yaml", `ruleset: t@1`+brDeclare+`
propagators:
  - symbol: example.com/third.Run
    flows: [{from: arg0, to: result}]
  - symbol: example.com/third.Clean
    pure: true
`)
	same := brAnalyze(t, app, "-rules", rules)
	if !reflect.DeepEqual(same.ids(), read.ids()) {
		t.Errorf("findings with the rows of what the body does:\n got %v\nwant %v", same.ids(), read.ids())
	}
}

// What the declared package may call back is not lost in silence: the function value that is given to it is taken to be
// called (with the other operands; its result is the result of the call), and so are the methods of a value of a type of
// the project given as an interface, each with an assumed_call. A row of calls in the model says it, and no note is made.
func TestBodyReplacedCallbacksAreTakenToBeCalled(t *testing.T) {
	t.Parallel()
	app := bodyReplacedProject(t)
	read := brAnalyze(t, app)
	rules := brRules(t, app, "decl.yaml", `ruleset: t@1`+brDeclare)
	got := brAnalyze(t, app, "-rules", rules)
	// the sink in the closure and in H.Handle are reached through what the package is given
	for _, id := range read.ids() {
		found := false
		for _, g := range got.ids() {
			found = found || g == id
		}
		if !found {
			t.Errorf("the finding %s of the run that reads the body is lost: a callback was dropped", id)
		}
	}
	assumed := strings.Join(got.notes("assumed_call"), "\n")
	for _, want := range []string{"example.com/third.Each: assumed_call: the function given to example.com/third.Each is taken to be called",
		"example.com/third.Serve: assumed_call: a value of example.com/app.H"} {
		if !strings.Contains(assumed, want) {
			t.Errorf("no assumed_call %q:\n%s", want, assumed)
		}
	}

	// the model says what the package calls: nothing is assumed
	rules = brRules(t, app, "decl.yaml", `ruleset: t@1`+brDeclare+`
propagators:
  - symbol: example.com/third.Each
    flows: [{from: arg0, to: result}]
    calls:
      - fn: arg1
        with: [arg0]
        ret: result
`)
	said := brAnalyze(t, app, "-rules", rules)
	if a := strings.Join(said.notes("assumed_call"), "\n"); strings.Contains(a, "third.Each") {
		t.Errorf("the model has a row of calls for Each, and it is assumed too:\n%s", a)
	}
	// the sink of the closure is still reached, by the row
	if n := len(said.ids()); n < len(read.ids())-1 {
		t.Errorf("findings with the row of calls: %d, %d when read", n, len(read.ids()))
	}
}

// A declaration is for the package it names and no other: example.com/third does not take example.com/third/sub. The
// ending /... names the package and what is under it.
func TestBodyReplacedNamesThePackageAndNoOtherUnlessSaid(t *testing.T) {
	t.Parallel()
	app := bodyReplacedProject(t)
	one := brAnalyze(t, app, "-body-replaced", "example.com/third")
	if len(one.Summary.Replaced) != 1 || one.Summary.Replaced[0].Declared != 5 || one.Summary.Replaced[0].DeclaredIn != "-body-replaced" {
		t.Fatalf("-body-replaced example.com/third: %+v", one.Summary.Replaced)
	}
	tree := brAnalyze(t, app, "-body-replaced", "example.com/third/...")
	if len(tree.Summary.Replaced) != 1 || tree.Summary.Replaced[0].Declared != 7 {
		t.Fatalf("-body-replaced example.com/third/...: %+v", tree.Summary.Replaced)
	}
	sub := brAnalyze(t, app, "-body-replaced", "example.com/third/sub", "-body-replaced", "example.com/third/other")
	if len(sub.Summary.Replaced) != 2 || sub.Summary.Replaced[0].Package != "example.com/third/other" || sub.Summary.Replaced[1].Declared != 1 {
		t.Fatalf("the two subpackages: %+v", sub.Summary.Replaced)
	}
	// a package of the project is always read; what names no function is said
	cmd := exec.Command(binary, "analyze", "-body-replaced", "example.com/app", "-body-replaced", "example.com/not/there", "./...")
	cmd.Dir = app
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err, stderr.String())
	}
	var r brReport
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Summary.Replaced) != 0 || !strings.Contains(stderr.String(), "-body-replaced example.com/app: no function of a dependency") {
		t.Errorf("the project's own package: %+v\n%s", r.Summary.Replaced, stderr.String())
	}
}

// -read-declared-bodies reads the body of what is declared: the result is the one of a run without the declaration, and
// the summary says how many declarations were left out.
func TestReadDeclaredBodiesIsTheRunWithoutTheDeclaration(t *testing.T) {
	t.Parallel()
	app := bodyReplacedProject(t)
	read := brAnalyze(t, app)
	rules := brRules(t, app, "decl.yaml", `ruleset: t@1`+brDeclare)
	got := brAnalyze(t, app, "-rules", rules, "-body-replaced", "example.com/third/sub", "-read-declared-bodies")
	if !reflect.DeepEqual(got.ids(), read.ids()) || got.Stats.Functions != read.Stats.Functions {
		t.Errorf("findings %v (%d functions), want %v (%d)", got.ids(), got.Stats.Functions, read.ids(), read.Stats.Functions)
	}
	if len(got.Summary.Replaced) != 0 || got.Summary.Ignored != 2 {
		t.Errorf("summary: %+v", got.Summary)
	}
	if !reflect.DeepEqual(got.notes("unmodeled_library_call"), read.notes("unmodeled_library_call")) {
		t.Errorf("the notes differ from the run that never had the declaration")
	}
}

// The report does not depend on the order the declarations were given in (flags, or the entries of a file).
func TestBodyReplacedIsDeterministic(t *testing.T) {
	t.Parallel()
	app := bodyReplacedProject(t)
	a := brRules(t, app, "a.yaml", "ruleset: a@1\nbody_replaced_packages:\n  - {package: example.com/third/sub, reason: r1}\n  - {package: example.com/third/other, reason: r2}\n")
	b := brRules(t, app, "b.yaml", "ruleset: b@1\nbody_replaced_packages:\n  - {package: example.com/third/other, reason: r2}\n  - {package: example.com/third/sub, reason: r1}\n")
	sumOf := func(args ...string) string {
		r := brAnalyzeFresh(t, app, args...)
		j, _ := json.Marshal(r.Summary.Replaced)
		return string(j) + strings.Join(r.ids(), ",")
	}
	x := sumOf("-body-replaced", "example.com/third/sub", "-body-replaced", "example.com/third/other")
	y := sumOf("-body-replaced", "example.com/third/other", "-body-replaced", "example.com/third/sub")
	if x != y {
		t.Errorf("the order of the flags changes the result:\n%s\n%s", x, y)
	}
	// declared_in differs (the file), the rest is the same
	ja, jb := sumOf("-rules", a), sumOf("-rules", b)
	strip := func(s string) string { return strings.NewReplacer(a, "F", b, "F").Replace(s) }
	if strip(ja) != strip(jb) {
		t.Errorf("the order of the entries changes the result:\n%s\n%s", ja, jb)
	}
	// the same run twice
	if x2 := sumOf("-body-replaced", "example.com/third/sub", "-body-replaced", "example.com/third/other"); x2 != x {
		t.Error("two runs differ")
	}
}

// A rule file of the project (<dir>/.rivus/rules) declares like any other.
func TestBodyReplacedOfAProjectRuleFile(t *testing.T) {
	t.Parallel()
	app := bodyReplacedProjectCopy(t)
	write(t, filepath.Join(app, ".rivus", "rules", "third.yaml"), "ruleset: third@1"+brDeclare)
	got := brAnalyze(t, app)
	if len(got.Summary.Replaced) != 1 || !strings.HasSuffix(got.Summary.Replaced[0].DeclaredIn, filepath.Join(".rivus", "rules", "third.yaml")) {
		t.Errorf("summary: %+v", got.Summary.Replaced)
	}
	if off := brAnalyze(t, app, "-no-project-rules"); len(off.Summary.Replaced) != 0 {
		t.Errorf("-no-project-rules leaves the declaration out too: %+v", off.Summary.Replaced)
	}
}

func TestBodyReplacedIsChecked(t *testing.T) {
	t.Parallel()
	app := bodyReplacedProject(t)
	for name, c := range map[string]struct{ rules, want string }{
		"no reason":   {"ruleset: t@1\nbody_replaced_packages:\n  - package: example.com/third\n", "reason is required"},
		"no package":  {"ruleset: t@1\nbody_replaced_packages:\n  - reason: x\n", "a package is required"},
		"twice":       {"ruleset: t@1\nbody_replaced_packages:\n  - {package: example.com/third, reason: x}\n  - {package: example.com/third, reason: y}\n", "declared twice"},
		"a pattern":   {"ruleset: t@1\nbody_replaced_packages:\n  - {package: example.com/*/third, reason: x}\n", "not the name of a package"},
		"dots inside": {"ruleset: t@1\nbody_replaced_packages:\n  - {package: example.com/.../third, reason: x}\n", "only at the end"},
		"the project": {"ruleset: t@1\nbody_replaced_packages:\n  - {package: \"file:\", reason: x}\n", "code of the project itself"},
		"unknown key": {"ruleset: t@1\nbody_replaced_packages:\n  - {package: example.com/third, reason: x, packages: y}\n", "field packages not found"},
	} {
		t.Run(name, func(t *testing.T) {
			rules := brRules(t, app, "bad.yaml", c.rules)
			cmd := exec.Command(binary, "analyze", "-rules", rules, "./...")
			cmd.Dir = app
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), c.want) {
				t.Errorf("want an error with %q: %v\n%s", c.want, err, out)
			}
		})
	}
	cmd := exec.Command(binary, "analyze", "-body-replaced", "example.com/a b", "./...")
	cmd.Dir = app
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "not the name of a package") {
		t.Errorf("-body-replaced with a space: %v\n%s", err, out)
	}
}

// Dart: the package is named package:<name>. Its functions are left out of the analysis, the findings are those of the run
// that reads them.
func TestBodyReplacedOfADartPackage(t *testing.T) {
	t.Parallel()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	dartTreeIR(t, dart, "monorepo/app", "monorepo/app", "monorepo/shared")
	run := func(args ...string) *brReport {
		cmd := exec.Command(binary, append([]string{"analyze", "-lang", "dart", "-C", filepath.Join(extractor, "test", "monorepo", "app")}, args...)...)
		cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze %v: %v", args, err)
		}
		var r brReport
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		return &r
	}
	read := run()
	got := run("-body-replaced", "package:fixnum", "-body-replaced", "package:no_such")
	if len(got.Summary.Replaced) != 1 || got.Summary.Replaced[0].Package != "package:fixnum" || got.Summary.Replaced[0].Declared == 0 || got.Summary.Replaced[0].Functions != 0 {
		t.Fatalf("summary: %+v", got.Summary.Replaced)
	}
	// the extractor does not read the package (stage 4), or reads it and the core leaves it out (stage 2,
	// -drop-after-read): the result is the same, but for the count of the functions that the report says were left out
	// (stage 2 counts what was read, stage 4 what the package declares)
	filtered := run("-body-replaced", "package:fixnum", "-body-replaced", "package:no_such", "-drop-after-read")
	if len(filtered.Summary.Replaced) != 1 || filtered.Summary.Replaced[0].Functions == 0 || filtered.Summary.Replaced[0].Declared != 0 {
		t.Fatalf("summary of the filter: %+v", filtered.Summary.Replaced)
	}
	if got.Stats.Functions != filtered.Stats.Functions || got.Stats.Functions >= read.Stats.Functions {
		t.Errorf("functions: %d in the extractor, %d in the filter, %d read", got.Stats.Functions, filtered.Stats.Functions, read.Stats.Functions)
	}
	if !reflect.DeepEqual(got.ids(), filtered.ids()) || strings.Join(got.Packages, ",") != strings.Join(filtered.Packages, ",") ||
		got.Summary.Replaced[0].Package != filtered.Summary.Replaced[0].Package || got.Summary.Replaced[0].Reason != filtered.Summary.Replaced[0].Reason {
		t.Errorf("the extractor and the filter differ: %+v, %+v", got, filtered)
	}
	if !reflect.DeepEqual(got.ids(), read.ids()) {
		t.Errorf("findings: %v, want %v", got.ids(), read.ids())
	}
	if strings.Join(read.Packages, ",") != "package:app,package:fixnum" || strings.Join(got.Packages, ",") != "package:app" {
		t.Errorf("packages analysed: %v, and %v with the declaration", read.Packages, got.Packages)
	}
	if again := run("-body-replaced", "package:fixnum", "-read-declared-bodies"); !reflect.DeepEqual(again.ids(), read.ids()) || again.Stats.Functions != read.Stats.Functions {
		t.Errorf("-read-declared-bodies is the run without the declaration: %d functions, %d", again.Stats.Functions, read.Stats.Functions)
	}
}

// A declaration in a rule file is in the key of an index with the file: another declaration is another index. And the
// packages of -body-replaced are in it too, whatever the order they were named in.
func TestBodyReplacedIsInTheIndexKey(t *testing.T) {
	t.Parallel()
	app, _ := graphProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-graph", db, "-body-replaced", "example.com/dep", "-body-replaced", "example.com/other", "./...")
	fresh := func(args ...string) bool {
		info := graphInfoOf(t, binary, app, nil, append([]string{"-graph", db, "-C", app}, append(args, "./...")...)...)
		return info.Snapshot.Fresh != nil && *info.Snapshot.Fresh
	}
	if !fresh("-body-replaced", "example.com/other", "-body-replaced", "example.com/dep") {
		t.Error("the same packages in another order: the index is the same")
	}
	if fresh("-body-replaced", "example.com/dep") {
		t.Error("another list of packages is another index")
	}
	if fresh("-read-declared-bodies") {
		t.Error("an index that reads the declared bodies is not the one that leaves them out")
	}
	if !fresh() {
		t.Error("a question that names no packages holds the file against the ones it was made with")
	}
	rules := filepath.Join(t.TempDir(), "d.yaml")
	write(t, rules, "ruleset: d@1\nbody_replaced_packages:\n  - {package: example.com/dep, reason: r}\n")
	db2 := filepath.Join(t.TempDir(), "g2.db")
	runIn(t, binary, app, nil, "index", "-graph", db2, "-rules", rules, "./...")
	check := func() bool {
		info := graphInfoOf(t, binary, app, nil, "-graph", db2, "-C", app, "-rules", rules, "./...")
		return info.Snapshot.Fresh != nil && *info.Snapshot.Fresh
	}
	if !check() {
		t.Fatal("unchanged rule file: fresh")
	}
	write(t, rules, "ruleset: d@1\nbody_replaced_packages:\n  - {package: example.com/dep, reason: r}\n  - {package: example.com/x, reason: r}\n")
	if check() {
		t.Error("a declaration added to the rule file: the index is stale")
	}
}

// The extractor that is told not to read a declared package (stage 4) and the filter of the core after it read it
// (stage 2, -drop-after-read) make the same program: the same findings, the same notes, the same functions. What the
// report says was left out is counted by what each saw (the filter: the functions that were read; the extractor: the
// functions the package declares).
func TestBodyReplacedInTheExtractorIsTheSameAsTheFilter(t *testing.T) {
	t.Parallel()
	app := bodyReplacedProject(t)
	rules := brRules(t, app, "decl.yaml", `ruleset: t@1`+brDeclare+`
propagators:
  - symbol: example.com/third.Run
    flows: [{from: arg0, to: result}]
`)
	for _, args := range [][]string{
		{"-rules", rules},
		{"-body-replaced", "example.com/third/...", "-body-replaced", "net/http"},
		{"-body-replaced", "example.com/third/sub"},
	} {
		extractor := brAnalyze(t, app, args...)
		filter := brAnalyze(t, app, append([]string{"-drop-after-read"}, args...)...)
		if !reflect.DeepEqual(extractor.ids(), filter.ids()) || extractor.Stats.Functions != filter.Stats.Functions {
			t.Errorf("%v: findings %v (%d functions) in the extractor, %v (%d) in the filter", args, extractor.ids(), extractor.Stats.Functions, filter.ids(), filter.Stats.Functions)
		}
		if !reflect.DeepEqual(extractor.notes("unmodeled_library_call"), filter.notes("unmodeled_library_call")) || strings.Join(extractor.Packages, ",") != strings.Join(filter.Packages, ",") {
			t.Errorf("%v: the notes or the packages differ", args)
		}
		if len(extractor.Summary.Replaced) != len(filter.Summary.Replaced) {
			t.Errorf("%v: summary %+v, %+v in the filter", args, extractor.Summary.Replaced, filter.Summary.Replaced)
			continue
		}
		for i, e := range extractor.Summary.Replaced {
			f := filter.Summary.Replaced[i]
			if e.Package != f.Package || e.DeclaredIn != f.DeclaredIn || e.Reason != f.Reason || e.Declared == 0 || e.Functions != 0 || f.Declared != 0 || f.Functions == 0 {
				t.Errorf("%v: summary %+v, %+v in the filter", args, e, f)
			}
		}
	}
}

// A generic function and a generic type of a package that is not read: go/ssa has no body to instantiate, and the
// program is the one the filter makes.
func TestBodyReplacedGenericsInTheExtractor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	app, third := filepath.Join(root, "app"), filepath.Join(root, "third")
	write(t, filepath.Join(third, "go.mod"), "module example.com/third\n\ngo 1.21\n")
	write(t, filepath.Join(third, "third.go"), `package third

type Box[T any] struct{ v T }

func (b *Box[T]) Get() T { return b.v }

func Of[T any](v T) *Box[T] { return &Box[T]{v} }

func Map[T any](v T, f func(T) T) T { return f(v) }

type Named string

func (n Named) String() string { return string(n) }
`)
	write(t, filepath.Join(app, "go.mod"), "module example.com/app\n\ngo 1.21\n\nrequire example.com/third v0.0.0\n\nreplace example.com/third => ../third\n")
	write(t, filepath.Join(app, "main.go"), `package main

import (
	"fmt"
	"net/http"
	"os/exec"

	"example.com/third"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		b := third.Of(q)
		exec.Command(b.Get()).Run()
		exec.Command(third.Map(q, func(s string) string { return s })).Run()
		fmt.Println(third.Named(q).String())
		var s fmt.Stringer = third.Named(q)
		exec.Command(s.String()).Run()
	})
}
`)
	extractor := brAnalyze(t, app, "-body-replaced", "example.com/third")
	filter := brAnalyze(t, app, "-body-replaced", "example.com/third", "-drop-after-read")
	if len(extractor.ids()) == 0 {
		t.Error("no finding: the test says nothing")
	}
	if !reflect.DeepEqual(extractor.ids(), filter.ids()) || extractor.Stats.Functions != filter.Stats.Functions {
		t.Errorf("findings %v (%d functions) in the extractor, %v (%d) in the filter", extractor.ids(), extractor.Stats.Functions, filter.ids(), filter.Stats.Functions)
	}
}

// The Dart extractor told --body-replaced does not write the functions of the package (it does not resolve its files),
// says how many functions it declares, and writes the rest as it does without the flag (the functions of the other
// packages, and of the project, are the same).
func TestDartExtractorBodyReplaced(t *testing.T) {
	t.Parallel()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	dartTreeIR(t, dart, "monorepo/app", "monorepo/app", "monorepo/shared")
	words := strings.Fields(dart)
	run := func(flags ...string) string {
		args := append(append([]string{}, words[1:]...), "run", filepath.Join(extractor, "bin", "rivus_dart.dart"))
		args = append(append(args, flags...), filepath.Join(extractor, "test", "monorepo", "app"))
		cmd := exec.Command(words[0], args...)
		cmd.Dir = extractor
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("extractor %v: %v", flags, err)
		}
		return string(out)
	}
	funcsOf := func(stream string) (n int, fixnum int, noBody string) {
		for _, line := range strings.Split(stream, "\n") {
			switch {
			case strings.HasPrefix(line, `{"kind":"func"`):
				n++
				if strings.Contains(line, `"id":"package:fixnum/`) {
					fixnum++
				}
			case strings.Contains(line, `"kind":"body_not_read"`):
				noBody = line
			}
		}
		return
	}
	n, fixnum, noBody := funcsOf(run())
	if fixnum == 0 || noBody != "" {
		t.Fatalf("without the flag: %d functions of fixnum, %q", fixnum, noBody)
	}
	m, left, noBody := funcsOf(run("--body-replaced", "package:fixnum", "--body-replaced", "package:no_such"))
	if left != 0 || m != n-fixnum {
		t.Errorf("with the flag: %d functions (%d of fixnum), want %d and none of fixnum", m, left, n-fixnum)
	}
	if !strings.Contains(noBody, `"unit":"package:fixnum"`) || strings.Contains(noBody, "no_such") {
		t.Errorf("body_not_read: %q", noBody)
	}
}
