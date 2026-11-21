package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
)

// The zoo cell clos/partial-callers: a helper calls the function it is given; one caller passes a closure
// made right there, another a function it reads from a field (a value the analysis cannot follow). The
// functions that were found stay the candidates (what is found now is not lost), and the call says that
// they may not be all: an unresolved_callee whose detail starts with "partial:". The uncertainty is in the
// helper, which is not a function of the lost flow's source or sink, so the @miss of the cell cannot name
// it as its trace; this test holds it.
func TestZooPartialCallersSaysItIsPartialDart(t *testing.T) {
	root := zooDartRoot(t)
	if os.Getenv("RIVUS_DART") == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	cellsDir := filepath.Join(root, "cells")
	irFile := filepath.Join(t.TempDir(), "zoo.jsonl")
	if err := os.WriteFile(irFile, zooDartStream(t, cellsDir), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "analyze", "-sites", "-ir", irFile, "-rules", filepath.Join(cellsDir, "rules.yaml"))
	cmd.Dir = cellsDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	partial := map[string]string{}
	for _, u := range rep.Uncertainties {
		if u.Kind == taint.KindUnresolvedCallee && strings.HasPrefix(u.Detail, "partial:") && strings.Contains(u.Position.File, "partial_callers") {
			partial[u.Func] = u.Detail
		}
	}
	for _, helper := range []string{"applyVuln", "applyFixed", "applyDecoy"} {
		found := false
		for fn, d := range partial {
			if strings.HasSuffix(fn, helper) {
				found = true
				if !strings.Contains(d, "untraced_value") {
					t.Errorf("%s: the cause is not said: %s", helper, d)
				}
			}
		}
		if !found {
			t.Errorf("%s: no partial uncertainty (have %v)", helper, partial)
		}
	}
}
