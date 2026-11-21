package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `extract -lang rust` reads in the container too, and without Docker it stops: it does not fall back to the machine.
func TestExtractRustWithoutDockerFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.lock"), []byte("version = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir()) // no docker
	t.Setenv("RIVUS_UNSAFE_HOST_EXEC", "")
	err := runExtract([]string{"-lang", "rust", "-C", dir})
	if err == nil || !strings.Contains(err.Error(), "Docker") || !strings.Contains(err.Error(), "-unsafe-host-exec") {
		t.Fatalf("want the error that names Docker, got %v", err)
	}
}
