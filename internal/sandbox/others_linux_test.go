//go:build linux

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The processes of the same user are not touched: a limit, the priority, the cpus of another process are not for the program (Landlock mediates the signals and nothing
// else of a process). It may do them to itself. The same probe, outside of the sandbox, does change the other process (the test shows something).

func victimProcess(t *testing.T) *exec.Cmd {
	t.Helper()
	c := exec.Command("sleep", "60")
	if err := c.Start(); err != nil {
		t.Skipf("no sleep: %v", err)
	}
	t.Cleanup(func() { c.Process.Kill(); c.Wait() })
	return c
}

func openFilesLimit(t *testing.T, pid int) string {
	t.Helper()
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/limits")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "Max open files") {
			return strings.Join(strings.Fields(l), " ")
		}
	}
	t.Fatal("no limit of open files")
	return ""
}

func niceOf(t *testing.T, pid int) string {
	t.Helper()
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	f := strings.Fields(s[strings.LastIndex(s, ")")+2:]) // after the name of the process: the field 3 is f[0], the nice (the field 19) is f[16]
	return f[16]
}

func TestSandboxOtherProcessesOfTheUserAreNotChanged(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	// without the sandbox the probe does change the other process
	v1 := victimProcess(t)
	before := openFilesLimit(t, v1.Process.Pid)
	if out, err := exec.Command(f.probe, "-only", "none", "-victim", strconv.Itoa(v1.Process.Pid)).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if after := openFilesLimit(t, v1.Process.Pid); after == before || niceOf(t, v1.Process.Pid) != "19" {
		t.Fatalf("on the host the probe does not change the other process (the test does not test what it says): %q -> %q, nice %s", before, after, niceOf(t, v1.Process.Pid))
	}
	// in the sandbox it does not
	v2 := victimProcess(t)
	before = openFilesLimit(t, v2.Process.Pid)
	res, err := Run(context.Background(), Spec{Argv: []string{f.probe, "-only", "none", "-victim", strconv.Itoa(v2.Process.Pid), "-self-limits"}, Dir: f.work,
		Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	out := string(res.Stdout)
	for _, want := range []string{"victim-prlimit: operation not permitted", "victim-setpriority: operation not permitted", "victim-setpriority-user: operation not permitted"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is not in:\n%s", want, out)
		}
	}
	if after := openFilesLimit(t, v2.Process.Pid); after != before {
		t.Errorf("the limit of the other process was changed: %q -> %q", before, after)
	}
	if got := niceOf(t, v2.Process.Pid); got != "0" {
		t.Errorf("the priority of the other process was changed: nice %s", got)
	}
	// what it does to itself still works (a build that runs with a limit of its own, nice, taskset)
	for _, want := range []string{"self-setrlimit: ok", "self-setpriority: ok", "self-done"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is not in:\n%s", want, out)
		}
	}
	// (the number of its own process is refused too: only 0 is "itself", and the filter cannot know the number of the process that asks)
	if !strings.Contains(out, "self-prlimit-pid: operation not permitted") {
		t.Errorf("prlimit with its own number is not refused as the others:\n%s", out)
	}
}
