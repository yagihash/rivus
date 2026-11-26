//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The sandbox does a chmod for the program after it has looked at the path of the file (notify_linux.go). That is sound only if the program cannot bring the inode of a
// file outside into the work directory under a name of its own: a chmod of the new name would be inside by its path and would change the file outside. Landlock
// refuses a link or a rename that takes a file from a directory that does not have the right REFER (the directory outside is only read); this shows that it does,
// for each of the ways, and that the file outside is not changed.

var linkChecksNames = []string{"link-outside", "linkat-outside", "linkat-proc-fd-outside", "rename-outside", "renameat2-outside"}

func linkFixture(t *testing.T, f fixture) (src string) {
	t.Helper()
	src = filepath.Join(f.outside, "src")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"l1", "l2", "l3", "l4", "l5"} {
		if err := os.WriteFile(filepath.Join(src, n), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return src
}

// Without the sandbox the names are made (and the file outside is changed): otherwise "blocked" below would show nothing.
func TestSandboxLinkOutsideWorksOnTheHost(t *testing.T) {
	f := setup(t)
	src := linkFixture(t, f)
	out, err := exec.Command(f.probe, "-link-src", src, "-link-dst", f.work).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	v := verdicts(t, string(out))
	for _, c := range linkChecksNames {
		if esc, ok := v[c]; !ok || !esc {
			t.Skipf("%s: on the host the probe could not make the name (a file system or a setting that does not allow it: fs.protected_hardlinks?):\n%s", c, out)
		}
	}
}

// Under the sandbox no name is made, and the mode of the file outside is not changed.
func TestSandboxLinkOutsideIsRefused(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	src := linkFixture(t, f)
	res := attrRun(t, f, []string{f.outside}, "-link-src", src, "-link-dst", f.work)
	t.Logf("probe under the sandbox:\n%s", res.Stdout)
	v := verdicts(t, string(res.Stdout))
	for _, c := range linkChecksNames {
		esc, ok := v[c]
		if !ok {
			t.Errorf("%s: the probe did not say\n%s", c, res.Stdout)
		} else if esc {
			t.Errorf("%s: ESCAPED under the sandbox: a file outside has a name in the work directory\n%s", c, res.Stdout)
		}
	}
	for _, n := range []string{"l1", "l2", "l3", "l4", "l5"} {
		if got := modeOf(t, filepath.Join(src, n)); got != 0o600 {
			t.Errorf("%s: the mode of the file outside was changed: %v", n, got)
		}
		if _, err := os.Lstat(filepath.Join(f.work, n)); err == nil {
			t.Errorf("%s: a name was made in the work directory", n)
		}
	}
}
