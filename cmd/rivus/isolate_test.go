package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMountable(t *testing.T) {
	if home, err := os.UserHomeDir(); err == nil {
		if err := mountable(home); err == nil {
			t.Error("the home directory is mounted")
		}
		if err := mountable(filepath.Dir(home)); err == nil {
			t.Error("a directory that holds the home directory is mounted")
		}
	}
	if err := mountable(t.TempDir()); err != nil {
		t.Errorf("a directory of the project: %v", err)
	}
}

func TestProjectToolchain(t *testing.T) {
	dir := t.TempDir()
	if projectToolchain(dir) != "" {
		t.Error("no file")
	}
	os.WriteFile(filepath.Join(dir, "rust-toolchain.toml"), []byte("[toolchain]\nchannel = \"1.70.0\"\n"), 0o644)
	if got := projectToolchain(dir); got != "1.70.0" {
		t.Errorf("toml: %q", got)
	}
	dir = t.TempDir()
	os.WriteFile(filepath.Join(dir, "rust-toolchain"), []byte("nightly-2020-01-01\n"), 0o644)
	if got := projectToolchain(dir); got != "nightly-2020-01-01" {
		t.Errorf("plain: %q", got)
	}
}

func TestCargoVolumeNeedsALockFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := cargoVolume(dir); err == nil || !strings.Contains(err.Error(), "Cargo.lock") {
		t.Errorf("no lock file: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "Cargo.lock"), []byte("a"), 0o644)
	a, err := cargoVolume(dir)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "Cargo.lock"), []byte("b"), 0o644)
	if b, _ := cargoVolume(dir); a == b {
		t.Error("two lock files, one volume")
	}
}

// The project that is hostile on purpose is not run on this machine, whatever the flags are.
func TestHostileProjectIsRefusedOnTheHost(t *testing.T) {
	o := analyzeOpts{}
	o.dir = "../../extractor/rust/test/hostile_sandbox"
	o.lang = "rust"
	o.unsafeHostExec = true
	o.rustExtractor = "/nonexistent/rivus-rust"
	_, err := o.runExtractor("rust")
	if err == nil || !strings.Contains(err.Error(), "hostile on purpose") {
		t.Errorf("not refused: %v", err)
	}
}
