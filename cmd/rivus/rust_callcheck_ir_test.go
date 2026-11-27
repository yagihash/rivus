package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSaveRustIRForCallcheck writes the IR of bench/rust-curated and of the whole Rust zoo to the directory that
// RIVUS_SAVE_IR names (curated.ir.jsonl, zoo-rust.ir.jsonl), the input of bench/rust-callcheck/callcheck.py (B(3) of
// docs/ROADMAP.md). It checks nothing else, and is skipped without RIVUS_SAVE_IR and RIVUS_RUST; the extractor is the
// one of RIVUS_RUST, run only here (the same way as TestCuratedRust and TestZooRust).
func TestSaveRustIRForCallcheck(t *testing.T) {
	out := os.Getenv("RIVUS_SAVE_IR")
	if out == "" {
		t.Skip("RIVUS_SAVE_IR is not set")
	}
	if os.Getenv("RIVUS_RUST") == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	curated, _ := rustCuratedStream(t)
	if err := os.WriteFile(filepath.Join(out, "curated.ir.jsonl"), curated, 0o644); err != nil {
		t.Fatal(err)
	}
	// the whole zoo as one program (no analysis: the extraction does not need the cells split)
	zoo := zooRustStream(t, zooRustCopy(t, zooRustRoot(t)))
	if err := os.WriteFile(filepath.Join(out, "zoo-rust.ir.jsonl"), zoo, 0o644); err != nil {
		t.Fatal(err)
	}
}
