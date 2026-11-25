package main

import (
	"os/exec"
	"strings"
	"testing"
)

// flagCommands are the commands that have flags (the list that the tests of the flags and of -h go through).
var flagCommands = [][]string{
	{"analyze"}, {"index"}, {"query"}, {"resolve"}, {"sites"}, {"models", "coverage"}, {"models", "versions"}, {"setup", "rust"}, {"setup", "dart"}, {"cache", "clean"}, {"debug", "extract"},
	{"graph", "info"}, {"graph", "callers"}, {"graph", "callees"}, {"graph", "path"}, {"graph", "impls"}, {"graph", "dispatchers"},
}

// -h of every command ends with 0 and prints no error; a command that needs a command under it ends with 2 when it is given none.
func TestHelpExitsZeroEverywhere(t *testing.T) {
	t.Parallel()
	cmds := append([][]string{}, flagCommands...)
	cmds = append(cmds, []string{"models"}, []string{"setup"}, []string{"graph"}, []string{"debug"})
	for _, c := range cmds {
		for _, extra := range [][]string{{"-h"}, {"-h", "-help-all"}} {
			var stderr strings.Builder
			cmd := exec.Command(binary, append(append([]string{}, c...), extra...)...)
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil || strings.Contains(stderr.String(), "help requested") {
				t.Errorf("rivus %s %s: %v, stderr %q", strings.Join(c, " "), strings.Join(extra, " "), err, stderr.String())
			}
		}
	}
	for _, c := range [][]string{{"models"}, {"setup"}, {"cache"}, {"graph"}, {"debug"}} {
		out, err := exec.Command(binary, c...).CombinedOutput()
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 2 || !strings.Contains(string(out), "Usage") {
			t.Errorf("rivus %s: %v, %s", c[0], err, out)
		}
	}
}

// Every flag of every command has exactly one side, usual or diagnostic (flagsides.go). The check runs when a command prepares its flags, so
// "-h" of each command reaches it: a flag added without a side makes that command stop with the names, and this test fails.
func TestEveryFlagHasOneSide(t *testing.T) {
	t.Parallel()
	for _, c := range flagCommands {
		name := strings.Join(c, " ")
		out, _ := exec.Command(binary, append(append([]string{}, c...), "-h")...).CombinedOutput()
		if strings.Contains(string(out), "without exactly one side") || strings.Contains(string(out), "panic") {
			t.Errorf("rivus %s -h: %s", name, out)
			continue
		}
		all, _ := exec.Command(binary, append(append([]string{}, c...), "-help-all")...).CombinedOutput()
		if strings.Contains(string(all), "Flags for diagnosis and tuning:") == !strings.Contains(string(out), "-help-all") {
			t.Errorf("rivus %s: -h and -help-all do not agree on whether there are diagnostic flags:\n%s\n%s", name, out, all)
		}
	}
}

func TestHelpHidesDiagnosticFlagsUntilHelpAll(t *testing.T) {
	t.Parallel()
	out, _ := exec.Command(binary, "analyze", "-h").CombinedOutput()
	for _, usual := range []string{"-lang", "-no-deps", "-workers", "-detail"} {
		if !strings.Contains(string(out), "\n  "+usual+" ") && !strings.Contains(string(out), "\n  "+usual+"\n") {
			t.Errorf("analyze -h does not list %s", usual)
		}
	}
	for _, diag := range []string{"-cpuprofile", "-pass-stats", "-max-passes", "-k ", "-func-timeout"} {
		if strings.Contains(string(out), "\n  "+diag) {
			t.Errorf("analyze -h lists the diagnostic flag %s", diag)
		}
	}
	if !strings.Contains(string(out), "-help-all") {
		t.Errorf("analyze -h does not say where the other flags are:\n%s", out)
	}
	all, _ := exec.Command(binary, "analyze", "-h", "-help-all").CombinedOutput()
	for _, diag := range []string{"-cpuprofile", "-pass-stats", "-max-passes", "-k int", "-func-timeout"} {
		if !strings.Contains(string(all), "\n  "+diag) {
			t.Errorf("analyze -h -help-all does not list %s", diag)
		}
	}
}

// extract moved under debug: the old name is not an alias; it says the new name and stops with 2.
func TestExtractMovedToDebug(t *testing.T) {
	t.Parallel()
	out, err := exec.Command(binary, "extract", "-text").CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 || !strings.Contains(string(out), "rivus debug extract") {
		t.Errorf("rivus extract: %v, %s", err, out)
	}
	for _, args := range [][]string{{"debug"}, {"debug", "-h"}} {
		out, _ := exec.Command(binary, args...).CombinedOutput()
		if !strings.Contains(string(out), "extract") {
			t.Errorf("rivus %v does not list extract: %s", args, out)
		}
	}
	if help, _ := exec.Command(binary, "help").CombinedOutput(); strings.Contains(string(help), "  extract ") || !strings.Contains(string(help), "  debug ") {
		t.Errorf("rivus help: %s", help)
	}
}
