package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/graphdb"
)

// what the commands print, as far as the tests look
type queryOut struct {
	Schema   string `json:"schema"`
	Relation string `json:"relation"`
	Snapshot struct {
		Verified bool  `json:"verified"`
		Fresh    *bool `json:"fresh"`
	} `json:"snapshot"`
	Query struct {
		Op    string `json:"op"`
		Depth int    `json:"depth"`
	} `json:"query"`
	Truncated struct {
		Count int `json:"count"`
		Limit int `json:"limit"`
	} `json:"truncated"`
	Omitted   map[string]int `json:"omitted"`
	Filters   []string       `json:"filters"`
	Hints     []string       `json:"hints"`
	Warnings  []string       `json:"warnings"`
	Start     []string       `json:"start"`
	Reachable bool           `json:"reachable"`
	Certainty string         `json:"certainty"`
	Nodes     []struct {
		ID string `json:"id"`
	} `json:"nodes"`
	Edges []struct {
		From       string `json:"from"`
		To         string `json:"to"`
		At         string `json:"at"`
		Kind       string `json:"kind"`
		Candidates int    `json:"candidates"`
	} `json:"edges"`
	Frontier []struct {
		At         string `json:"at"`
		Candidates int    `json:"candidates"`
	} `json:"frontier"`
	Ambiguous []struct {
		At string `json:"at"`
	} `json:"ambiguous"`
	Paths []struct {
		Certainty string `json:"certainty"`
		Steps     []struct {
			Func string `json:"func"`
			At   string `json:"at"`
			Kind string `json:"kind"`
		} `json:"steps"`
	} `json:"paths"`
}

func graphQuery(t *testing.T, dir string, args ...string) queryOut {
	t.Helper()
	out := runIn(t, binary, dir, nil, append([]string{"graph"}, args...)...)
	var q queryOut
	if err := json.Unmarshal(out, &q); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if q.Schema != "rivus.graph/1" || q.Relation != "calls" || q.Filters == nil || q.Omitted == nil {
		t.Errorf("the common fields: %s", out)
	}
	return q
}

