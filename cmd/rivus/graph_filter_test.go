package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// what the filters, the entry and the short names add to the answers
type stage3Out struct {
	Flows    string   `json:"flows"`
	Filters  []string `json:"filters"`
	Hints    []string `json:"hints"`
	Searched []string `json:"searched"`
	Entry    *struct {
		Source     string   `json:"source"`
		Count      int      `json:"count"`
		Definition string   `json:"definition"`
		Examples   []string `json:"examples"`
	} `json:"entry"`
	WithoutFilters *struct {
		Reachable bool `json:"reachable"`
	} `json:"without_filters"`
	Reachable      bool  `json:"reachable"`
	Ambiguous      []any `json:"ambiguous"`
	AmbiguousTotal int   `json:"ambiguous_total"`
	Paths          []struct {
		Steps []struct {
			Func      string `json:"func"`
			DefinedAt string `json:"defined_at"`
			NoBody    bool   `json:"no_body"`
		} `json:"steps"`
	} `json:"paths"`
	Query struct {
		Resolved string `json:"resolved"`
	} `json:"query"`
}

func graphStage3(t *testing.T, dir string, args ...string) stage3Out {
	t.Helper()
	out := runIn(t, binary, dir, nil, append([]string{"graph"}, args...)...)
	var q stage3Out
	if err := json.Unmarshal(out, &q); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if q.Flows != "not_checked" {
		t.Errorf("flows: %s", out)
	}
	return q
}

func TestGraphFiltersEntryAndShortNamesOfGoCode(t *testing.T) {
	app, _ := graphProject(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, app, nil, "index", "-graph", db, "./...")

	// main reaches os/exec.Command only through Runner.Run: shell.Run or quiet.Run
	base := []string{"path", "-graph", db, "--from", "example.com/app.main", "--to", "os/exec.Command"}
	if p := graphStage3(t, app, base...); !p.Reachable || len(p.Filters) != 0 || p.WithoutFilters != nil || len(p.Searched) != 2 {
		t.Errorf("%+v", p)
	}
	// --only-impl: the implementation that does call it, and the one that does not
	if p := graphStage3(t, app, append(base, "--only-impl", "shell")...); !p.Reachable || len(p.Filters) != 1 || !strings.HasPrefix(p.Filters[0], "only-impl shell") {
		t.Errorf("%+v", p)
	}
	q := graphStage3(t, app, append(base, "--only-impl", "quiet")...)
	if q.Reachable || len(q.Filters) != 1 || q.WithoutFilters == nil || !q.WithoutFilters.Reachable {
		t.Errorf("%+v", q)
	}
	// --avoid and --via, repeated, with a short name (the type and the method)
	if p := graphStage3(t, app, append(base, "--via", "shell.Run")...); !p.Reachable || p.Paths[0].Steps[1].Func != "(example.com/app.shell).Run" {
		t.Errorf("%+v", p)
	}
	if p := graphStage3(t, app, append(base, "--avoid", "shell.Run")...); p.Reachable || p.WithoutFilters == nil || !p.WithoutFilters.Reachable {
		t.Errorf("%+v", p)
	}
	if p := graphStage3(t, app, append(base, "--avoid", "quiet.Run", "--avoid", "shell.Run")...); p.Reachable || len(p.Filters) != 2 {
		t.Errorf("%+v", p)
	}
	// the filters of callers and callees
	c := graphStage3(t, app, "callees", "-graph", db, "--only-impl", "quiet", "main.go:"+itoa(lineOf(t, app, `_ = r.Run("ls")`)))
	if len(c.Filters) != 1 {
		t.Errorf("%+v", c)
	}
	// the entry: main.main reaches it, and the answer says what an entry is
	e := graphStage3(t, app, "path", "-graph", db, "--from-entry", "--to", "os/exec.Command")
	if !e.Reachable || e.Entry == nil || e.Entry.Source != "entry" || e.Entry.Count < 1 || e.Entry.Definition == "" || e.Paths[0].Steps[0].Func == "" {
		t.Errorf("%+v", e)
	}
	if got := e.Paths[0].Steps[0].Func; got != "example.com/app.main" && !strings.HasSuffix(got, ".init") {
		t.Errorf("the first step is an entry: %s", got)
	}
	if last := e.Paths[0].Steps[len(e.Paths[0].Steps)-1]; last.DefinedAt == "" && !last.NoBody {
		t.Errorf("the end says where it is: %+v", last)
	}
	if out := runFail(t, app, "graph", "path", "-graph", db, "--to", "x"); !strings.Contains(out, "--from-entry") {
		t.Errorf("%s", out)
	}
	// a short name: the one function that ends so; several are a choice, not an error
	if s := graphStage3(t, app, "callers", "-graph", db, "Double"); len(s.Ambiguous) != 0 || !strings.Contains(s.Query.Resolved, "the end of it") {
		t.Errorf("%+v", s)
	}
	s := graphStage3(t, app, "callers", "-graph", db, "Run")
	if len(s.Ambiguous) < 2 || s.AmbiguousTotal < 2 || len(s.Hints) == 0 {
		t.Errorf("%+v", s)
	}
}
