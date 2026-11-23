package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/testutil"
)

// notFailed is err, unless it is the exit code of an analysis that is not complete (exitIncomplete): the output is written in
// full then, and a test that looks at the output (and not at the exit code) goes on with it.
func notFailed(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == exitIncomplete {
		return nil
	}
	return err
}

// notFailedIf is notFailed when allowed, and err as it is when not: a run that must converge (and is not one of the
// zoo's chains that stop by the pass limit on purpose) fails the test on exit code 4, as it does on any other code.
func notFailedIf(err error, allowed bool) error {
	if allowed {
		return notFailed(err)
	}
	return err
}

// analyzeExit runs "rivus analyze" on the scc test module and gives the exit code and the summary of what it wrote.
func analyzeExit(t *testing.T, args ...string) (int, report.Summary) {
	t.Helper()
	cmd := exec.Command(binary, append([]string{"analyze", "-rules", "rules.yaml", "-detail", "summary"}, append(args, "./...")...)...)
	cmd.Dir = filepath.Join(testutil.Testdata(), "..", "scc")
	var out bytes.Buffer
	cmd.Stdout = &out
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	var r struct {
		Summary report.Summary `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("the output is written in full before the exit (exit code %d): %v", code, err)
	}
	return code, r.Summary
}

// A run that is complete ends with 0, and says so.
func TestExitCodeOfACompleteRun(t *testing.T) {
	t.Parallel()
	code, s := analyzeExit(t)
	if code != 0 || s.Incomplete || len(s.IncompleteReasons) != 0 {
		t.Errorf("exit code %d, incomplete %v %v: a complete run is 0", code, s.Incomplete, s.IncompleteReasons)
	}
}

// The time limit of the whole run: the result is written, summary.incomplete says why, and the exit code is exitIncomplete.
func TestExitCodeOfATimedOutRun(t *testing.T) {
	t.Parallel()
	code, s := analyzeExit(t, "-timeout", "1ns")
	if code != exitIncomplete || !s.Incomplete || !reflect.DeepEqual(s.IncompleteReasons, []string{report.ReasonTimeout}) {
		t.Errorf("exit code %d, incomplete %v %v", code, s.Incomplete, s.IncompleteReasons)
	}
}

// The pass limit: the summaries did not settle.
func TestExitCodeOfARunThatDidNotConverge(t *testing.T) {
	t.Parallel()
	code, s := analyzeExit(t, "-max-passes", "1")
	if code != exitIncomplete || !s.Incomplete || !reflect.DeepEqual(s.IncompleteReasons, []string{report.ReasonNotConverged}) {
		t.Errorf("exit code %d, incomplete %v %v", code, s.Incomplete, s.IncompleteReasons)
	}
}

func TestExitCodeOfIncompleteError(t *testing.T) {
	t.Parallel()
	if got := exitCode(incomplete(&report.Report{Summary: report.Summary{Incomplete: true, IncompleteReasons: []string{"timeout"}}})); got != exitIncomplete {
		t.Errorf("exit code %d", got)
	}
	if incomplete(&report.Report{}) != nil || incomplete(nil) != nil {
		t.Error("a complete report is not an error")
	}
	if exitIncomplete == exitDeclined || exitIncomplete == 130 || exitIncomplete <= 2 {
		t.Errorf("exit code %d is one that is in use", exitIncomplete)
	}
}

// A function that was cut by its own limit makes the run incomplete even when the run converged: the summary of that
// function may miss flows (ADR-120, the section of the cut of a function). The two reasons are told apart.
func TestIncompleteReasonsOfFunctionLimits(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name            string
		timeouts, spent int
		want            []string
	}{
		{"none", 0, 0, []string{}},
		{"clock", 3, 0, []string{report.ReasonFuncTimeout}},
		{"work", 0, 2, []string{report.ReasonBudget}},
		{"both", 3, 2, []string{report.ReasonFuncTimeout, report.ReasonBudget}},
	} {
		st := &report.Report{}
		st.Stats.Converged, st.Stats.FuncTimeouts, st.Stats.BudgetExhausted = true, c.timeouts, c.spent
		if got := report.IncompleteReasons(&st.Stats); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: reasons %v, want %v", c.name, got, c.want)
		}
	}
}

// The time limit of one function: the run converges, the output is written, the exit code is exitIncomplete, and the reason
// is func_timeout (not timeout: the run itself did not run out of time).
func TestExitCodeOfAFunctionTimeout(t *testing.T) {
	t.Parallel()
	code, s := analyzeExit(t, "-func-timeout", "1ns")
	if code != exitIncomplete || !s.Incomplete || !reflect.DeepEqual(s.IncompleteReasons, []string{report.ReasonFuncTimeout}) {
		t.Errorf("exit code %d, incomplete %v %v", code, s.Incomplete, s.IncompleteReasons)
	}
}

// stats.func_timeouts and stats.budget_exhausted are always in the output, 0 too: a reader (and the ledgers of bench/) tells
// "none" from "not written".
func TestStatsAlwaysHaveTheFunctionLimitCounts(t *testing.T) {
	t.Parallel()
	cmd := exec.Command(binary, "analyze", "-rules", "rules.yaml", "-detail", "summary", "./...")
	cmd.Dir = filepath.Join(testutil.Testdata(), "..", "scc")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Stats map[string]any `json:"stats"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"func_timeouts", "budget_exhausted"} {
		if v, ok := r.Stats[k]; !ok || v != float64(0) {
			t.Errorf("stats.%s = %v (present %v), want 0 and present", k, v, ok)
		}
	}
}
