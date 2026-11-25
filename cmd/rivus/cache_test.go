package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests work in a temporary directory that is passed to the functions: they never touch the data directory of the user.

const (
	nameA = "rivus-cargo-0123456789abcdef"
	nameB = "rivus-cargo-fedcba9876543210"
)

// place makes a place with a file of size bytes whose time (and the time of the place) is age ago.
func place(t *testing.T, root, name string, size int, age time.Duration) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(p, "registry"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(p, "registry", "crate")
	if err := os.WriteFile(f, make([]byte, size), 0o444); err != nil { // read-only, as cargo makes them
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	for _, x := range []string{f, filepath.Join(p, "registry"), p} {
		if err := os.Chtimes(x, when, when); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

const day = 24 * time.Hour

func TestCacheListSizeAndDate(t *testing.T) {
	root := t.TempDir()
	place(t, root, nameA, 1500, 10*day)
	var out bytes.Buffer
	if err := cacheList(root, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	want := time.Now().Add(-10 * day).Format("2006-01-02")
	for _, s := range []string{filepath.Join(root, nameA), "1500", want} {
		if !strings.Contains(got, s) {
			t.Errorf("the list lacks %q:\n%s", s, got)
		}
	}
}

func TestCacheListNoRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "none")
	var out bytes.Buffer
	if err := cacheList(root, &out); err != nil {
		t.Fatalf("a missing directory is not an error: %v", err)
	}
	if !strings.Contains(out.String(), root) {
		t.Errorf("the place that was looked in is not said: %s", out.String())
	}
}

func TestCacheCleanDefaultOnlyLists(t *testing.T) {
	root := t.TempDir()
	p := place(t, root, nameA, 10, 40*day)
	var out bytes.Buffer
	if err := cacheClean(root, 30, time.Now(), false, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !exists(p) {
		t.Fatal("removed without -yes")
	}
	if !strings.Contains(out.String(), nameA) {
		t.Errorf("the place that would go is not listed: %s", out.String())
	}
}

func TestCacheCleanAskNo(t *testing.T) {
	root := t.TempDir()
	p := place(t, root, nameA, 10, 40*day)
	if err := cacheClean(root, 30, time.Now(), false, func(string) bool { return false }, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !exists(p) {
		t.Fatal("removed after the answer no")
	}
}

func TestCacheCleanYesRemovesOnlyOld(t *testing.T) {
	root := t.TempDir()
	old := place(t, root, nameA, 10, 40*day)
	young := place(t, root, nameB, 10, 5*day)
	if err := cacheClean(root, 30, time.Now(), true, nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if exists(old) {
		t.Error("the old place (with read-only files) was not removed")
	}
	if !exists(young) {
		t.Error("the young place was removed")
	}
}

func TestCacheCleanAskYes(t *testing.T) {
	root := t.TempDir()
	old := place(t, root, nameA, 10, 40*day)
	if err := cacheClean(root, 30, time.Now(), false, func(string) bool { return true }, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if exists(old) {
		t.Error("not removed after the answer yes")
	}
}

func TestCacheCleanKeepsInUse(t *testing.T) {
	root := t.TempDir()
	// older than 0 days, but written 10 minutes ago: in use
	p := place(t, root, nameA, 10, 10*time.Minute)
	var out bytes.Buffer
	if err := cacheClean(root, 0, time.Now(), true, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !exists(p) {
		t.Fatal("a place in use was removed")
	}
	if !strings.Contains(out.String(), "in use") {
		t.Errorf("the skip is not said: %s", out.String())
	}
}

func TestCacheCleanOutsideIsKept(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "rust")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	precious := filepath.Join(outside, "precious")
	os.WriteFile(precious, []byte("x"), 0o644)
	old := time.Now().Add(-90 * day)
	os.Chtimes(precious, old, old)
	os.Chtimes(outside, old, old)

	os.MkdirAll(root, 0o755)
	// a link with the right name that points outside
	if err := os.Symlink(outside, filepath.Join(root, nameA)); err != nil {
		t.Fatal(err)
	}
	// directories that are not places: a name that is not of the form, and the extractor
	others := []string{"extractor", "rivus-cargo-xyz", "rivus-cargo-0123456789ABCDEF", "rivus-cargo-0123456789abcdef0", "my-rivus-cargo-0123456789abcdef", "rivus-cargo-..", "rivus-cargo-0123456789abcde."}
	for _, n := range others {
		place(t, root, n, 10, 90*day)
	}
	// a link inside a good place that points outside: the link goes, not what it points to
	good := place(t, root, nameB, 10, 90*day)
	link := filepath.Join(good, "link")
	os.Symlink(outside, link)
	if out, err := exec.Command("touch", "-h", "-t", old.Format("200601021504"), link).CombinedOutput(); err != nil { // the time of the link itself
		t.Fatalf("%v: %s", err, out)
	}
	os.Chtimes(good, old, old)

	var out bytes.Buffer
	if err := cacheClean(root, 30, time.Now(), true, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !exists(precious) {
		t.Fatal("something outside was removed")
	}
	for _, n := range others {
		if !exists(filepath.Join(root, n)) {
			t.Errorf("%s is not a place and was removed", n)
		}
	}
	if !exists(filepath.Join(root, nameA)) {
		t.Error("the link was removed (it is not a place)")
	}
	if exists(good) {
		t.Error("the good old place was not removed")
	}
}

func TestCacheRootIsALinkStillChecksRealPaths(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	place(t, real, nameA, 10, 90*day)
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	ps, err := cachePlaces(link)
	if err != nil || len(ps) != 1 {
		t.Fatalf("%v, %v", ps, err)
	}
}

func TestCacheCleanNegativeDays(t *testing.T) {
	root := t.TempDir()
	p := place(t, root, nameA, 10, 90*day)
	if err := cacheClean(root, -1, time.Now(), true, nil, &bytes.Buffer{}); err == nil {
		t.Fatal("a negative number of days is an error")
	}
	if !exists(p) {
		t.Fatal("removed on an error")
	}
}

func TestCacheCleanCommandNeedsOlderThan(t *testing.T) {
	for _, args := range [][]string{{"cache", "clean"}, {"cache", "clean", "-older-than", "-3"}} {
		if out, err := exec.Command(binary, args...).CombinedOutput(); err == nil {
			t.Errorf("rivus %v: no error: %s", args, out)
		}
	}
}

// the command reads the data directory of the environment (the tests set RIVUS_RUST_DATA_DIR to a temporary one).
func TestCacheListCommandUsesTheDataDir(t *testing.T) {
	dir := t.TempDir()
	place(t, dir, nameA, 7, 2*day)
	cmd := exec.Command(binary, "cache", "list")
	cmd.Env = append(os.Environ(), "RIVUS_RUST_DATA_DIR="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), nameA) {
		t.Errorf("%v: %s", err, out)
	}
}
