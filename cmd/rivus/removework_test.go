package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The work directory that a build ran in is removed also when the program made a directory in it that cannot be written (`mkdir d; touch
// d/f; chmod 500 d`): os.RemoveAll fails there (unlinkat: permission denied), and the failure was dropped by the defer, so the directory stayed.
func TestRemoveWorkWithAReadOnlyDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes it anyway")
	}
	work := filepath.Join(t.TempDir(), "rivus-extract-x")
	d := filepath.Join(work, "d")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(d, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(d, 0o755) })
	var said strings.Builder
	removeWork(work, &said)
	if _, err := os.Lstat(work); err == nil {
		t.Errorf("the work directory is still there (said: %q)", said.String())
	}
	if said.Len() != 0 {
		t.Errorf("nothing to say when it is removed: %q", said.String())
	}
}

// When it cannot be removed, it is said (on the standard error of the run), not dropped.
func TestRemoveWorkSaysWhenItFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes it anyway")
	}
	keep := filepath.Join(t.TempDir(), "keep")
	if err := os.MkdirAll(filepath.Join(keep, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// the parent cannot be written: sub cannot be unlinked
	if err := os.Chmod(keep, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(keep, 0o755) })
	var said strings.Builder
	removeWork(filepath.Join(keep, "sub"), &said)
	if !strings.Contains(said.String(), "could not remove") {
		t.Errorf("a failure is said: %q", said.String())
	}
}
