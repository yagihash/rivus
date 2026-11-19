package main

import (
	"bytes"

	"github.com/yagihash/rivus/internal/ir"
	"os"
	"os/exec"
	"path/filepath"
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
	checkSampleFlows(t, stream, filepath.Join(root, "rules.yaml"), root, []string{"src/lib.rs", "src/std_flows.rs", "src/traits.rs", "src/async_flows.rs", "src/unsafe_flows.rs", "src/global_flows.rs", "src/review_flows.rs"}, 40)

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
