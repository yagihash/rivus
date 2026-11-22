package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/testutil"
)

// run runs the binary in dir and returns what it printed (stdout), or fails.
func runIn(t *testing.T, bin, dir string, env []string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rivus %v: %v\n%s", args, err, stderr.String())
	}
	return out
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// graphProject is a module with a static call, a call through an interface and a call of a function value,
// and a module it depends on by a path (replace).
func graphProject(t *testing.T) (app, dep string) {
	t.Helper()
	root := t.TempDir()
	app, dep = filepath.Join(root, "app"), filepath.Join(root, "dep")
	write(t, filepath.Join(dep, "go.mod"), "module example.com/dep\n\ngo 1.21\n")
	write(t, filepath.Join(dep, "dep.go"), "package dep\n\nfunc Double(n int) int { return n * 2 }\n")
	write(t, filepath.Join(app, "go.mod"), "module example.com/app\n\ngo 1.21\n\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => ../dep\n")
	write(t, filepath.Join(app, "main.go"), `package main

import (
	"fmt"
	"os/exec"

	"example.com/dep"
)

type Runner interface{ Run(s string) error }

type shell struct{}

func (shell) Run(s string) error { return exec.Command("sh", "-c", s).Run() }

type quiet struct{}

func (quiet) Run(s string) error { return nil }

func apply(f func(int) int, n int) int { return f(n) }

func main() {
	var r Runner = shell{}
	if len(fmt.Sprint(dep.Double(2))) > 3 {
		r = quiet{}
	}
	_ = r.Run("ls")
	fmt.Println(apply(dep.Double, 3))
}
`)
	write(t, filepath.Join(app, "main_test.go"), "package main\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) { main() }\n")
	write(t, filepath.Join(app, "tagged.go"), "//go:build never\n\npackage main\n\nfunc tagged() {}\n")
	return app, dep
}

type infoOut struct {
	Graph struct {
		SourceHash string `json:"source_hash"`
		Languages  []string
		Counts     graphdb.Counts `json:"counts"`
		SizeBytes  int64          `json:"size_bytes"`
	} `json:"graph"`
	Snapshot struct {
		Verified bool     `json:"verified"`
		Fresh    *bool    `json:"fresh"`
		Reasons  []string `json:"stale_reasons"`
		Note     string
	} `json:"snapshot"`
	Layers struct {
		Present []string `json:"present"`
		Taint   string   `json:"taint"`
	} `json:"layers"`
}

func graphInfoOf(t *testing.T, bin, dir string, env []string, args ...string) infoOut {
	t.Helper()
	out := runIn(t, bin, dir, env, append([]string{"graph", "info"}, args...)...)
	var info infoOut
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return info
}

