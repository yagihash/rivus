//go:build unix

package main

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// A signal that ends rivus must stop what it started: the program under the sandbox is in a session of its own (Ctrl-C does not reach it) and
// its time limit is kept by rivus, so when rivus ended at once the build went on (a build script that loops, writing). cancelOn cancels the
// context of the run when a signal arrives on the channel. (No real signal is sent: the channel is the one that signal.Notify would fill.)
func TestCancelOnSignal(t *testing.T) {
	sig := make(chan os.Signal, 1)
	ctx, stop := cancelOn(context.Background(), sig)
	defer stop()
	select {
	case <-ctx.Done():
		t.Fatal("cancelled before a signal")
	default:
	}
	sig <- syscall.SIGTERM
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("a signal did not cancel the context")
	}
	if ctx.Err() != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", ctx.Err())
	}
}

// The parent's end ends it too, and stop releases the goroutine without a signal.
func TestCancelOnParentAndStop(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	ctx, stop := cancelOn(parent, make(chan os.Signal, 1))
	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the end of the parent did not end it")
	}
	stop()
	ctx2, stop2 := cancelOn(context.Background(), make(chan os.Signal, 1))
	stop2()
	if ctx2.Err() == nil {
		t.Error("stop does not end the context")
	}
}

// What the cancelled context does to the program: a program run with it (a stand-in for the build: `sleep`, in a group of its own, as
// the sandbox starts it) is killed with its group when the context is cancelled, and nothing outside the group is signalled.
func TestCancelledContextStopsTheGroup(t *testing.T) {
	sig := make(chan os.Signal, 1)
	ctx, stop := cancelOn(context.Background(), sig)
	defer stop()
	cmd := exec.CommandContext(ctx, "sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) } // the group that this run started, only
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	sig <- syscall.SIGINT
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		syscall.Kill(-pid, syscall.SIGKILL)
		t.Fatal("the program was not stopped by the signal")
	}
}
