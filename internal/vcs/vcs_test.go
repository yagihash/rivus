package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=a@b", "-c", "user.name=x"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	dir := t.TempDir()
	run(t, dir, "init", "-q", ".")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-qm", "init")
	return dir
}

// The repository under analysis is not trusted: git is not run at all to find the revision, so nothing that
// the configuration names can run (yagihash/rivus #6).
func TestDetectRunsNothingAndFindsTheRevision(t *testing.T) {
	dir := repo(t)
	mark := filepath.Join(t.TempDir(), "RAN")
	extra := filepath.Join(dir, ".git", "extra")
	if err := os.WriteFile(extra, []byte("[filter \"x\"]\n\tclean = touch "+mark+"; cat\n[core]\n\tfsmonitor = touch "+mark+"; false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "config", "include.path", "extra")
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("*.txt filter=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	_ = os.Chtimes(filepath.Join(dir, "a.txt"), later, later)
	want, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	// the mark made by the git of this test itself (rev-parse reads no filter) is removed before Detect
	_ = os.Remove(mark)
	tg := Detect(dir)
	if tg == nil || tg.Revision != strings.TrimSpace(string(want)) {
		t.Fatalf("the revision is not found: %+v, want %s", tg, want)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("the repository's configuration ran a command")
	}
}

func TestDetectReadsPackedRefsAndWorktrees(t *testing.T) {
	dir := repo(t)
	want, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	run(t, dir, "pack-refs", "--all")
	if tg := Detect(dir); tg == nil || tg.Revision != strings.TrimSpace(string(want)) {
		t.Fatalf("packed refs: %+v", tg)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	run(t, dir, "worktree", "add", "-q", "-b", "other", wt)
	if tg := Detect(wt); tg == nil || tg.Revision != strings.TrimSpace(string(want)) {
		t.Fatalf("linked worktree: %+v", tg)
	}
	sub := filepath.Join(dir, "d")
	_ = os.MkdirAll(sub, 0o755)
	if tg := Detect(sub); tg == nil {
		t.Fatal("a subdirectory of the working tree")
	}
	if tg := Detect(t.TempDir()); tg != nil {
		t.Fatalf("outside git: %+v", tg)
	}
}
