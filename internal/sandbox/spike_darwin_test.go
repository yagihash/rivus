//go:build darwin

package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A spike (not committed): the extractor of Rust under the sandbox, to see what it needs.
func TestSpikeRustExtract(t *testing.T) {
	if os.Getenv("RIVUS_SBX_SPIKE") == "" {
		t.Skip()
	}
	home, _ := os.UserHomeDir()
	tc := filepath.Join(home, ".rustup", "toolchains", "nightly-2026-11-15-aarch64-apple-darwin")
	extractor, _ := filepath.Abs("../../extractor/rust/target/debug/rivus-rust")
	proj := os.Getenv("RIVUS_SBX_SPIKE_PROJECT")
	if proj == "" {
		proj = "../../bench/zoo-rust"
	}
	project, _ := filepath.Abs(proj)
	work := t.TempDir()
	cargoHome := filepath.Join(home, ".cargo")
	extra := strings.Fields(os.Getenv("RIVUS_SBX_SPIKE_EXTRA"))
	res, err := Run(context.Background(), Spec{
		Argv: []string{extractor, "--locked", "--offline", project},
		Dir:  project,
		Env: []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "RIVUS_RUST_DIRECT=1", "HOME=" + work, "TMPDIR=" + work, "CARGO_HOME=" + cargoHome,
			"RUSTUP_HOME=" + filepath.Join(home, ".rustup"), "RUSTUP_TOOLCHAIN=nightly-2026-11-15"},
		Read:      append([]string{project, filepath.Join(cargoHome, "registry"), filepath.Join(home, ".rustup")}, extra...),
		Exec:      []string{tc, "/Library/Developer/CommandLineTools"},
		ExecFiles: append([]string{"/bin/sh", "/usr/bin/cc", "/usr/bin/clang", "/usr/bin/ld", "/usr/bin/ar", "/usr/bin/xcrun", "/usr/bin/xcode-select"}, strings.Fields(os.Getenv("RIVUS_SBX_SPIKE_EXEC"))...),
		Write:     []string{work},
		Timeout:   10 * time.Minute,
	})
	fmt.Println("err:", err)
	if res != nil {
		os.WriteFile(os.Getenv("RIVUS_SBX_SPIKE_OUT"), res.Stdout, 0o644)
		fmt.Println("exit:", res.ExitCode, "stdout bytes:", len(res.Stdout))
		e := string(res.Stderr)
		if len(e) > 2500 {
			e = e[:2500]
		}
		fmt.Println("stderr tail:\n" + e)
	}
}
