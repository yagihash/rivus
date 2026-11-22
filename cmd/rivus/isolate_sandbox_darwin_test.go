//go:build darwin

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSandboxedBuildScriptCannotEscape: a build script that is hostile on purpose (extractor/rust/test/hostile_sandbox) runs the probe
// (internal/sandbox/probe), which tries to get out in every way that the tests of the sandbox know of; it is built by the extractor under
// the sandbox, the way a project is. What the probe says goes into the IR (the report of the build script): every check must say BLOCKED.
// It touches the environment of the user in the harmless way of the design (a unique name, removed after), so it needs RIVUS_SBX_RED=1.
func TestSandboxedBuildScriptCannotEscape(t *testing.T) {
	if os.Getenv("RIVUS_SBX_RED") != "1" {
		t.Skip("set RIVUS_SBX_RED=1 (it runs the probe, which makes harmless marks outside the sandbox if it gets out)")
	}
	rust := os.Getenv("RIVUS_RUST")
	if rust == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	probe := filepath.Join(t.TempDir(), "probe")
	if out, err := exec.Command("go", "build", "-o", probe, "../../internal/sandbox/probe").CombinedOutput(); err != nil {
		t.Fatalf("building the probe: %v\n%s", err, out)
	}
	outside := t.TempDir() // a directory of the test that the build may not write to
	canary := filepath.Join(outside, "canary.txt")
	os.WriteFile(canary, []byte("a fake secret\n"), 0o644)
	project, _ := filepath.Abs("../../extractor/rust/test/hostile_sandbox")
	extractor, _ := filepath.Abs(rust)
	realHome, _ := os.UserHomeDir()
	args := []string{"-home", realHome, "-id", "e2e" + filepath.Base(outside), "-dir", outside, "-canary", canary, "-sensitive",
		"-only", "read_canary,write_outside,write_home,defaults_write,launchagents_file,shell_rc_file,osascript_finder_folder,launchctl_submit,open_finder,pbpaste,securityd_reachable,dns_socket,shm_open,crontab_list,unix_socket_docker"}
	iso := &isolationOpts{
		extractor: []string{extractor}, timeout: 10 * time.Minute, noFetch: false, stderr: &bytes.Buffer{},
		env:       []string{"RIVUS_PROBE_BIN=" + probe, "RIVUS_PROBE_ARGS=" + strings.Join(args, "\x01")},
		execExtra: []string{probe},
	}
	// the probe is read by path: it is in a temporary directory outside the project
	iso.mounts = []string{filepath.Dir(probe) + ":" + filepath.Dir(probe)}
	out, err := iso.extractRustSandboxed(context.Background(), project, []string{project})
	if err != nil {
		t.Fatalf("the extraction under the sandbox: %v\n%s", err, iso.stderr.(*bytes.Buffer).String())
	}
	report := ""
	for _, l := range strings.Split(string(out), "\n") {
		if i := strings.Index(l, "BLOCKED read_canary"); i >= 0 {
			report = l
			break
		}
	}
	if report == "" {
		t.Fatalf("the report of the build script is not in the IR (the probe did not run?): %d bytes\n%s", len(out), iso.stderr.(*bytes.Buffer).String())
	}
	if strings.Contains(report, "ESCAPED") {
		t.Errorf("the build script got out:\n%s", strings.ReplaceAll(report, ";", "\n"))
	}
	if strings.Contains(report, "PROBE_NOT_RUN") {
		t.Errorf("the probe did not run: %s", report)
	}
}
