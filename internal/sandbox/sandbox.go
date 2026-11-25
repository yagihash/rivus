//go:build darwin || linux

// Package sandbox runs a program under the sandbox of the system: Seatbelt (sandbox-exec) on macOS (seatbelt_darwin.go), Landlock and
// seccomp on Linux (landlock_linux.go). See docs/DESIGN-macos-sandbox.md and docs/DESIGN-linux-sandbox.md.
//
// What is not here: nothing is run on the host when the sandbox cannot be made (Run returns ErrUnavailable): there is no
// way to ask for "the sandbox if there is one".
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// ErrUnavailable says that the sandbox cannot be used on this machine (on macOS: sandbox-exec is not there, or the profile is not
// accepted; on Linux: Landlock or seccomp is not there or is turned off): nothing was run.
var ErrUnavailable = errors.New("the sandbox of this system is not available")

// Spec is a program to run and what it may touch.
type Spec struct {
	// Argv is the program and its arguments (an absolute path for the program).
	Argv []string
	// Dir is the working directory (it must be readable).
	Dir string
	// Env is the whole environment of the program: nothing is taken from the environment of the caller.
	Env []string
	// Read are the directories (subpaths) that may be read, Exec the ones that programs may be run from (they are read too),
	// Write the ones that may be written (and read).
	Read, Exec, Write []string
	// ExecFiles are programs that may be run, each by its path (a file, not a directory): the way to allow the few programs a build
	// needs without allowing everything in /usr/bin (osascript, security, curl, ssh ... are there).
	ExecFiles []string
	// ReadFiles are single files that may be read, each by its path: the manifests above the project (cargo looks for a workspace in
	// every directory above it, and stops with an error when it is not allowed to read one that is there).
	ReadFiles []string
	// Network allows the network (the fetch stage: cargo gets the sources of the dependencies, and resolves names itself): the
	// outgoing connections, and the services of the system that name resolution and TLS need (NetworkMach). The extraction never has it.
	Network bool
	// NetworkMach are the mach services that are allowed with the network (what the spike found that name resolution and TLS need).
	NetworkMach []string
	// ProxyPort, when not 0, is the one TCP port on the loopback that the program may connect to (the proxy of the fetch).
	// 0: no network at all.
	ProxyPort int
	// Timeout stops the program (and its process group) after this time. 0: no limit.
	Timeout time.Duration
	// MaxOutput is the most bytes of standard output that are kept; over it, the program is stopped. 0: no limit.
	MaxOutput int64
	// Stdout, when it is set, gets the standard output as it is written (the limit MaxOutput counts it too), and
	// Result.Stdout is nil: the caller that reads a long output does not have it all in memory.
	Stdout io.Writer
}

// Result is what a run gave.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	// TimedOut and OutputTooBig say why the program was stopped.
	TimedOut, OutputTooBig bool
	// MayHaveLeftProcesses is always true: a program that starts a process in a session of its own (setsid, a double fork)
	// leaves the process group that is killed at the end, and nothing here looks for it (rivus does not send a signal to a process
	// outside the group it started: the way of finding one that is not a mistake was tried and it killed other processes of the
	// user). The caller says so to the user. A process that is left is still under the sandbox.
	MayHaveLeftProcesses bool
	// Warnings are things that the caller says to the user about this run (a kernel that cannot close something, for example).
	Warnings []string
	// AttributeChanges are the files outside the places that the run may write, whose mode, owner, times or extended attributes the program
	// changed (Linux: Landlock does not stop that, so it is reported after the fact: "chmod /home/u/.ssh/id_rsa"). More than a few hundred are cut.
	AttributeChanges []string
}

// real resolves the symbolic links of a path: the sandbox matches the real path (/tmp is /private/tmp).
func real(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// limitWriter keeps at most left bytes; at the first byte over, it closes over (and drops the rest).
type limitWriter struct {
	w    io.Writer
	left int64 // 0: no limit
	over chan struct{}
	n    int64
	done bool
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.left == 0 {
		return l.w.Write(p)
	}
	if l.n+int64(len(p)) > l.left {
		if !l.done {
			l.done = true
			close(l.over)
		}
		return len(p), nil
	}
	l.n += int64(len(p))
	return l.w.Write(p)
}

// capWriter keeps at most left bytes of what is written.
type capWriter struct {
	w    io.Writer
	left int64
}

func (c *capWriter) Write(p []byte) (int, error) {
	n := len(p)
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	c.left -= int64(len(p))
	c.w.Write(p)
	return n, nil
}

// supervise runs cmd (its path and arguments are the caller's: the program, or the helper that makes the sandbox and then runs the program) in a
// session and a process group of its own, with the limits of s, and says how it ended. afterStart, when it is not nil, is called once the process
// has started. Only the group that this run started is killed: nothing is looked for outside it.
func supervise(ctx context.Context, s Spec, cmd *exec.Cmd, afterStart func()) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var cancel context.CancelFunc
	if s.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.Timeout)
		defer cancel()
	}
	cmd.Dir = s.Dir
	cmd.Env = s.Env
	cmd.Stdin = nil // /dev/null
	// a session of its own: no controlling terminal, and the group is the program and what it starts
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var stdout, stderr bytes.Buffer
	var out io.Writer = &stdout
	if s.Stdout != nil {
		out = s.Stdout
	}
	lw := &limitWriter{w: out, left: s.MaxOutput, over: make(chan struct{})}
	cmd.Stdout = lw
	cmd.Stderr = &capWriter{w: &stderr, left: 1 << 20}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if afterStart != nil {
		afterStart()
	}
	pgid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	res := &Result{}
	var werr error
	select {
	case werr = <-done:
	case <-ctx.Done():
		res.TimedOut = true
		syscall.Kill(-pgid, syscall.SIGKILL)
		werr = <-done
	case <-lw.over:
		res.OutputTooBig = true
		syscall.Kill(-pgid, syscall.SIGKILL)
		werr = <-done
	}
	// what is left of the group is killed whatever the way the program ended.
	syscall.Kill(-pgid, syscall.SIGKILL)
	res.MayHaveLeftProcesses = true
	res.Stderr = stderr.Bytes()
	if s.Stdout == nil {
		res.Stdout = stdout.Bytes()
	}
	if ee, ok := werr.(*exec.ExitError); ok {
		res.ExitCode = ee.ExitCode()
	} else if werr != nil {
		return res, werr
	}
	return res, nil
}
