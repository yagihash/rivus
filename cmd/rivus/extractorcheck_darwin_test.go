//go:build darwin

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/sandbox"
)

// The version of the Rust extractor is asked under the sandbox that reads a project (ADR-126): the same profile, an environment of PATH, HOME and
// TMPDIR only, nothing written, no network. The fake extractors are scripts (they need /bin/sh, and /bin/bash that it runs as: execExtra).
func versionIso(t *testing.T, script string) *isolationOpts {
	t.Helper()
	if err := sandbox.Available(); err != nil {
		t.Skipf("the sandbox is not available here: %v", err)
	}
	tc := t.TempDir() // the toolchain of the call is not used by a script: its lib is only read
	os.MkdirAll(filepath.Join(tc, "lib"), 0o755)
	t.Setenv("RIVUS_RUST_TOOLCHAIN_DIR", tc)
	return &isolationOpts{extractor: []string{script}, execExtra: []string{"/bin/sh", "/bin/bash"}}
}

func TestSandboxedVersionCallPassesNoDriverEnvironment(t *testing.T) {
	withDriverInEnvironment(t)
	iso := versionIso(t, fakeExtractor(t, "rivus-rust", newExtractorBody))
	out, err := iso.rustExtractorVersionSandboxed(context.Background())
	if err != nil || strings.TrimSpace(out) != "rivus-rust rivus.ir/1 source=abc123" {
		t.Fatalf("new extractor, RIVUS_RUST_DRIVER in the environment: %q, %v: the call must not pass it on", out, err)
	}
	iso = versionIso(t, fakeExtractor(t, "rivus-rust", oldExtractorBody))
	if _, err := iso.rustExtractorVersionSandboxed(context.Background()); err == nil || !strings.Contains(err.Error(), "exit 2") || !strings.Contains(err.Error(), "--rivus-version") {
		t.Errorf("old extractor: %v, want an error that says exit 2 and what it said", err)
	}
}

// The call writes nothing: the write of a program that tries is refused.
func TestSandboxedVersionCallWritesNothing(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "mark")
	iso := versionIso(t, fakeExtractor(t, "rivus-rust", `echo x > `+outside+`; echo "rivus-rust rivus.ir/1 source=abc"`))
	iso.rustExtractorVersionSandboxed(context.Background()) // (the script goes on after the write that is refused)
	if _, err := os.Stat(outside); err == nil {
		t.Error("the call wrote a file outside its directory")
	}
}

func TestSandboxedVersionCallTimesOut(t *testing.T) {
	old := rustVersionTimeout
	defer func() { rustVersionTimeout = old }()
	rustVersionTimeout = 300 * time.Millisecond
	iso := versionIso(t, fakeExtractor(t, "rivus-rust", "exec /bin/sleep 30"))
	iso.execExtra = append(iso.execExtra, "/bin/sleep")
	start := time.Now()
	_, err := iso.rustExtractorVersionSandboxed(context.Background())
	if err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Errorf("an extractor that does not answer: %v, want an error that says so", err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("took %v", d)
	}
}

// Through checkExtractor (what the run and `rivus version` call): the sandbox is the default, and the states come out the same. The extractor is a
// program (not a script: a script needs /bin/sh, which the call does not give to an extractor).
func TestCheckExtractorUnderTheSandbox(t *testing.T) {
	withDriverInEnvironment(t)
	oldHash := rustSourceHash
	defer func() { rustSourceHash = oldHash }()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	prog := `package main
import ("fmt";"os")
func main() {
	if len(os.Args) != 2 || os.Args[1] != "--rivus-version" || os.Getenv("RIVUS_RUST_DRIVER") != "" || os.Getenv("SECRET_OF_THE_USER") != "" { os.Exit(3) }
	fmt.Println("rivus-rust rivus.ir/1 source=abc123")
}
`
	if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "rivus-rust")
	if out, err := exec.Command("go", "build", "-o", exe, src).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	versionIso(t, exe) // (the toolchain directory, and the sandbox)
	for _, c := range []struct{ expected, match string }{{"abc123", matchYes}, {"zzz", matchNo}, {"unknown", matchUnknownBinary}} {
		rustSourceHash = c.expected
		o := &analyzeOpts{rustExtractor: exe}
		got, ok := o.checkExtractor(langRust)
		if !ok || got.Match != c.match {
			t.Errorf("expected %q: %+v, want %q", c.expected, got, c.match)
		}
	}
}

// The real extractor links to the libraries of the toolchain: the call under the sandbox must be able to read them, and gets the line that it
// answers without the sandbox.
func TestRealRustExtractorAnswersUnderTheSandbox(t *testing.T) {
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	if err := sandbox.Available(); err != nil {
		t.Skipf("the sandbox is not available here: %v", err)
	}
	if _, err := pinnedToolchainDir(); err != nil {
		t.Skipf("the pinned toolchain is not there: %v", err)
	}
	bin, _ = filepath.Abs(bin)
	direct, err := exec.Command(bin, "--rivus-version").Output()
	if err != nil {
		t.Fatal(err)
	}
	withDriverInEnvironment(t)
	got, err := (&isolationOpts{extractor: []string{bin}}).rustExtractorVersionSandboxed(context.Background())
	if err != nil || got != string(direct) {
		t.Errorf("under the sandbox: %q, %v; without it: %q", got, err, direct)
	}
}
