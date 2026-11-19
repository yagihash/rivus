package main

import (
	"bytes"

	"github.com/yagihash/rivus/internal/ir"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The Rust extractor builds a sample crate with its own driver in place of
// rustc and writes the neutral IR of what rustc made of it. It needs the
// nightly toolchain it is pinned to: RIVUS_RUST names the extractor binary
// (extractor/rust: cargo build), for example extractor/rust/target/debug/rivus-rust.
// On a Mac whose newest SDK the linker cannot read, set SDKROOT to an older one.
func TestRustExtractor(t *testing.T) {
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		t.Skip("RIVUS_RUST is not set")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../extractor/rust/test/sample")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, root)
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v", err)
	}
	checkSampleFlows(t, stream, filepath.Join(root, "rules.yaml"), root, []string{"src/lib.rs", "src/std_flows.rs", "src/traits.rs", "src/async_flows.rs", "src/unsafe_flows.rs", "src/global_flows.rs", "src/review_flows.rs"}, 51)

	// What the extractor could not express is said in the IR (REVIEW-2026-11-19, the rule
	// that nothing is left out without a trace): the sample has nothing of the kind, and a
	// call of a foreign function says that it leaves the language.
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	foreign := false
	for _, f := range prog.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpUnsupported {
					t.Errorf("%s: unsupported: %s", f.ID, in.Reason)
				}
				if in.Op == ir.OpCall && in.Callee != nil && in.Callee.FFI && f.ID == "sample::review_flows::foreign_call" {
					foreign = true
				}
			}
		}
	}
	if !foreign {
		t.Error("the call of a foreign function is not marked ffi")
	}
}

// A toolchain that is not there is said as that, not as code that does not compile
// (yagihash/rivus #49): a rustc that fails stands for the missing nightly.
func TestRustExtractorMissingToolchain(t *testing.T) {
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		t.Skip("RIVUS_RUST is not set")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../extractor/rust/test/sample")
	if err != nil {
		t.Fatal(err)
	}
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "rustc"), []byte("#!/bin/sh\necho 'error: toolchain is not installed' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, root)
	cmd.Env = append(os.Environ(), "PATH="+fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("the extractor succeeded without its toolchain")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("rustup toolchain install")) || bytes.Contains(stderr.Bytes(), []byte("the code has to compile")) {
		t.Errorf("the message does not say the toolchain is missing: %s", stderr.String())
	}
}

// The nightly is written in one place, rust-toolchain.toml: the extractor reads it when it is built, and
// the install hint of the command must name the same one.
func TestRustToolchainHintMatches(t *testing.T) {
	b, err := os.ReadFile("../../extractor/rust/rust-toolchain.toml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`channel\s*=\s*"([^"]+)"`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no channel in rust-toolchain.toml")
	}
	o := &analyzeOpts{rustExtractor: "rivus-rust-does-not-exist-xyz"}
	_, err = o.extractorCommand(langRust)
	if err == nil || !strings.Contains(err.Error(), string(m[1])) {
		t.Errorf("the install hint does not name %s: %v", m[1], err)
	}
}

// With --include-tests the library is built twice (as it is, and as its tests): they are one
// crate, not two versions of it, and a function that the tests change (the closures and impls that
// its calls may reach) is not a conflict (yagihash/rivus #68).
func TestRustExtractorIncludeTestsIsNotAConflict(t *testing.T) {
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		t.Skip("RIVUS_RUST is not set")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, text := range map[string]string{
		"Cargo.toml": "[package]\nname = \"inctest\"\nversion = \"0.0.0\"\nedition = \"2021\"\n\n[workspace]\n",
		"src/lib.rs": "pub fn run<F: FnOnce()>(f: F) { f() }\n\npub fn go() { run(|| {}); }\n\n#[cfg(test)]\nmod tests {\n    #[test]\n    fn t() { super::run(|| { let _ = 1; }); }\n}\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(bin, "--include-tests", dir)
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v", err)
	}
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range prog.LoadErrors {
		t.Errorf("load_error: %s: %s", e.Package, e.Message)
	}
}
