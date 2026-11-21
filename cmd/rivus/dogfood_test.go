package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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
	if len(entries) != 8 {
		t.Errorf("want findings from the 8 commands that reach a sink, got from %d: %v", len(entries), entries)
	}
	// Before ADR-048 (8 sources x 11 sinks = 88 findings) 77 paths did not go
	// through their own source function: the same object was written back by one
	// source and read by the others (CORE-A / CORE-B, see
	// TestParamSourceSharedObject). Now every path has a step in its source
	// function. This is a ceiling: it must go down, never up.
	const maxNoSourceOnPath = 0
	t.Logf("%d findings, %d with no step in the source function", len(rep.Findings), noSourceOnPath)
	if noSourceOnPath > maxNoSourceOnPath {
		t.Errorf("%d findings have a path without a step in their source function: more than the %d of today", noSourceOnPath, maxNoSourceOnPath)
	}
	checkDogfoodFlows(t, &rep)
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
	const wantCaptureUnbound = 37 // the closures of the code of C-7 itself (the same set with and without it); 37: sg.ExactClosures has two (add, walk) where ExactFuncs had one
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

// updateDogfoodFlows (go test ./cmd/rivus -run 'TestDogfood$' -update) rewrites testdata/dogfood-flows.txt from the findings of
// today. Without the flag the test is read-only.
var updateDogfoodFlows = flag.Bool("update", false, "rewrite cmd/rivus/testdata/dogfood-flows.txt from the findings of this run")

const dogfoodFlowsFile = "testdata/dogfood-flows.txt"

// checkDogfoodFlows holds the findings of rivus on itself as a list of (the function of the source, the function that has the
// sink, the sink), not as a count: a count hides a swap (a real flow that is lost while another one appears keeps the same
// number). A flow that is added or lost is red. A flow is listed once per finding, so a finding that is added to a pair that
// is already there is red too.
//
// How to change the list:
//   - go test ./cmd/rivus -run 'TestDogfood$' -update rewrites the file. The flows that are new get the comment
//     "# TODO(reason)" at the end of their line, and the test stays red until it is replaced.
//   - Say WHY the flow is real (or why it is the price of the code that was added), in that comment: which path of the
//     project (the directory of -C, a flag) reaches which operation, and why that is what the code is for. If it is
//     not real (the analysis follows a value that does not flow), it is a bug of rivus: fix that, do not list it.
//   - A flow that is lost is taken off the list by -update. Say in the commit why it is gone (a fix, or a flow that was
//     lost by mistake: look at it, a lost flow is a lost detection).
//   - Never put the flows back in by hand to make the test green. The comments at the head of the file say why the groups exist.
func checkDogfoodFlows(t *testing.T, rep *report.Report) {
	t.Helper()
	const prefix = "github.com/yagihash/rivus/"
	var got []string
	for _, f := range rep.Findings {
		got = append(got, strings.ReplaceAll(f.Source.Func+" -> "+f.Sink.Func+" ["+f.Sink.Symbol+"]", prefix, ""))
	}
	sort.Strings(got)

	old, err := os.ReadFile(dogfoodFlowsFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var head []string // the comment lines before the first flow
	comments := map[string][]string{}
	var want []string
	for _, l := range strings.Split(string(old), "\n") {
		flow, comment, _ := strings.Cut(l, "  # ")
		switch {
		case strings.TrimSpace(l) == "":
		case strings.HasPrefix(l, "#"):
			if len(want) == 0 {
				head = append(head, l)
			}
		default:
			flow = strings.TrimSpace(flow)
			want = append(want, flow)
			comments[flow] = append(comments[flow], comment)
		}
	}

	if *updateDogfoodFlows {
		var b strings.Builder
		for _, l := range head {
			b.WriteString(l + "\n")
		}
		b.WriteString("\n")
		used := map[string]int{}
		for _, g := range got {
			c := "TODO(reason)"
			if i := used[g]; i < len(comments[g]) {
				c = comments[g][i]
			}
			used[g]++
			b.WriteString(g)
			if c != "" {
				b.WriteString("  # " + c)
			}
			b.WriteString("\n")
		}
		if err := os.WriteFile(dogfoodFlowsFile, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s rewritten (%d flows)", dogfoodFlowsFile, len(got))
	}

	count := func(xs []string) map[string]int {
		m := map[string]int{}
		for _, x := range xs {
			m[x]++
		}
		return m
	}
	g, w := count(got), count(want)
	if *updateDogfoodFlows {
		w = g
	}
	for k, n := range g {
		if n > w[k] {
			t.Errorf("a flow was added (x%d, was x%d): %s\n\tif it is real, run with -update and write the reason on its line in %s", n, w[k], k, dogfoodFlowsFile)
		}
	}
	for k, n := range w {
		if n > g[k] {
			t.Errorf("a flow was lost (x%d, was x%d): %s\n\ta lost flow can be a lost detection: look at it; if it is gone for a reason, run with -update and say why in the commit", g[k], n, k)
		}
	}
	for flow, cs := range comments {
		for _, c := range cs {
			if strings.Contains(c, "TODO(reason)") {
				t.Errorf("the flow has no reason yet (%s): replace TODO(reason) in %s", flow, dogfoodFlowsFile)
			}
		}
	}
}
