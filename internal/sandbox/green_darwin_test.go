//go:build darwin

package sandbox

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// The green half: the probe runs under the sandbox, and every check says BLOCKED. The programs of the system may be run
// here (/bin, /usr/bin, ...), so that what stops a check is the rule of the operation (the file, the mach service, the
// network, the signal), not the fact that the program cannot be started at all. It touches the environment of the user
// in the same harmless way as the red half, so it needs the same RIVUS_SBX_RED=1 (what it makes is removed).
func TestSandboxStopsEveryHoleOfTheProbe(t *testing.T) {
	if os.Getenv("RIVUS_SBX_RED") != "1" {
		t.Skip("set RIVUS_SBX_RED=1 (see red_darwin_test.go)")
	}
	probe := probeBinary(t)
	w := newWorld(t, probe)
	defer w.close()
	res, err := Run(context.Background(), Spec{
		Argv:    append([]string{probe}, w.args("-daemon", "-sensitive")...),
		Dir:     "/",
		Env:     []string{"PATH=/usr/bin:/bin", "RIVUS_SBX_FAKE_TOKEN_NOT_PASSED=1"},
		Exec:    []string{"/bin", "/usr/bin", "/usr/sbin", "/sbin"},
		Timeout: 2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	said := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && f[0] != "LEFT_PID" {
			said[f[1]] = f[0]
		}
	}
	if len(said) < 20 {
		t.Fatalf("the probe said little (exit %d): stdout\n%s\nstderr\n%s", res.ExitCode, res.Stdout, res.Stderr)
	}
	// leave_process is not stopped (a build script starts processes: that is allowed), and the run does not look for what leaves
	// the group: it says that processes may be left (MayHaveLeftProcesses). read_env_of_other_process is a weakness that was measured and is documented
	// (DESIGN-macos-sandbox.md, 2): the sandbox cannot take away the environment of other processes of the user. When it
	// is blocked one day, this test says so, and the weakness is struck from the design.
	knownWeak := map[string]bool{"read_env_of_other_process": true, "leave_process": true}
	for n, st := range said {
		switch {
		case knownWeak[n]:
		case st != "BLOCKED":
			t.Errorf("%s: %s under the sandbox", n, st)
		}
	}
	if said["read_env_of_other_process"] == "BLOCKED" {
		t.Errorf("read_env_of_other_process is blocked now: strike it from the weaknesses of the design, and from knownWeak")
	}
	// A process that left the group (setsid, a double fork) is not found by the run, and the run says so. It is the test that
	// knows it (the probe reported its pid), and removes it when the test ends.
	if len(w.noteLeft(res.Stdout)) == 0 {
		t.Errorf("the probe did not report the process that it left behind")
	}
	if !res.MayHaveLeftProcesses {
		t.Errorf("the run must say that processes may have been left behind")
	}
	t.Logf("what the probe said under the sandbox:\n%s", res.Stdout)
}

// TestSandboxClosesEveryWayOut is the set of tests that the acceptance of a weakness stands on: under the sandbox, other
// processes of the user can be asked for their arguments and environment (kern.procargs2: the sandbox cannot refuse it; see
// TestSandboxProcargsRange and DESIGN-macos-sandbox.md, 2). The user accepted that on the ground that **nothing that is read can be sent out
// of the build**. If any one of the ways out below is open, an environment that was read can leave, and the acceptance is
// gone with it. So each way is tried, and none may work: the network (TCP, UDP, IPv6), UNIX domain sockets (the
// agents, docker), the name resolution service, other applications and services (Apple Events, open, launchctl, the
// pasteboard, the preferences), writing outside what is allowed, shared memory. (What stays open is the output of rivus
// itself: the result and the log can hold what the build wrote. That is said in the documents.)
func TestSandboxClosesEveryWayOut(t *testing.T) {
	if os.Getenv("RIVUS_SBX_RED") != "1" {
		t.Skip("set RIVUS_SBX_RED=1 (see red_darwin_test.go)")
	}
	ways := []string{"network_loopback", "network_lan", "network_udp", "network_ipv6_loopback", "unix_socket_test", "unix_socket_docker",
		"unix_socket_ssh_agent", "dns_socket", "osascript_finder_folder", "launchctl_submit", "open_finder", "pbpaste", "securityd_reachable",
		"defaults_write", "defaults_read", "write_outside", "write_home", "launchagents_file", "shell_rc_file", "shm_open", "crontab_list"}
	probe := probeBinary(t)
	w := newWorld(t, probe)
	defer w.close()
	res, err := Run(context.Background(), Spec{
		Argv:    append([]string{probe}, w.args("-sensitive", "-only", strings.Join(ways, ","))...),
		Dir:     "/",
		Env:     []string{"PATH=/usr/bin:/bin"},
		Exec:    []string{"/bin", "/usr/bin", "/usr/sbin", "/sbin"},
		Timeout: 2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	said := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
		if f := strings.Fields(l); len(f) >= 2 {
			said[f[1]] = f[0]
		}
	}
	for _, n := range ways {
		switch said[n] {
		case "BLOCKED":
		case "":
			t.Errorf("%s: the probe did not say (stdout %q)", n, res.Stdout)
		default:
			t.Errorf("%s: %s: a way out is open under the sandbox", n, said[n])
		}
	}
}