func TestGraphIndexIsFreshUntilSomethingChanges(t *testing.T) {
	app, dep := graphProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	run := func(args ...string) []byte { return runIn(t, binary, app, nil, args...) }
	run("index", "-graph", db, "./...")

	info := graphInfoOf(t, binary, app, nil, "-graph", db, "-C", app, "./...")
	if !info.Snapshot.Verified || info.Snapshot.Fresh == nil || !*info.Snapshot.Fresh {
		t.Fatalf("an unchanged tree is fresh: %+v", info.Snapshot)
	}
	// without -C, nothing is said to match
	noC := graphInfoOf(t, binary, app, nil, "-graph", db)
	if noC.Snapshot.Verified || noC.Snapshot.Fresh != nil || noC.Snapshot.Note == "" {
		t.Errorf("without -C the match is not checked, and says so: %+v", noC.Snapshot)
	}
	if len(info.Layers.Present) != 1 || info.Layers.Present[0] != "structure" || !strings.Contains(info.Layers.Taint, "absent") {
		t.Errorf("layers: %+v", info.Layers)
	}
	// the same again, and again: the key does not drift
	for i := 0; i < 2; i++ {
		if again := graphInfoOf(t, binary, app, nil, "-graph", db, "-C", app, "./..."); !*again.Snapshot.Fresh {
			t.Fatalf("a second check is not fresh: %v", again.Snapshot.Reasons)
		}
	}

	// each change makes the file stale, and is undone before the next
	changes := []struct {
		name string
		do   func() (undo func())
		args []string
		env  []string
		pkgs string
	}{
		{name: "one byte of a source file", do: edit(t, filepath.Join(app, "main.go"), func(s string) string { return s + "\n" })},
		{name: "a new file in the package", do: create(t, filepath.Join(app, "extra.go"), "package main\n")},
		{name: "a new file that is not Go", do: create(t, filepath.Join(app, "notes.txt"), "x")},
		{name: "a file build constraints leave out", do: edit(t, filepath.Join(app, "tagged.go"), func(s string) string { return s + "// x\n" })},
		{name: "go.mod", do: edit(t, filepath.Join(app, "go.mod"), func(s string) string { return s + "\n// x\n" })},
		{name: "go.sum appears", do: create(t, filepath.Join(app, "go.sum"), "")},
		{name: "a file of a module that replace points to", do: edit(t, filepath.Join(dep, "dep.go"), func(s string) string { return strings.Replace(s, "n * 2", "n * 3", 1) })},
		{name: "a new file of a module that replace points to", do: create(t, filepath.Join(dep, "more.go"), "package dep\n")},
		{name: "the go.mod of a module that replace points to", do: edit(t, filepath.Join(dep, "go.mod"), func(s string) string { return s + "\n// x\n" })},
		{name: "-tests", args: []string{"-tests"}},
		{name: "-no-deps", args: []string{"-no-deps"}},
		{name: "-k", args: []string{"-k", "2"}},
		{name: "-numeric-taint", args: []string{"-numeric-taint"}},
		{name: "-max-implementations", args: []string{"-max-implementations", "7"}},
		{name: "-lang (another language is read too)", args: []string{"-lang", "go,dart"}},
		{name: "-no-builtin-rules", args: []string{"-no-builtin-rules"}},
		{name: "-deterministic-only", args: []string{"-deterministic-only"}},
		{name: "the packages", pkgs: "./nothing"},
		{name: "GOFLAGS (the build tags)", env: []string{"GOFLAGS=-tags=never"}},
		{name: "GOOS", env: []string{"GOOS=" + otherGOOS()}},
		{name: "CGO_ENABLED", env: []string{"CGO_ENABLED=0"}},
		{name: "a project rule file appears", do: create(t, filepath.Join(app, ".rivus", "rules", "r.yaml"), "ruleset: t@1\nsinks:\n  - symbol: os/exec.Command\n    category: x\n")},
		{name: "-rules", do: create(t, filepath.Join(app, "..", "extra-rules.yaml"), "ruleset: t@1\nsinks:\n  - symbol: os.Open\n    category: x\n"),
			args: []string{"-rules", filepath.Join(app, "..", "extra-rules.yaml")}},
	}
	for _, c := range changes {
		t.Run(c.name, func(t *testing.T) {
			var undo func()
			if c.do != nil {
				undo = c.do()
			}
			if undo != nil {
				defer undo()
			}
			args := append([]string{"-graph", db, "-C", app}, c.args...)
			if c.pkgs == "" {
				c.pkgs = "./..."
			}
			args = append(args, c.pkgs)
			info := graphInfoOf(t, binary, app, c.env, args...)
			if info.Snapshot.Fresh == nil || *info.Snapshot.Fresh {
				t.Fatalf("%s: the file must be stale, and it is not", c.name)
			}
			if len(info.Snapshot.Reasons) == 0 {
				t.Errorf("%s: stale, with no reason", c.name)
			}
			t.Logf("%s: %v", c.name, info.Snapshot.Reasons)
		})
	}
	// every change was undone: fresh again (the key is of the content, not of the time)
	if again := graphInfoOf(t, binary, app, nil, "-graph", db, "-C", app, "./..."); !*again.Snapshot.Fresh {
		t.Fatalf("after the changes were undone it is fresh again: %v", again.Snapshot.Reasons)
	}
}

func edit(t *testing.T, path string, f func(string) string) func() (undo func()) {
	return func() func() {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		write(t, path, f(string(b)))
		return func() { write(t, path, string(b)) }
	}
}

func create(t *testing.T, path, content string) func() (undo func()) {
	return func() func() {
		write(t, path, content)
		return func() { os.Remove(path) }
	}
}

// What is not read is not a change: a test file that -tests did not ask for.
func TestGraphIgnoresWhatWasNotRead(t *testing.T) {
	app, _ := graphProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-graph", db, "./...")
	write(t, filepath.Join(app, "main_test.go"), "package main\n\n// changed\n")
	if info := graphInfoOf(t, binary, app, nil, "-graph", db, "-C", app, "./..."); !*info.Snapshot.Fresh {
		t.Errorf("a _test.go file that was not read changes nothing: %v", info.Snapshot.Reasons)
	}
	// with -tests it is read
	runIn(t, binary, app, nil, "index", "-tests", "-graph", db, "./...")
	write(t, filepath.Join(app, "main_test.go"), "package main\n\n// changed again\n")
	if info := graphInfoOf(t, binary, app, nil, "-tests", "-graph", db, "-C", app, "./..."); *info.Snapshot.Fresh {
		t.Errorf("with -tests, a _test.go file is read")
	}
}

