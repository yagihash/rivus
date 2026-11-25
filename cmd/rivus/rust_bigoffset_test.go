package main

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The Rust extractor makes the offset of a position as line*100000+col, which does
// not fit in 32 bits from about the 21,475th line of a file. The steps of a path
// there keep their offset, so that each finds its text. The crate is made in a
// temporary directory (a file of 22,000 lines is not kept in the repository), and
// the test needs the extractor (RIVUS_RUST) like the other tests of it.
func TestRustStepsPastLine21475KeepTheirText(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	fn := func(name string) string {
		return "pub fn " + name + "() {\n    let a: Vec<String> = std::env::args().collect();\n    let _ = std::fs::File::create(&a[1]);\n}\n"
	}
	proj := t.TempDir()
	files := map[string]string{
		"Cargo.toml": "[package]\nname = \"big\"\nversion = \"0.0.0\"\nedition = \"2021\"\n\n[workspace]\n",
		"Cargo.lock": "version = 3\n\n[[package]]\nname = \"big\"\nversion = \"0.0.0\"\n",
		"src/lib.rs": fn("early") + strings.Repeat("\n", 22000) + fn("late"),
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(proj, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(proj, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"analyze", "-lang", "rust", "-C", proj, "-no-project-rules", "-yes", "-rust-extractor", bin}
	if runtime.GOOS != "darwin" {
		args = append(args, "-unsafe-host-exec")
	}
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), "RIVUS_RUST_EXTRACTOR="+bin)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, stderr.String())
	}
	var rep struct {
		Findings []struct {
			Paths []struct {
				Steps []struct {
					Position struct {
						Offset int `json:"offset"`
					} `json:"position"`
					Snippet string `json:"snippet"`
				} `json:"steps"`
			} `json:"paths"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 2 {
		t.Fatalf("%d findings, want 2 (one before the line 21,475 and one after it)", len(rep.Findings))
	}
	large := 0
	for _, f := range rep.Findings {
		for _, p := range f.Paths {
			for _, s := range p.Steps {
				if s.Position.Offset < 0 || s.Snippet == "" {
					t.Errorf("step at offset %d has the snippet %q", s.Position.Offset, s.Snippet)
				}
				if s.Position.Offset > math.MaxInt32 {
					large++
				}
			}
		}
	}
	if large == 0 {
		t.Error("no step has an offset past 32 bits: the test checks nothing")
	}
}
