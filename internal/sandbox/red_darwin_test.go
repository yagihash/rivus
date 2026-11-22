//go:build darwin

package sandbox

import (
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// The red half of the tests of the sandbox (docs/DESIGN-macos-sandbox.md, 3.9): without the sandbox, the holes are real.
// The probe runs on this machine, with the rights of the user, and every check of the red set must say ESCAPED. A check
// that says BLOCKED here proves nothing when it says BLOCKED under the sandbox, so this half is what gives the green half
// its meaning.
//
// It touches the environment of the user, in the harmless way of the design (a unique name rivus-sbx-<id> that is
// removed: a preference domain, an empty file in LaunchAgents, a job of launchctl, a folder that Finder makes in a
// directory of the test), and it may make macOS ask whether the terminal may control Finder. So it runs only when
// RIVUS_SBX_RED=1 is set. The pasteboard, the keychain, the screen and the resources are never run here.
func TestSandboxHolesAreRealWithoutTheSandbox(t *testing.T) {
	if os.Getenv("RIVUS_SBX_RED") != "1" {
		t.Skip("set RIVUS_SBX_RED=1: this runs the probe on this machine, without a sandbox, and touches (harmlessly) the environment of the user")
	}
	probe := probeBinary(t)
	w := newWorld(t, probe)
	defer w.close()
	cmd := exec.Command(probe, w.args("-daemon")...)
	cmd.Env = append(os.Environ(), "RIVUS_SBX_FAKE_TOKEN="+w.token)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v\n%s", err, out)
	}
	got := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && f[0] != "LEFT_PID" {
			got[f[1]] = f[0]
		}
	}
	// the checks that must escape. The ones that depend on the machine (a LAN address, a Docker socket, an ssh agent, a terminal,
	// Finder allowed to be controlled) are listed apart: they are said, and not required.
	red := []string{"read_canary", "names_visible", "write_outside", "write_home", "defaults_write", "launchagents_file", "shell_rc_file",
		"unix_socket_test", "network_loopback", "network_udp", "network_ipv6_loopback", "dns_socket", "env_token", "signal_other_process",
		"read_env_of_other_process", "defaults_read", "leave_process"}
	maybe := []string{"crontab_list", "osascript_finder_folder", "launchctl_submit", "open_finder", "network_lan", "unix_socket_docker",
		"unix_socket_ssh_agent", "dev_tty", "write_toolchain"}
	for _, n := range red {
		if got[n] != "ESCAPED" {
			t.Errorf("%s: %q without the sandbox: the hole is not there, or the probe cannot show it\n%s", n, got[n], out)
		}
	}
	// the process that was left behind is reported by the probe (LEFT_PID), and removed when the test ends
	if len(w.noteLeft(out)) == 0 {
		t.Errorf("the probe did not report the process that it left behind (setsid, double fork)")
	}
	var names []string
	for n := range got {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range maybe {
		t.Logf("%-26s %s", n, got[n])
	}
	t.Logf("what the probe said:\n%s", out)
}