func TestGraphIsStaleForAnotherRivus(t *testing.T) {
	app, _ := graphProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-graph", db, "./...")
	other := filepath.Join(t.TempDir(), "rivus-other")
	if out, err := exec.Command("go", "build", "-ldflags", "-X main.versionStamp=other", "-o", other, ".").CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	info := graphInfoOf(t, other, app, nil, "-graph", db, "-C", app, "./...")
	if *info.Snapshot.Fresh {
		t.Fatal("a file made by another build of rivus is stale")
	}
	found := false
	for _, r := range info.Snapshot.Reasons {
		found = found || strings.HasPrefix(r, "rivus:")
	}
	if !found {
		t.Errorf("the reasons name rivus: %v", info.Snapshot.Reasons)
	}
}

func TestGraphDefaultsToTheUserCache(t *testing.T) {
	app, _ := graphProject(t)
	cache := t.TempDir()
	env := []string{"RIVUS_CACHE_DIR=" + cache}
	runIn(t, binary, app, env, "index", "-C", app, "./...")
	if entries, _ := os.ReadDir(filepath.Join(app, ".rivus")); len(entries) != 0 {
		t.Errorf("nothing is written in the analysed directory: %v", entries)
	}
	matches, _ := filepath.Glob(filepath.Join(cache, "*", "graph.db"))
	if len(matches) != 1 {
		t.Fatalf("the file is in the cache: %v", matches)
	}
	// found again from -C, and checked
	info := graphInfoOf(t, binary, app, env, "-C", app, "./...")
	if info.Snapshot.Fresh == nil || !*info.Snapshot.Fresh {
		t.Errorf("%+v", info.Snapshot)
	}
	// a file in the analysed directory is not found by itself
	if out, err := exec.Command(binary, "graph", "info").CombinedOutput(); err == nil {
		t.Errorf("graph info with neither -graph nor -C must fail: %s", out)
	}
}

func TestGraphRefusesAnotherSchemaAndForeignFiles(t *testing.T) {
	app, _ := graphProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-graph", db, "./...")
	// the same file with another schema version
	b, _ := os.ReadFile(db)
	other := filepath.Join(t.TempDir(), "old.db")
	// user_version is the 4 bytes at offset 60 of the header
	b[60], b[61], b[62], b[63] = 0, 0, 0, 99
	os.WriteFile(other, b, 0o600)
	cmd := exec.Command(binary, "graph", "info", "-graph", other)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "rivus index") || !strings.Contains(string(out), "version 99") {
		t.Errorf("another schema version is refused, with the way to make it again: %v %s", err, out)
	}
	// a file that is not a database
	junk := filepath.Join(t.TempDir(), "junk.db")
	os.WriteFile(junk, []byte("this is not sqlite, it is a file somebody put there"), 0o600)
	if out, err := exec.Command(binary, "graph", "info", "-graph", junk).CombinedOutput(); err == nil {
		t.Errorf("a file that is not a graph file is refused: %s", out)
	}
	// the file is opened read only: info does not change it
	before, _ := os.Stat(db)
	graphInfoOf(t, binary, app, nil, "-graph", db)
	after, _ := os.Stat(db)
	if !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		t.Error("reading changed the file")
	}
}

// What the file holds is what extract and the graph of the analysis have: the numbers of functions, calls and
// edges, counted from them.
func TestGraphCountsAgreeWithTheProgram(t *testing.T) {
	app, _ := graphProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-no-deps", "-graph", db, "./...")
	info := graphInfoOf(t, binary, app, nil, "-graph", db)

	// extract -all: the functions with a body, and the calls they make
	out := runIn(t, binary, app, nil, "debug", "extract", "-all", "./...")
	prog, err := ir.Read(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, f := range prog.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpCall {
					calls++
				}
			}
		}
	}
	c := info.Graph.Counts
	if c.FuncsWithBody != len(prog.Funcs) {
		t.Errorf("functions with a body: the file has %d, extract has %d", c.FuncsWithBody, len(prog.Funcs))
	}
	if c.CallSites != calls {
		t.Errorf("call sites: the file has %d, extract has %d", c.CallSites, calls)
	}
	if c.Funcs < c.FuncsWithBody {
		t.Errorf("the functions without a body are in the file too: %+v", c)
	}
	if c.Edges == 0 || c.Files == 0 {
		t.Errorf("%+v", c)
	}

	// the kinds of the calls are kept apart
	d, err := graphdb.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	at, err := d.CallSitesAt("main.go", lineOf(t, app, `r.Run("ls")`), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(at) == 0 {
		t.Fatal("no call on the line of r.Run")
	}
	var run *graphdb.CallSite
	for i := range at {
		if at[i].Kind == graphdb.KindInterface {
			run = &at[i]
		}
	}
	if run == nil || len(run.Targets) != 2 {
		t.Fatalf("a call through an interface has the implementations as candidates: %+v", at)
	}
	var ids []string
	for _, tg := range run.Targets {
		ids = append(ids, tg.ID)
	}
	sort.Strings(ids)
	if ids[0] != "(example.com/app.quiet).Run" && !strings.Contains(ids[0], "quiet") {
		t.Errorf("candidates: %v", ids)
	}
	// the position of the call decides it: file:line:col is one call
	one, err := d.CallSitesAt(run.File, run.Line, run.Col)
	if err != nil || len(one) != 1 || one[0].ID != run.ID {
		t.Errorf("file:line:col is one call site: %v %+v", err, one)
	}
	// the call of a function value
	vals, _ := d.CallSitesAt("main.go", lineOf(t, app, `return f(n)`), 0)
	kinds := map[string]bool{}
	for _, v := range vals {
		kinds[v.Kind] = true
	}
	if !kinds[graphdb.KindValue] {
		t.Errorf("f(n) is a call of a function value: %+v", vals)
	}
}