func TestGraphCallersCalleesPathOfGoCode(t *testing.T) {
	t.Parallel()
	app, _ := graphProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-graph", db, "./...")

	// a symbol: who calls dep.Double (main, and apply by the function value)
	q := graphQuery(t, app, "callers", "-graph", db, "example.com/dep.Double")
	var from []string
	for _, e := range q.Edges {
		from = append(from, e.From+":"+e.Kind)
	}
	if !strings.Contains(strings.Join(from, " "), "example.com/app.main:static") || !strings.Contains(strings.Join(from, " "), "example.com/app.apply:value") {
		t.Errorf("%v", from)
	}
	if q.Snapshot.Verified || q.Snapshot.Fresh != nil {
		t.Errorf("without -C the match is not checked: %+v", q.Snapshot)
	}
	for _, e := range q.Edges {
		if e.Kind == "value" && e.Candidates == 0 {
			t.Errorf("a dynamic edge has its candidates: %+v", e)
		}
	}
	// the flags may follow the target
	if q2 := graphQuery(t, app, "callers", "example.com/dep.Double", "--depth", "2", "-graph", db); q2.Query.Depth != 2 {
		t.Errorf("%+v", q2.Query)
	}
	// by a position, relative or absolute, with the same answer
	line := lineOf(t, app, `_ = r.Run("ls")`)
	rel := graphQuery(t, app, "callees", "-graph", db, "main.go:"+itoa(line))
	abs := graphQuery(t, app, "callees", "-graph", db, filepath.Join(app, "main.go")+":"+itoa(line))
	if len(rel.Edges) == 0 || len(rel.Edges) != len(abs.Edges) || rel.Edges[0].Kind != "interface" || rel.Edges[0].Candidates != 2 {
		t.Errorf("%+v / %+v", rel.Edges, abs.Edges)
	}
	// a line with several calls: the choice, not an error
	amb := graphQuery(t, app, "callers", "-graph", db, "main.go:"+itoa(lineOf(t, app, "if len(fmt.Sprint(dep.Double(2)))")))
	if len(amb.Ambiguous) < 2 || len(amb.Edges) != 0 || len(amb.Hints) == 0 {
		t.Errorf("%+v", amb)
	}
	for _, a := range amb.Ambiguous {
		// each of the choices is a call that can be asked for
		if got := graphQuery(t, app, "callees", "-graph", db, a.At); len(got.Start) != 1 {
			t.Errorf("%s: %+v", a.At, got)
		}
	}
	// the builtin len is left out, and counted
	c := graphQuery(t, app, "callees", "-graph", db, "example.com/app.main")
	if c.Omitted["builtin"] < 1 {
		t.Errorf("%v", c.Omitted)
	}
	for _, n := range c.Nodes {
		if strings.HasPrefix(n.ID, "builtin") {
			t.Errorf("%v", n)
		}
	}
	// the path: main to os/exec.Command goes through an interface, so it is uncertain
	p := graphQuery(t, app, "path", "-graph", db, "--from", "example.com/app.main", "--to", "os/exec.Command")
	if !p.Reachable || p.Certainty != "uncertain" || len(p.Paths) == 0 || p.Paths[0].Steps[0].Func != "example.com/app.main" {
		t.Errorf("%+v", p)
	}
	// a path that does not exist
	if n := graphQuery(t, app, "path", "-graph", db, "--from", "example.com/dep.Double", "--to", "example.com/app.main"); n.Reachable || len(n.Paths) != 0 {
		t.Errorf("%+v", n)
	}
	// a symbol that is not there: an error that names what is near
	cmd := runFail(t, app, "graph", "callers", "-graph", db, "os/exec.Comman")
	if !strings.Contains(cmd, "no function") {
		t.Errorf("%s", cmd)
	}
	// neither -graph nor -C
	if out := runFail(t, app, "graph", "callers", "x"); !strings.Contains(out, "-graph") {
		t.Errorf("%s", out)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func runFail(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("rivus %v: no error:\n%s", args, out)
	}
	return string(out)
}

// With -C the file is held against the source, and a stale one is said so in the answer.
func TestGraphQueryWarnsWhenTheFileIsStale(t *testing.T) {
	t.Parallel()
	app, _ := graphProject(t)
	run := func(args ...string) []byte { return runIn(t, binary, app, nil, args...) }
	run("index", "./...")
	var q queryOut
	if err := json.Unmarshal(run("graph", "callers", "-C", app, "example.com/dep.Double"), &q); err != nil {
		t.Fatal(err)
	}
	if !q.Snapshot.Verified || q.Snapshot.Fresh == nil || !*q.Snapshot.Fresh || len(q.Warnings) != 0 {
		t.Fatalf("an unchanged tree: %+v %v", q.Snapshot, q.Warnings)
	}
	edit(t, filepath.Join(app, "main.go"), func(s string) string { return s + "\nfunc extra() {}\n" })()
	q = queryOut{}
	if err := json.Unmarshal(run("graph", "callers", "-C", app, "example.com/dep.Double"), &q); err != nil {
		t.Fatal(err)
	}
	if !q.Snapshot.Verified || q.Snapshot.Fresh == nil || *q.Snapshot.Fresh || len(q.Warnings) == 0 || !strings.Contains(q.Warnings[0], "stale") {
		t.Errorf("a changed tree is said to be stale: %+v %v", q.Snapshot, q.Warnings)
	}
	if len(q.Edges) == 0 {
		t.Errorf("the answer is still given")
	}
}

// For a Go sample of the tests: every command works on the sample of the repository.
func TestGraphQueryOnTheSampleOfTheRepository(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	db := filepath.Join(dir, "g.db")
	sample := testdataGo(t)
	runIn(t, binary, sample, nil, "index", "-graph", db, "./...")
	d, err := graphdb.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := d.QueryForTest(`SELECT f.fid, 0, 0 FROM edges e JOIN funcs f ON f.id = e.to_func WHERE e.kind = 'static' AND f.has_body = 1 GROUP BY f.id ORDER BY count(*) DESC, f.fid LIMIT 1`)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	q := graphQuery(t, sample, "callers", "-graph", db, pos.File)
	if len(q.Edges) == 0 {
		t.Errorf("%s has no caller", pos.File)
	}
	// a path from a caller to it
	p := graphQuery(t, sample, "path", "-graph", db, "--from", q.Edges[0].From, "--to", pos.File)
	if !p.Reachable || len(p.Paths) == 0 {
		t.Errorf("%+v", p)
	}
}

func testdataGo(t *testing.T) string {
	t.Helper()
	d, err := filepath.Abs("../../testdata/go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d); err != nil {
		t.Skip(err)
	}
	return d
}
