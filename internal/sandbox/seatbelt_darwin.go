//go:build darwin

// The sandbox of macOS (Seatbelt, sandbox-exec). See docs/DESIGN-macos-sandbox.md. The types and the supervision are in sandbox.go.
package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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

// Run runs the program under the sandbox and waits for it.
func Run(ctx context.Context, s Spec) (*Result, error) {
	if len(s.Argv) == 0 || !filepath.IsAbs(s.Argv[0]) {
		return nil, fmt.Errorf("sandbox: the program must be an absolute path: %v", s.Argv)
	}
	// the home directory is denied for reading by name (profile): when it is not known, real("") would be the current directory, the
	// deny would be for the wrong place, and the home directory would be as readable as the allowed paths say. Nothing is run then.
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, fmt.Errorf("sandbox: the home directory of the user is not known (HOME is not set), so it cannot be kept out of the sandbox: %v", err)
	}
	exe, err := exec.LookPath("sandbox-exec")
	if err != nil {
		return nil, fmt.Errorf("%w: sandbox-exec is not there", ErrUnavailable)
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

	cmd := exec.Command(exe, args...)
	return supervise(ctx, s, cmd, nil)
}

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