// Every call site of a file has one position; the sites of the file are the sites that "rivus sites" lists, with the same IDs
// and positions, and a site of a call is tied to its call site.
func TestGraphSitesAreTheSitesOfTheSitesCommand(t *testing.T) {
	dir := testutil.Testdata()
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, dir, nil, "index", "-no-project-rules", "-rules", "rules.yaml", "-graph", db, "./cases/...")
	var want struct {
		Sites []struct {
			ID       string `json:"id"`
			Position struct {
				File   string `json:"file"`
				Line   int    `json:"line"`
				Column int    `json:"column"`
			} `json:"position"`
		} `json:"sites"`
	}
	if err := json.Unmarshal(runIn(t, binary, dir, nil, "sites", "-no-project-rules", "-rules", "rules.yaml", "./cases/..."), &want); err != nil {
		t.Fatal(err)
	}
	d, err := graphdb.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	got, err := d.Sites()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want.Sites) {
		t.Fatalf("the file has %d sites, rivus sites lists %d", len(got), len(want.Sites))
	}
	byID := map[string]graphdb.Site{}
	for _, s := range got {
		byID[s.ID] = s
	}
	tied := 0
	for _, w := range want.Sites {
		s, ok := byID[w.ID]
		if !ok {
			t.Errorf("site %s is not in the file", w.ID)
			continue
		}
		if s.File != w.Position.File || s.Line != w.Position.Line || s.Col != w.Position.Column {
			t.Errorf("site %s: %s:%d:%d, want %s:%d:%d", w.ID, s.File, s.Line, s.Col, w.Position.File, w.Position.Line, w.Position.Column)
		}
		if s.CallSite != 0 {
			tied++
		}
	}
	if tied == 0 {
		t.Error("no site is tied to a call site")
	}
}

func lineOf(t *testing.T, dir, text string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for i, l := range strings.Split(string(b), "\n") {
		if strings.Contains(l, text) {
			return i + 1
		}
	}
	t.Fatalf("no %q in main.go", text)
	return 0
}

// The key does not depend on how the directory is named: index from inside it, check with -C, with a rule file in it.
func TestGraphKeyIsTheSameWhereverItIsMade(t *testing.T) {
	app, _ := graphProject(t)
	write(t, filepath.Join(app, ".rivus", "rules", "r.yaml"), "ruleset: t@1\nsinks:\n  - symbol: os/exec.Command\n    category: x\n")
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-graph", db)
	elsewhere := t.TempDir()
	if info := graphInfoOf(t, binary, elsewhere, nil, "-graph", db, "-C", app); !*info.Snapshot.Fresh {
		t.Errorf("checked from another directory: %v", info.Snapshot.Reasons)
	}
	// the rule file is part of it
	write(t, filepath.Join(app, ".rivus", "rules", "r.yaml"), "ruleset: t@1\nsinks:\n  - symbol: os/exec.CommandContext\n    category: x\n")
	if info := graphInfoOf(t, binary, elsewhere, nil, "-graph", db, "-C", app); *info.Snapshot.Fresh {
		t.Error("a changed project rule file makes the file stale")
	}
}

// otherGOOS is a GOOS that is not the one the test runs on: setting the one it runs on changes nothing.
func otherGOOS() string {
	if runtime.GOOS == "linux" {
		return "darwin"
	}
	return "linux"
}
