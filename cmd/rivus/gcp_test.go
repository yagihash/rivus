package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model/builtin"
	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
)

// gcpKnownMisses are the cases of bench/gcp that rivus does not find, with the reason: a ratchet (the test fails
// the day it is found, and the entry is to be taken out).
var gcpKnownMisses = map[string]string{
	// a propagator's flow into a variadic destination (row.Columns(&a)) does not reach the pointee of the
	// elements of the slice: the flow into an argument that is not variadic (ColumnByName, ToStruct) does.
	"ReadColumns": "the flow of a propagator into a variadic destination",
	// the address of a Statement goes to a function that is not read (it may move a parameter to the SQL), and the
	// Statement is given to the sink by value, as every Spanner call does: the root that the call leaves is not
	// followed through the copy. The same call before a sink that takes the pointer is reported with
	// field_imprecision (opaque_callee), and so is a function that takes the Statement by value and returns it
	// (URewrite below). Found with a small project outside bench/gcp: the exec_test.go runs both and the value
	// does reach the SQL, so this is a miss and not a quiet decoy.
	"UOpaque":       "an unread function that takes the address of a Statement, then the Statement is copied into the sink",
	"UOpaqueAssign": "the same, the parameter set by an assignment",
}

// gcpUncertain are the cases that are reported with a field_imprecision of a known why: how a form is reported
// today. The day the engine says more (a path in `to:` of flows, stage 5 of docs/DESIGN-field-sinks.md), the test is
// red, and the change is seen.
var gcpUncertain = map[string]string{
	"URewrite": "whole_value",
}

// TestGCPRows is the gate of the dead rows of the Google Cloud model (internal/model/builtin/go-gcp.yaml) and of
// its decoys, on the sample bench/gcp (the official modules of Cloud Spanner and BigQuery; the code builds the
// calls and does not send them). Every sink of the rule set must be hit by a finding of a vulnerable case (V*);
// no fixed case (F*) and no decoy (D*) may have a finding; no path of a row may name a field that the program
// does not have (field_unresolved). The modules are in the module cache (`go mod download` in bench/gcp);
// RIVUS_REQUIRE_GCP=1 makes their absence a failure.
func TestGCPRows(t *testing.T) {
	if testing.Short() {
		t.Skip("analyses the Google Cloud clients: about a minute and 4 GB")
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "gcp"))
	if err != nil {
		t.Fatal(err)
	}
	probe := exec.Command("go", "list", "-deps", "./...")
	probe.Dir = dir
	if out, err := probe.CombinedOutput(); err != nil {
		if os.Getenv("RIVUS_REQUIRE_GCP") == "1" {
			t.Fatalf("RIVUS_REQUIRE_GCP=1 says that the test must run: %v\n%s", err, out)
		}
		t.Skipf("the modules of bench/gcp are not in the module cache (go mod download): %v", err)
	}

	cmd := exec.Command(binary, "analyze", "-rules", "rules-stored.yaml", "./...")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}

	byCase := map[string][]*taint.Finding{}
	hit := map[string]bool{}
	for i := range rep.Findings {
		f := rep.Findings[i]
		hit[f.Sink.Symbol] = true
		name := f.Sink.Func[strings.LastIndex(f.Sink.Func, ".")+1:]
		if i := strings.Index(name, "$"); i >= 0 {
			name = name[:i]
		}
		byCase[name] = append(byCase[name], f)
	}

	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, rs := range sets {
		if rs.Name != "rivus/go/gcp@1" {
			continue
		}
		for _, s := range rs.Sinks {
			rows++
			if !hit[s.Symbol] {
				t.Errorf("%s: no vulnerable case of bench/gcp reaches this row (a dead row: the key does not name the symbol that the IR has, or a case is missing)", s.Symbol)
			}
		}
	}
	if rows == 0 {
		t.Fatal("rivus/go/gcp@1 has no sink")
	}

	for _, u := range rep.Uncertainties {
		if u.Kind == taint.KindFieldUnresolved {
			t.Errorf("a path of go-gcp.yaml has a name that the program does not have: %s", u.Detail)
		}
	}
	cases := map[string]bool{}
	for _, fn := range gcpCaseNames(t, dir) {
		cases[fn] = true
	}
	for fn := range cases {
		fs := byCase[fn]
		switch {
		case gcpKnownMisses[fn] != "":
			if len(fs) > 0 {
				t.Errorf("%s is found now: take it out of gcpKnownMisses (%s)", fn, gcpKnownMisses[fn])
			}
		case gcpUncertain[fn] != "":
			why := gcpUncertain[fn]
			found := false
			for _, f := range fs {
				for _, u := range f.Uncertainties {
					if u.Kind == taint.KindFieldImprecision && strings.Contains(u.Detail, "why: "+why) {
						found = true
					}
				}
			}
			if !found {
				t.Errorf("%s: want a finding with field_imprecision (why: %s), got %d findings", fn, why, len(fs))
			}
		case strings.HasPrefix(fn, "V") || strings.HasPrefix(fn, "Read"):
			if len(fs) == 0 {
				t.Errorf("%s: no finding (a vulnerable case must be reported)", fn)
			}
			for _, f := range fs {
				for _, u := range f.Uncertainties {
					if u.Kind == taint.KindFieldImprecision || u.Kind == taint.KindFieldUnresolved {
						t.Errorf("%s: the field is not told apart: %s %s", fn, u.Kind, u.Detail)
					}
				}
			}
		default: // F* (fixed) and D* (decoy)
			for _, f := range fs {
				t.Errorf("%s: a finding on a case that must be quiet: %s at %s:%d", fn, f.Sink.Symbol, f.Sink.Position.File, f.Sink.Position.Line)
			}
		}
	}
	for fn := range gcpKnownMisses {
		if !cases[fn] {
			t.Errorf("gcpKnownMisses names %s, which is not a case", fn)
		}
	}

	// BigQuery: Client.Query(q) and Run / Read are both sinks; one flow is reported at both.
	dup := 0
	for _, fn := range []string{"VRun", "VRead"} {
		if len(byCase[fn]) == 2 {
			dup++
		}
	}
	t.Logf("BigQuery: %d of 2 cases that build the query with Client.Query have the flow reported twice (Client.Query and Run/Read); VField (q.Q = x) once", dup)
	if len(byCase["VField"]) != 1 {
		t.Errorf("VField (q.Q = x, then Run): %d findings, want 1 (the Q of the Run)", len(byCase["VField"]))
	}
}

// gcpCaseNames are the handlers of bench/gcp: the methods of the Server types.
func gcpCaseNames(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	for _, pkg := range []string{"spanner", "bigquery"} {
		src, err := os.ReadFile(filepath.Join(dir, pkg, pkg+".go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(src), "\n") {
			const p = "func (s *Server) "
			if strings.HasPrefix(line, p) {
				names = append(names, line[len(p):strings.Index(line, "(w ")])
			}
		}
	}
	return names
}
