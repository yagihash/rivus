package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yagihash/rivus/internal/report"
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
	if len(entries) != 6 {
		t.Errorf("want findings from the 6 commands, got from %d: %v", len(entries), entries)
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
	if s.DepthLimitShare > 0.05 {
		t.Errorf("depth_limit is %.1f%% of the uncertainties: the limits are too tight for rivus' own code", 100*s.DepthLimitShare)
	}
}
