//go:build darwin || linux

package sandbox

import (
	"context"
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
