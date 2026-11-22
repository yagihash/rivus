//go:build darwin

package sandbox

import (
	"context"
	"os"
	"path/filepath"
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

// A program that is run under the sandbox writes what it likes to its standard error: it must not be able to make rivus say that the sandbox
// cannot be made (which would send the user to run the code without it). Whether the sandbox can be made is found out by rivus before it
// runs anything (Available), and what a program writes is only what it wrote.
func TestSandboxStderrOfTheProgramIsNotAnAnswer(t *testing.T) {
	res, err := Run(context.Background(), Spec{Argv: []string{"/bin/sh", "-c", "echo 'sandbox_apply: Operation not permitted' >&2; exit 1"}, Dir: "/",
		Env: []string{"PATH=/usr/bin:/bin"}, Exec: []string{"/bin", "/usr/bin"}, Timeout: time.Minute})
	if err != nil {
		t.Fatalf("Run: %v (a program that says sandbox_apply and exits 1 is a program that failed, not a sandbox that cannot be made)", err)
	}
	if res.ExitCode != 1 {
		t.Errorf("exit %d, want the 1 that the program gave", res.ExitCode)
	}
	if err := Available(); err != nil {
		t.Errorf("Available: %v", err)
	}
}

// The directory of the program is not allowed with it: a program that is installed with brew or cargo is among the others of its directory.
func TestSandboxProgramIsAllowedAsAFileNotAsItsDirectory(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	for _, p := range []string{a, b} {
		os.WriteFile(p, []byte("#!/bin/sh\necho ran\n"), 0o755)
	}
	spec := Spec{Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, ExecFiles: []string{"/bin/sh", "/bin/bash"}, Timeout: time.Minute}
	spec.Argv = []string{"/bin/sh", "-c", a + "; " + b}
	spec.ExecFiles = append(spec.ExecFiles, a)
	spec.Read = []string{dir}
	res, err := Run(context.Background(), spec)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Count(string(res.Stdout), "ran") != 1 {
		t.Errorf("the program that was allowed ran, and the one beside it must not have:\n%s%s", res.Stdout, res.Stderr)
	}
}
