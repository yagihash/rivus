package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A handler that no call reaches: it is registered with a router (named as a value, stored in a map).
func refsProject(t *testing.T) string {
	t.Helper()
	app := t.TempDir()
	write(t, filepath.Join(app, "go.mod"), "module example.com/refs\n\ngo 1.21\n")
	write(t, filepath.Join(app, "main.go"), `package main

type router struct{ h map[string]func() }

func (r *router) Get(p string, f func()) { r.h[p] = f }

func Activity() {}

func routes(r *router) {
	r.Get("/activity", Activity)
}

func main() {
	routes(&router{h: map[string]func(){}})
}
`)
	return app
}

type refsOut struct {
	Relation  string   `json:"relation"`
	Reachable bool     `json:"reachable"`
	Certainty string   `json:"certainty"`
	Searched  []string `json:"searched"`
	Exhaust   bool     `json:"exhaustive"`
	Listed    struct {
		Stage string `json:"stage"`
		Refs  *int   `json:"refs"`
	} `json:"listed"`
	Limits      []string `json:"limits"`
	Omitted     map[string]int
	Filters     []string `json:"filters"`
	TargetFacts []struct {
		ValueReferences *int `json:"value_references"`
		Owners          []struct {
			Func  string `json:"func"`
			Count int    `json:"count"`
		} `json:"value_reference_owners"`
	} `json:"target_facts"`
	Paths []struct {
		Relation  string `json:"relation"`
		Certainty string `json:"certainty"`
		Note      string `json:"note"`
		Steps     []struct {
			Func      string `json:"func"`
			At        string `json:"at"`
			Kind      string `json:"kind"`
			RefKind   string `json:"ref_kind"`
			ViaCallee string `json:"via_callee"`
			Note      string `json:"note"`
		} `json:"steps"`
	} `json:"paths"`
	Edges []struct {
		From      string `json:"from"`
		To        string `json:"to"`
		At        string `json:"at"`
		Kind      string `json:"kind"`
		RefKind   string `json:"ref_kind"`
		ViaCallee string `json:"via_callee"`
	} `json:"edges"`
}

func refsQuery(t *testing.T, dir string, args ...string) refsOut {
	t.Helper()
	out := runIn(t, binary, dir, nil, append([]string{"graph"}, args...)...)
	var r refsOut
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return r
}

func TestGraphPathThroughAReference(t *testing.T) {
	t.Parallel()
	app := refsProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-graph", db, "./...")
	// from the entry: main -> routes (a call), routes holds Activity as a value (the registration line)
	r := refsQuery(t, app, "path", "-graph", db, "--from-entry", "--to", "example.com/refs.Activity")
	if !r.Reachable || r.Certainty != "reference" || r.Relation != "calls+refs" || r.Listed.Stage != "ref" || r.Listed.Refs == nil || *r.Listed.Refs != 1 ||
		strings.Join(r.Searched, ",") != "static,dynamic,hub,ref" || len(r.Paths) == 0 {
		t.Fatalf("%+v", r)
	}
	p := r.Paths[0]
	if p.Relation != "calls+refs" || p.Note == "" || len(p.Steps) != 3 || p.Steps[0].Func != "example.com/refs.main" {
		t.Fatalf("%+v", p)
	}
	line := lineOf(t, app, `r.Get("/activity", Activity)`)
	reg := p.Steps[1]
	if reg.Kind != "ref" || reg.RefKind != "arg" || !strings.HasPrefix(reg.At, "main.go:"+itoa(line)+":") || !strings.Contains(reg.ViaCallee, "Get") || reg.Note == "" {
		t.Errorf("the registration: %+v (line %d)", reg, line)
	}
	if p.Steps[0].Kind != "static" || p.Steps[2].Func != "example.com/refs.Activity" {
		t.Errorf("%+v", p.Steps)
	}
	// --no-refs: no path, and not exhaustive (a stored function is not an edge of this search)
	n := refsQuery(t, app, "path", "-graph", db, "--from-entry", "--to", "example.com/refs.Activity", "--no-refs")
	if n.Reachable || n.Exhaust || len(n.Limits) != 3 || len(n.Searched) != 3 || n.Relation != "calls" ||
		len(n.TargetFacts) != 1 || n.TargetFacts[0].ValueReferences == nil || *n.TargetFacts[0].ValueReferences != 1 {
		t.Errorf("%+v", n)
	}
	// --refs begins at the fourth stage; both flags together are an error
	if q := refsQuery(t, app, "path", "-graph", db, "--from", "example.com/refs.routes", "--to", "example.com/refs.Activity", "--refs"); !q.Reachable || q.Listed.Stage != "ref" {
		t.Errorf("%+v", q)
	}
	if out := runFail(t, app, "graph", "path", "-graph", db, "--from-entry", "--to", "x", "--refs", "--no-refs"); !strings.Contains(out, "--refs") {
		t.Errorf("%s", out)
	}
	// callers: the references are left out, and counted; --refs adds them as edges of kind ref
	c := refsQuery(t, app, "callers", "-graph", db, "example.com/refs.Activity")
	if len(c.Edges) != 0 || c.Omitted["refs"] != 1 {
		t.Errorf("%+v", c)
	}
	c = refsQuery(t, app, "callers", "-graph", db, "example.com/refs.Activity", "--refs")
	if len(c.Edges) != 1 || c.Edges[0].Kind != "ref" || c.Edges[0].From != "example.com/refs.routes" || c.Edges[0].RefKind != "arg" || !strings.Contains(c.Edges[0].ViaCallee, "Get") || len(c.Filters) == 0 {
		t.Errorf("%+v", c)
	}
	d := refsQuery(t, app, "callees", "-graph", db, "example.com/refs.routes", "--refs")
	found := false
	for _, e := range d.Edges {
		found = found || (e.Kind == "ref" && e.To == "example.com/refs.Activity")
	}
	if !found {
		t.Errorf("%+v", d.Edges)
	}
	if out := runFail(t, app, "graph", "impls", "-graph", db, "--refs", "x"); !strings.Contains(out, "--refs") {
		t.Errorf("%s", out)
	}
	// the table is in the file, and graph info counts it
	info := runIn(t, binary, app, nil, "graph", "info", "-graph", db)
	var in struct {
		Graph struct {
			SchemaVersion int `json:"schema_version"`
			Counts        struct {
				Refs int `json:"refs"`
			} `json:"counts"`
		} `json:"graph"`
	}
	if err := json.Unmarshal(info, &in); err != nil || in.Graph.SchemaVersion != 2 || in.Graph.Counts.Refs < 1 {
		t.Errorf("%v %s", err, info)
	}
}

// A file of the first schema is refused, and the message says to make it again.
func TestGraphRefusesTheFirstSchema(t *testing.T) {
	t.Parallel()
	app := refsProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-graph", db, "./...")
	b, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(t.TempDir(), "old.db")
	// user_version is the 4 bytes at offset 60 of the header
	b[60], b[61], b[62], b[63] = 0, 0, 0, 1
	os.WriteFile(old, b, 0o600)
	for _, cmd := range [][]string{{"info"}, {"callers", "x"}, {"path", "--from-entry", "--to", "x"}} {
		out := runFail(t, app, append([]string{"graph", cmd[0], "-graph", old}, cmd[1:]...)...)
		if !strings.Contains(out, "version 1") || !strings.Contains(out, "rivus index") || !strings.Contains(out, "version 2") {
			t.Errorf("%v: %s", cmd, out)
		}
	}
}
