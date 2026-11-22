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
func profile(nRead, nExec, nWrite, nFiles int, proxy bool, network bool, nMach int) string {
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
	// the directories above a directory that is named can be stat-ed (not listed, not read): the path is resolved by looking at them
	for i := 0; i < nRead; i++ {
		fmt.Fprintf(&b, "; may be read: a directory that the caller named (the project, the dependencies, the toolchain)\n(allow file-read* (subpath (param \"R%d\")))\n", i)
		writeAncestors(&b, "R", i)
	}
	for i := 0; i < nExec; i++ {
		fmt.Fprintf(&b, "; programs may be run from here, and the directory is read: the toolchain, the compilers, the extractor\n(allow file-read* process-exec* (subpath (param \"X%d\")))\n", i)
		writeAncestors(&b, "X", i)
	}
	for i := 0; i < nFiles; i++ {
		fmt.Fprintf(&b, "; this program may be run (it is named by its path): one that a build needs\n(allow file-read* process-exec* (literal (param \"E%d\")))\n", i)
		writeAncestors(&b, "E", i)
	}
	for i := 0; i < nWrite; i++ {
		fmt.Fprintf(&b, "; may be written: the working directory of this run only. Programs may be run from it too: the build scripts and the proc macros that the build makes there. THIS IS THE RUN OF THE CODE OF THE PROJECT ITSELF (a build script is the code that is being read): it cannot be narrowed, and what the sandbox does is to keep that code in.\n(allow file-read* file-write* process-exec* (subpath (param \"W%d\")))\n(allow file-map-executable (subpath (param \"W%d\")))\n", i, i)
		writeAncestors(&b, "W", i)
	}
	b.WriteString("; a program may start programs (a build script runs the compiler)\n(allow process-fork)\n")
	if network {
		b.WriteString("; the fetch stage only: the network, and the services of the system that name resolution and TLS use (each named; see Spec.NetworkMach)\n(allow network-outbound)\n(allow system-socket)\n")
		for i := 0; i < nMach; i++ {
			fmt.Fprintf(&b, "(allow mach-lookup (global-name (param \"M%d\")))\n", i)
		}
	}
	if proxy {
		b.WriteString("; the one TCP port on the loopback that the caller named: the proxy of the fetch. Nothing else: no other port, address, protocol or name resolution.\n(allow network-outbound (remote tcp (string-append \"localhost:\" (param \"PROXY\"))))\n")
	}
	return b.String()
}

// writeAncestors writes the line that lets the directories above the directory of a parameter be stat-ed (not listed, not read): the
// path is resolved by looking at them.
func writeAncestors(b *strings.Builder, kind string, i int) {
	fmt.Fprintf(b, "(allow file-read-metadata (path-ancestors (param \"%s%d\")))\n", kind, i)
}

// argList is the arguments of sandbox-exec: the parameters of the profile are added one by one.
type argList []string

func (a *argList) param(k, v string) { *a = append(*a, "-D", k+"="+v) }

// waitFor waits for the command and says how it ended.
func waitFor(cmd *exec.Cmd, done chan<- error) { done <- cmd.Wait() }

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
	// the program itself is run as a file: its directory is not allowed with it (the directory of a program that was installed with brew
	// or cargo holds other programs)
	files := append(append([]string(nil), s.ExecFiles...), s.Argv[0])
	pf := filepath.Join(dir, "profile.sb")
	if err := os.WriteFile(pf, []byte(profile(len(s.Read), len(s.Exec), len(s.Write), len(files), s.ProxyPort != 0, s.Network, len(s.NetworkMach))), 0o600); err != nil {
		return nil, err
	}
	args := argList{"-f", pf}
	home, _ := os.UserHomeDir()
	args.param("HOME", real(home))
	for i, d := range s.Read {
		args.param(fmt.Sprintf("R%d", i), real(d))
	}
	for i, d := range s.Exec {
		args.param(fmt.Sprintf("X%d", i), real(d))
	}
	for i, f := range files {
		args.param(fmt.Sprintf("E%d", i), real(f))
	}
	for i, d := range s.Write {
		args.param(fmt.Sprintf("W%d", i), real(d))
	}
	for i, m := range s.NetworkMach {
		args.param(fmt.Sprintf("M%d", i), m)
	}
	if s.ProxyPort != 0 {
		args.param("PROXY", fmt.Sprint(s.ProxyPort))
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
	go waitFor(cmd, done)
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

// Available says whether the sandbox can be made on this machine: sandbox-exec is there and accepts the profile that a run uses (a
// program that does nothing is run under it once). A caller asks before it runs the code of a project, and never takes what
// the project's program writes to its standard error for an answer.
func Available() error {
	res, err := Run(context.Background(), Spec{Argv: []string{"/usr/bin/true"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Timeout: 30 * time.Second})
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("%w: the profile of a run was not accepted (a program that does nothing exited %d): %s", ErrUnavailable, res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}
