//go:build darwin

// Package sandbox runs a program under the sandbox of macOS (Seatbelt, sandbox-exec). See docs/DESIGN-macos-sandbox.md.
//
// What is not here: nothing is run on the host when the sandbox cannot be made (Run returns ErrUnavailable): there is no
// way to ask for "the sandbox if there is one".
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// ErrUnavailable says that the sandbox cannot be used on this machine (sandbox-exec is not there, or the profile is not
// accepted): nothing was run.
var ErrUnavailable = errors.New("the sandbox of macOS (sandbox-exec) is not available")

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
	// ProxyPort, when not 0, is the one TCP port on the loopback that the program may connect to (the proxy of the fetch).
	// 0: no network at all.
	ProxyPort int
	// Timeout stops the program (and its process group) after this time. 0: no limit.
	Timeout time.Duration
	// MaxOutput is the most bytes of standard output that are kept; over it, the program is stopped. 0: no limit.
	MaxOutput int64
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
}

// profile is the text of the profile. Every path comes in as a parameter ((param "P0")), never in the text: a directory
// whose name has a parenthesis or a quote in it cannot change the profile (TestSandboxProfileTakesNoPathInItsText).
// A rule that is allowed has the reason beside it.
func profile(nRead, nExec, nWrite int, proxy bool) string {
	var b strings.Builder
	b.WriteString(`(version 1)
; Everything is denied, and what is needed is allowed, one line at a time with its reason.
(deny default)

; The base of the system that every program needs to start (dyld, the system libraries, /dev/null, ...): Apple's own profile.
(import "/System/Library/Sandbox/Profiles/bsd.sb")
; Other processes, services and applications are not reachable: no lookup of a mach service, no Apple Events, no signals out of the sandbox.
; (A rule without a filter does not take away what the base profile allows by a filter of its own: every name is denied by pattern.)
(deny mach-lookup (global-name-regex #"."))
(deny mach-lookup (xpc-service-name-prefix ""))
(deny mach-register)
(deny signal (target others))
; The home directory of the user: not its names, not its contents (what the caller names below it is allowed again, after this line).
(deny file-read* (subpath (param "HOME")))
; (Not here: the arguments and the environment of other processes of the user (kern.procargs2) can be read, and the sandbox cannot
; refuse it: a deny of the name, of the prefix "kern.", of process-info*, none of them stopped it. What is read cannot leave: TestSandboxClosesEveryWayOut.)
; The names of the top of the file system are not shown (nothing needs them: the paths that are allowed are named whole).
(deny file-read-metadata (literal "/Users") (literal "/Volumes") (literal "/Applications"))
`)
	for i := 0; i < nRead; i++ {
		fmt.Fprintf(&b, "; may be read: a directory that the caller named (the project, the dependencies, the toolchain)\n(allow file-read* (subpath (param \"R%d\")))\n", i)
	}
	for i := 0; i < nExec; i++ {
		fmt.Fprintf(&b, "; programs may be run from here, and the directory is read: the toolchain, the compilers, the extractor\n(allow file-read* process-exec* (subpath (param \"X%d\")))\n", i)
	}
	for i := 0; i < nWrite; i++ {
		fmt.Fprintf(&b, "; may be written: the working directory of this run only\n(allow file-read* file-write* (subpath (param \"W%d\")))\n", i)
	}
	b.WriteString("; a program may start programs (a build script runs the compiler)\n(allow process-fork)\n")
	if proxy {
		b.WriteString("; the one TCP port on the loopback that the caller named: the proxy of the fetch. Nothing else: no other port, address, protocol or name resolution.\n(allow network-outbound (remote tcp (string-append \"localhost:\" (param \"PROXY\"))))\n")
	}
	return b.String()
}

// Run runs the program under the sandbox and waits for it.
func Run(ctx context.Context, s Spec) (*Result, error) {
	if len(s.Argv) == 0 || !filepath.IsAbs(s.Argv[0]) {
		return nil, fmt.Errorf("sandbox: the program must be an absolute path: %v", s.Argv)
	}
	exe, err := exec.LookPath("sandbox-exec")
	if err != nil {
		return nil, ErrUnavailable
	}
	dir, err := os.MkdirTemp("", "rivus-sandbox-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	pf := filepath.Join(dir, "profile.sb")
	if err := os.WriteFile(pf, []byte(profile(len(s.Read), len(s.Exec)+1, len(s.Write), s.ProxyPort != 0)), 0o600); err != nil {
		return nil, err
	}
	args := []string{"-f", pf}
	home, _ := os.UserHomeDir()
	args = append(args, "-D", "HOME="+real(home))
	param := func(k, v string) { args = append(args, "-D", k+"="+v) }
	for i, d := range s.Read {
		param(fmt.Sprintf("R%d", i), real(d))
	}
	// the program itself is run from its own directory
	execs := append([]string{filepath.Dir(s.Argv[0])}, s.Exec...)
	for i, d := range execs {
		param(fmt.Sprintf("X%d", i), real(d))
	}
	for i, d := range s.Write {
		param(fmt.Sprintf("W%d", i), real(d))
	}
	if s.ProxyPort != 0 {
		param("PROXY", fmt.Sprint(s.ProxyPort))
	}
	args = append(args, "--")
	args = append(args, s.Argv...)

	if ctx == nil {
		ctx = context.Background()
	}
	var cancel context.CancelFunc
	if s.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.Timeout)
		defer cancel()
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = s.Dir
	cmd.Env = s.Env
	cmd.Stdin = nil // /dev/null
	// a session of its own: no controlling terminal, and the group is the program and what it starts
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var stdout, stderr bytes.Buffer
	lw := &limitWriter{w: &stdout, left: s.MaxOutput, over: make(chan struct{})}
	cmd.Stdout = lw
	cmd.Stderr = &capWriter{w: &stderr, left: 1 << 20}
	if err := cmd.Start(); err != nil {
		return nil, err
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
	// what is left of the group is killed whatever the way the program ended. (Only the group that this run started: nothing
	// is looked for outside it.)
	syscall.Kill(-pgid, syscall.SIGKILL)
	res.MayHaveLeftProcesses = true
	res.Stdout, res.Stderr = stdout.Bytes(), stderr.Bytes()
	if ee, ok := werr.(*exec.ExitError); ok {
		res.ExitCode = ee.ExitCode()
	} else if werr != nil {
		return res, werr
	}
	if bytes.Contains(res.Stderr, []byte("sandbox_apply")) && res.ExitCode != 0 {
		return res, ErrUnavailable
	}
	return res, nil
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

var _ = sort.Strings
