package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// The tests, examples and benchmarks of a crate are not built unless --include-tests asks, and the stream
// says how many targets were left out and which dependencies are declared for development only (ADR-062).
func TestRustExtractorSaysWhatItDidNotBuild(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, text := range map[string]string{
		"Cargo.toml":        "[package]\nname = \"skipped\"\nversion = \"0.0.0\"\nedition = \"2021\"\n\n[dev-dependencies]\n\n[workspace]\n",
		"src/lib.rs":        "pub fn go() {}\n",
		"tests/it.rs":       "#[test]\nfn t() { skipped::go(); }\n",
		"examples/demo.rs":  "fn main() { skipped::go(); }\n",
		"benches/bench1.rs": "fn main() { skipped::go(); }\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(args ...string) *ir.Program {
		out, err := exec.Command(bin, append(args, dir)...).Output()
		if err != nil {
			t.Fatalf("the extractor failed: %v", err)
		}
		prog, err := ir.Read(bytes.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		return prog
	}
	if ts := read().TestsSkipped; ts == nil || ts.Targets != 3 {
		t.Errorf("tests_skipped = %+v, want 3 targets (a test, an example, a benchmark)", ts)
	}
	if ts := read("--include-tests").TestsSkipped; ts != nil {
		t.Errorf("with --include-tests nothing is left out, but: %+v", ts)
	}
}
