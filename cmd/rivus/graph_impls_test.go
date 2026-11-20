package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// what impls and dispatchers print, as far as the tests look
type implsOut struct {
	Schema             string   `json:"schema"`
	Relation           string   `json:"relation"`
	Flows              string   `json:"flows"`
	Filters            []string `json:"filters"`
	Hints              []string `json:"hints"`
	Method             string   `json:"method"`
	Basis              string   `json:"basis"`
	CallSites          int      `json:"call_sites"`
	ZeroCandidateSites int      `json:"zero_candidate_sites"`
	Reason             string   `json:"reason"`
	ImplsTotal         int      `json:"impls_total"`
	Truncated          struct {
		Count int    `json:"count"`
		Unit  string `json:"unit"`
	} `json:"truncated"`
	Impls []struct {
		ID        string `json:"id"`
		CallSites int    `json:"call_sites"`
	} `json:"impls"`
	ByPackage      []struct{ Package string } `json:"by_package"`
	Ambiguous      []any                      `json:"ambiguous"`
	AmbiguousTotal int                        `json:"ambiguous_total"`
}

type dispatchersOut struct {
	Schema         string `json:"schema"`
	Flows          string `json:"flows"`
	Basis          string `json:"basis"`
	CallSitesTotal int    `json:"call_sites_total"`
	Truncated      struct {
		Count int `json:"count"`
	} `json:"truncated"`
	CallSites []struct {
		At         string `json:"at"`
		In         string `json:"in"`
		Kind       string `json:"kind"`
		Candidates int    `json:"candidates"`
		Callee     string `json:"callee"`
	} `json:"call_sites"`
	ByCallee []struct {
		Callee    string `json:"callee"`
		CallSites int    `json:"call_sites"`
	} `json:"by_callee"`
	Filters []string `json:"filters"`
}

func graphImpls(t *testing.T, dir string, args ...string) *implsOut {
	t.Helper()
	out := runIn(t, binary, dir, nil, append([]string{"graph"}, args...)...)
	var a implsOut
	if err := json.Unmarshal(out, &a); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if a.Schema != "rivus.graph/1" || a.Relation != "calls" || a.Flows != "not_checked" || a.Filters == nil {
		t.Errorf("the common fields: %s", out)
	}
	return &a
}

func graphDispatch(t *testing.T, dir string, args ...string) *dispatchersOut {
	t.Helper()
	out := runIn(t, binary, dir, nil, append([]string{"graph"}, args...)...)
	var a dispatchersOut
	if err := json.Unmarshal(out, &a); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if a.Schema != "rivus.graph/1" || a.Flows != "not_checked" || a.Filters == nil {
		t.Errorf("the common fields: %s", out)
	}
	return &a
}

// impls and dispatchers on a Go sample: an interface with two implementations, and the way back.
func TestGraphImplsAndDispatchersOfGoCode(t *testing.T) {
	sample := testdataGo(t)
	db := filepath.Join(t.TempDir(), "g.db")
	runIn(t, binary, sample, nil, "index", "-graph", db, "./...")

	// the interface method by its short name: the call is b.Fetch(key) in get, and VTA leaves both types
	r := graphImpls(t, sample, "impls", "-graph", db, "Backend.Fetch")
	if !strings.HasSuffix(r.Method, "dispatch.Backend).Fetch") || r.CallSites != 1 || r.ZeroCandidateSites != 0 || !strings.Contains(r.Basis, "VTA") || !strings.Contains(r.Basis, "not every type") {
		t.Fatalf("%+v", r)
	}
	var mem, disk string
	for _, i := range r.Impls {
		if strings.HasSuffix(i.ID, "dispatch.memBackend).Fetch") {
			mem = i.ID
		}
		if strings.HasSuffix(i.ID, "dispatch.diskBackend).Fetch") {
			disk = i.ID
		}
	}
	if mem == "" || disk == "" || r.ImplsTotal != len(r.Impls) || r.Truncated.Count != 0 || r.Truncated.Unit != "impls" {
		t.Errorf("impls: %+v", r.Impls)
	}
	// --limit, with the total and the makeup
	l := graphImpls(t, sample, "impls", "-graph", db, "Backend.Fetch", "--limit", "1")
	if len(l.Impls) != 1 || l.Truncated.Count != r.ImplsTotal-1 || len(l.ByPackage) == 0 {
		t.Errorf("limit: %+v", l)
	}
	// a filter
	f := graphImpls(t, sample, "impls", "-graph", db, "Backend.Fetch", "--avoid", disk)
	if len(f.Filters) != 1 || len(f.Impls) != r.ImplsTotal-1 {
		t.Errorf("avoid: %+v", f)
	}

	// back: the calls through an interface that list the implementation
	d := graphDispatch(t, sample, "dispatchers", "-graph", db, disk)
	if d.CallSitesTotal != 1 || len(d.CallSites) != 1 || d.CallSites[0].Kind != "interface" || !strings.HasSuffix(d.CallSites[0].Callee, "dispatch.Backend).Fetch") || !strings.HasSuffix(d.CallSites[0].In, "dispatch.get") || d.CallSites[0].Candidates < 2 {
		t.Errorf("dispatchers: %+v", d)
	}
	if !strings.Contains(d.Basis, "candidates") {
		t.Errorf("basis: %q", d.Basis)
	}
	if n := graphDispatch(t, sample, "dispatchers", "-graph", db, disk, "--avoid", "dispatch.get"); n.CallSitesTotal != 0 || len(n.Filters) != 1 {
		t.Errorf("avoid: %+v", n)
	}

	// an ambiguous short name, a name with no call site, an unknown one
	if a := graphImpls(t, sample, "impls", "-graph", db, "Fetch"); a.AmbiguousTotal < 2 && len(a.Impls) == 0 && a.Method == "" {
		t.Errorf("Fetch: %+v", a)
	}
	if n := graphImpls(t, sample, "impls", "-graph", db, disk); len(n.Impls) != 0 || n.CallSites != 0 || n.Reason == "" {
		t.Errorf("a function is not an interface method: %+v", n)
	}
	if msg := runFail(t, sample, "graph", "impls", "-graph", db, "No.Such.Method"); !strings.Contains(msg, "no function") {
		t.Errorf("%s", msg)
	}
}
