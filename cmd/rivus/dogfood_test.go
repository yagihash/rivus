package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
)

// TestDogfood analyses rivus with rivus (docs: ADR-037). What the command
// line says reaches the file operations of rivus: that is what a CLI does,
// and it is what the analysis must find in its own code.
func TestDogfood(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")

	cmd := exec.Command(binary, "analyze", "-rules", "dogfood/rules.yaml", "-timeout", "120s", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("rivus analyze: %v\n%s", err, stderr)
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}

	if !rep.Stats.Converged || rep.Stats.TimedOut {
		t.Fatalf("the analysis of rivus did not finish: %+v", rep.Stats)
	}
	// Every command is an entry point for taint, and each reaches file
	// operations: reading rules and bookmarks, writing indexes and profiles.
	entries := map[string]bool{}
	sinks := map[string]bool{}
	var humanSource, flagModel bool
	// A finding whose path has no step in the function of its source is counted by
	// offSourcePath (see there, and the ceiling below).
	noSourceOnPath := len(offSourcePath(&rep))
	for _, f := range rep.Findings {
		entries[f.Source.Func] = true
		sinks[f.Sink.Symbol] = true
		for _, m := range f.Models {
			if m.Origin == "human" && m.Role == "source" {
				humanSource = true
			}
			if m.Origin == "builtin" && m.Symbol == "(*flag.FlagSet).Parse" {
				flagModel = true
			}
			// The pure model for File.Name removes the flow that made the
			// name of a file look like it came from the command line.
			if m.Symbol == "(*go/token.File).Name" {
				t.Errorf("a finding still goes through File.Name: %+v", f.Sink)
			}
		}
	}
	// runExtract is declared as a source too, and no sink of the rules is reachable from
	// it: it had 11 findings while a param source was written back to the caller
	// (ADR-048), and none of them had a step in runExtract.
	if len(entries) != 7 {
		t.Errorf("want findings from the 7 commands that reach a sink, got from %d: %v", len(entries), entries)
	}
	// Before ADR-048 (8 sources x 11 sinks = 88 findings) 77 paths did not go
	// through their own source function: the same object was written back by one
	// source and read by the others (CORE-A / CORE-B, see
	// TestParamSourceSharedObject). Now every path has a step in its source
	// function. These are ceilings: they must go down, never up.
	const maxFindings, maxNoSourceOnPath = 39, 0
	t.Logf("%d findings, %d with no step in the source function", len(rep.Findings), noSourceOnPath)
	if len(rep.Findings) > maxFindings {
		t.Errorf("%d findings: more than the %d of today", len(rep.Findings), maxFindings)
	}
	if noSourceOnPath > maxNoSourceOnPath {
		t.Errorf("%d findings have a path without a step in their source function: more than the %d of today", noSourceOnPath, maxNoSourceOnPath)
	}
	for _, s := range []string{"os.ReadFile", "os.Create", "os.WriteFile"} {
		if !sinks[s] {
			t.Errorf("no finding reaches %s: %v", s, sinks)
		}
	}
	if !humanSource {
		t.Error("the entry points declared in the dogfood rules should be on the paths, with their origin")
	}
	if !flagModel {
		t.Error("the command line reaches the commands through the flag set: its built-in model should be on the paths")
	}

	// The numbers to watch (ADR-008): what rivus cannot improve itself and
	// what is its own limit.
	s := rep.Summary
	t.Logf("%d findings, %d uncertainties: %v; rivus share %.2f, depth_limit share %.3f",
		s.Findings, s.Program.Total, s.Program.ByAttribution, s.RivusShare, s.DepthLimitShare)
	// depth_limit is held in two parts (ADR-053, "the split of the ceiling"): capture_unbound
	// (a closure that captures a value and is called by a helper, whose summary does not carry
	// it: a limit of rivus that was silent) has a ceiling of its own, a count; every other
	// depth_limit (path_limit, budget, guard_limit, ...) stays under 5% of the uncertainties.
	// Both are ceilings: a count that goes down fails too, so that the ceiling is brought down
	// (C-7 is meant to make capture_unbound go down).
	capture := 0
	for _, u := range rep.Uncertainties {
		if u.Kind == taint.KindDepthLimit && strings.HasPrefix(u.Detail, "capture_unbound") {
			capture++
		}
	}
	const wantCaptureUnbound = 35
	t.Logf("capture_unbound: %d (ceiling %d)", capture, wantCaptureUnbound)
	if capture != wantCaptureUnbound {
		t.Errorf("capture_unbound is %d, the ceiling is %d: if it went down, lower the ceiling (that is the point of C-7); if it went up, a closure capture is lost silently somewhere new", capture, wantCaptureUnbound)
	}
	if s.Program.Total > 0 {
		other := float64(s.Program.ByKind[taint.KindDepthLimit]-capture) / float64(s.Program.Total)
		if other > 0.05 {
			t.Errorf("depth_limit other than capture_unbound is %.1f%% of the uncertainties: the limits are too tight for rivus' own code", 100*other)
		}
	}
}
