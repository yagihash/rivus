//go:build darwin

package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"
)

// The limits that the container had, under the sandbox: the time, the size of the output, the environment, and the group that is stopped.
// (They touch nothing of the user: no environment variable is read, nothing is made outside the run.)

func TestSandboxTimeLimitStopsTheProgramAndItsGroup(t *testing.T) {
	start := time.Now()
	res, err := Run(context.Background(), Spec{Argv: []string{"/bin/sh", "-c", "/bin/sleep 60 & /bin/sleep 60"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"},
		Exec: []string{"/bin", "/usr/bin"}, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.TimedOut {
		t.Errorf("the run was not stopped by its time limit")
	}
	if d := time.Since(start); d > 20*time.Second {
		t.Errorf("the run took %v for a limit of 2s", d)
	}
}

func TestSandboxOutputLimitStopsTheProgram(t *testing.T) {
	res, err := Run(context.Background(), Spec{Argv: []string{"/usr/bin/yes"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"},
		Exec: []string{"/usr/bin"}, Timeout: time.Minute, MaxOutput: 1 << 20})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.OutputTooBig || res.TimedOut {
		t.Errorf("OutputTooBig=%v TimedOut=%v: the program that writes without end must be stopped by the output limit", res.OutputTooBig, res.TimedOut)
	}
	if len(res.Stdout) > 1<<20 {
		t.Errorf("%d bytes kept for a limit of 1 MiB", len(res.Stdout))
	}
}

func TestSandboxEnvironmentHoldsOnlyWhatIsGiven(t *testing.T) {
	t.Setenv("RIVUS_TEST_SECRET_TOKEN", "must-not-be-seen")
	res, err := Run(context.Background(), Spec{Argv: []string{"/usr/bin/env"}, Dir: "/", Env: []string{"ONLY=this"},
		Exec: []string{"/usr/bin"}, Timeout: time.Minute})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := string(res.Stdout)
	if strings.Contains(out, "must-not-be-seen") || strings.Contains(out, "RIVUS_TEST_SECRET_TOKEN") {
		t.Errorf("the environment of the caller reached the program:\n%s", out)
	}
	if !strings.Contains(out, "ONLY=this") {
		t.Errorf("what was given is not there:\n%s", out)
	}
}

func TestSandboxUnavailableStopsWithoutRunning(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	if _, err := Run(context.Background(), Spec{Argv: []string{"/usr/bin/true"}}); err != ErrUnavailable {
		t.Errorf("without sandbox-exec: %v, want ErrUnavailable (nothing is run)", err)
	}
}
