//go:build darwin || linux

package sandbox

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// supervise on its own (no sandbox): the way it waits and says why a program was stopped.

type superviseOut struct {
	res *Result
	err error
}

// runSupervised runs sh -c script, and fails when it does not come back in limit (a goroutine: a hang is a failure, not a stuck test).
func runSupervised(t *testing.T, ctx context.Context, s Spec, script string, limit time.Duration) (*Result, time.Duration) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", script)
	ch := make(chan superviseOut, 1)
	start := time.Now()
	go func() {
		r, err := supervise(ctx, s, cmd, nil)
		ch <- superviseOut{r, err}
	}()
	select {
	case o := <-ch:
		if o.err != nil {
			t.Fatalf("supervise: %v", o.err)
		}
		return o.res, time.Since(start)
	case <-time.After(limit):
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // (the group of this test only)
		}
		t.Fatalf("supervise did not return in %v", limit)
		return nil, 0
	}
}

// A process that leaves the group and keeps the output open does not hold the run after the program has ended.
func TestSuperviseDoesNotWaitForAProcessThatHoldsTheOutput(t *testing.T) {
	// the child makes a session (and a group) of its own and sleeps with the output open; the parent says its pid and ends
	script := `/usr/bin/perl -e 'use POSIX; my $p = fork(); if ($p == 0) { POSIX::setsid(); exec("/bin/sleep", "30"); } print "$p\n";'`
	res, d := runSupervised(t, context.Background(), Spec{Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}}, script, 15*time.Second)
	// the group of the sleep is the one that this test made: it is stopped here, and nothing else
	if pid, err := strconv.Atoi(strings.TrimSpace(string(res.Stdout))); err == nil && pid > 1 {
		syscall.Kill(-pid, syscall.SIGKILL)
	}
	if d > 8*time.Second {
		t.Errorf("returned after %v", d)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit %d", res.ExitCode)
	}
	if len(res.Warnings) == 0 {
		t.Errorf("the output that was closed is not said")
	}
}

func TestSuperviseSaysCancelledAndTimedOutApart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(500 * time.Millisecond); cancel() }()
	res, _ := runSupervised(t, ctx, Spec{Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Timeout: time.Minute}, "/bin/sleep 20", 10*time.Second)
	if res.TimedOut || !res.Cancelled {
		t.Errorf("cancel: TimedOut=%v Cancelled=%v", res.TimedOut, res.Cancelled)
	}
	res, _ = runSupervised(t, context.Background(), Spec{Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Timeout: 500 * time.Millisecond}, "/bin/sleep 20", 10*time.Second)
	if !res.TimedOut || res.Cancelled {
		t.Errorf("deadline: TimedOut=%v Cancelled=%v", res.TimedOut, res.Cancelled)
	}
}

// Output over the limit is said, also when the program ends at once after it (the program ends and the output is over at the same time:
// both were ready, and select took either of them).
func TestSuperviseSaysOutputTooBigWhenTheProgramEndsAtOnce(t *testing.T) {
	for i := 0; i < 60; i++ {
		res, _ := runSupervised(t, context.Background(), Spec{Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, MaxOutput: 10}, "/usr/bin/head -c 100000 /dev/zero", 10*time.Second)
		if !res.OutputTooBig {
			t.Fatalf("run %d: 100000 bytes were written with a limit of 10, and OutputTooBig is false (exit %d): cut output looks like a whole one", i, res.ExitCode)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// A Stdout that fails (the file of a spill that is full) is an error of the run: the program is stopped (it gets SIGPIPE, or is killed), and
// the error is returned. Before, it was lost: the program ended by a signal, the caller got no error, and the output that was cut off looked
// like a whole one.
func TestSuperviseReturnsTheErrorWhenTheOutputCannotBeWritten(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "/usr/bin/head -c 1000000 /dev/zero")
	ch := make(chan superviseOut, 1)
	go func() {
		r, err := supervise(context.Background(), Spec{Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Stdout: failingWriter{}}, cmd, nil)
		ch <- superviseOut{r, err}
	}()
	select {
	case o := <-ch:
		if o.err == nil || !strings.Contains(o.err.Error(), "disk full") {
			t.Errorf("the write error is not returned: %v", o.err)
		}
	case <-time.After(10 * time.Second):
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // (the group of this test only)
		}
		t.Fatal("the program is blocked on the output that nobody reads, and the run does not end")
	}
}
