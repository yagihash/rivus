package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
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

// The repository under analysis is not trusted: what its configuration names is not run to
// find the revision and whether the tree is dirty (yagihash/rivus #6).
func TestDetectRunsNothingThatTheRepositoryNames(t *testing.T) {
	dir := repo(t)
	mark := filepath.Join(t.TempDir(), "RAN")
	run(t, dir, "config", "core.fsmonitor", "touch "+mark+"; false")
	if tg := Detect(dir); tg == nil || tg.Revision == "" {
		t.Fatalf("the revision is not found: %+v", tg)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("core.fsmonitor of the repository was run")
	}
}

func TestDetectDoesNotRunTheFiltersOfTheRepository(t *testing.T) {
	dir := repo(t)
	mark := filepath.Join(t.TempDir(), "RAN")
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("*.txt filter=evil\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "config", "filter.evil.clean", "sh -c 'touch "+mark+"; cat'")
	// a file whose stat differs from the index is the one git passes through the clean filter
	f := filepath.Join(dir, "a.txt")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(f, later, later); err != nil {
		t.Fatal(err)
	}
	if tg := Detect(dir); tg == nil {
		t.Fatal("no target")
	}
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("a filter of the repository was run")
	}
}

func TestDetectStillSeesADirtyTree(t *testing.T) {
	dir := repo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if tg := Detect(dir); tg == nil || !tg.Dirty {
		t.Fatalf("a changed file is not dirty: %+v", tg)
	}
}
