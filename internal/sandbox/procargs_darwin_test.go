//go:build darwin

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The range of kern.procargs2 (docs/DESIGN-macos-sandbox.md, 2: the arguments and the environment of other processes of the user can
// be read under the sandbox, and the user accepted it because nothing that is read can be sent out: TestSandboxClosesEveryWayOut). This says
// which processes can be asked, with and without the sandbox. The probe says only yes or no for each: what a process holds is not
// kept, not printed. No signal is sent. The processes are named by pgrep (it only reads the list).
func TestSandboxProcargsRange(t *testing.T) {
	if os.Getenv("RIVUS_SBX_RED") != "1" {
		t.Skip("set RIVUS_SBX_RED=1 (see red_darwin_test.go)")
	}
	pgrep := func(args ...string) string {
		out, err := exec.Command("/usr/bin/pgrep", args...).Output()
		if err != nil {
			return ""
		}
		f := strings.Fields(string(out))
		if len(f) == 0 {
			return ""
		}
		return f[0]
	}
	var targets []string
	add := func(label, pid string) {
		if pid != "" {
			targets = append(targets, label+"="+pid)
		}
	}
	add("launchd_root", "1")
	add("root_daemon_configd", pgrep("-x", "configd"))
	add("user_app_Finder", pgrep("-x", "Finder"))
	add("user_shell_zsh", pgrep("-x", "zsh"))
	add("browser_helper", pgrep("-f", "Helper"))
	add("other_user_WindowServer", pgrep("-x", "WindowServer"))
	probe := probeBinary(t)
	w := newWorld(t, probe)
	defer w.close()
	args := w.args("-only", "procargs_range", "-targets", strings.Join(targets, ","))
	red, err := exec.Command(probe, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v\n%s", err, red)
	}
	res, err := Run(context.Background(), Spec{
		Argv: append([]string{probe}, args...), Dir: "/", Env: []string{"PATH=/usr/bin:/bin"},
		Exec: []string{"/bin", "/usr/bin"}, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("without the sandbox: %s", strings.TrimSpace(string(red)))
	t.Logf("under the sandbox:   %s", strings.TrimSpace(string(res.Stdout)))
}
