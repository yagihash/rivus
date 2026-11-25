package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/extractorhash"
	"github.com/yagihash/rivus/internal/ir"
)

// The extractors say which source they were built from: `source=<hash>` after the schema version, the hash that scripts/extractor-hash prints
// for the tree (ADR-126). A binary that was built from an older tree, or without the hash, says another value.
func wantVersionLine(t *testing.T, lang, name string) string {
	t.Helper()
	h, err := extractorhash.Compute(lang, filepath.Join("..", "..", "extractor", lang))
	if err != nil {
		t.Fatal(err)
	}
	return name + " " + ir.SchemaVersion + " source=" + h
}

func TestRustExtractorSaysItsSourceHash(t *testing.T) {
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	cmd := exec.Command(bin, "--rivus-version")
	// RIVUS_RUST_DRIVER makes rivus-rust act as rustc: the answer must not depend on it (a rivus that is run by a build, or a stale value in the environment)
	cmd.Env = append(os.Environ(), "RIVUS_RUST_DRIVER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(out))
	if strings.HasSuffix(got, " source=unknown") {
		t.Skip("this rivus-rust was built without RIVUS_EXTRACTOR_SRC_HASH (make test builds it with the hash: see the Makefile)")
	}
	if want := wantVersionLine(t, "rust", "rivus-rust"); got != want {
		t.Errorf("rivus-rust --rivus-version = %q, want %q (build it again with RIVUS_EXTRACTOR_SRC_HASH=$(go run ./scripts/extractor-hash rust))", got, want)
	}
}

func TestDartExtractorSaysItsSourceHash(t *testing.T) {
	if dartReal == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	if dartExePath == "" {
		t.Skip("the extractor runs from source in this run (no -D)")
	}
	out, err := exec.Command(dartExePath, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(out))
	if want := wantVersionLine(t, "dart", "rivus_dart"); got != want {
		t.Errorf("rivus_dart --version = %q, want %q", got, want)
	}
}
