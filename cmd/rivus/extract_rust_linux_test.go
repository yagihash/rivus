//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/sandbox"
)

// `extract -lang rust` reads under the sandbox, and when the sandbox cannot be made it stops, runs nothing, and says that -unsafe-host-exec is the way for code that is
// trusted: it does not fall back to the machine. (The system that has no sandbox is made here: every runner of CI has one.)
func TestExtractRustWithoutTheSandboxStopsOnLinux(t *testing.T) {
	old := sandboxAvailable
	sandboxAvailable = func() error { return fmt.Errorf("%w: Landlock is not there", sandbox.ErrUnavailable) }
	t.Cleanup(func() { sandboxAvailable = old })
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.lock"), []byte("version = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RIVUS_UNSAFE_HOST_EXEC", "")
	err := runExtract([]string{"-lang", "rust", "-C", dir})
	if err == nil || !strings.Contains(err.Error(), "-unsafe-host-exec") || !strings.Contains(err.Error(), "nothing was run") || !strings.Contains(err.Error(), "Landlock is not there") {
		t.Fatalf("want the error that says nothing was run, why, and names -unsafe-host-exec, got %v", err)
	}
	if sandboxUsable() == nil {
		t.Error("sandboxUsable does not say what sandboxAvailable says")
	}
}
