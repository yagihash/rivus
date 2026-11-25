package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// `extract -lang rust` reads under the sandbox too, and when the sandbox cannot be made (sandbox-exec is not there; on another system there is none) it stops
// and runs nothing: it does not fall back to the machine.
func TestExtractRustWithoutTheSandboxStops(t *testing.T) {
	if runtime.GOOS == "linux" && sandboxUsable() == nil {
		// the sandbox of Linux (Landlock) does not depend on a program on PATH: the case here is a system that has none
		t.Skip("this system has the sandbox of Linux")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.lock"), []byte("version = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir()) // no sandbox-exec
	t.Setenv("RIVUS_UNSAFE_HOST_EXEC", "")
	err := runExtract([]string{"-lang", "rust", "-C", dir})
	if err == nil || !strings.Contains(err.Error(), "-unsafe-host-exec") || !strings.Contains(err.Error(), "nothing was run") {
		t.Fatalf("want the error that says nothing was run and names -unsafe-host-exec, got %v", err)
	}
}
