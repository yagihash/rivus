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

// A spike: cargo fetch --locked under the sandbox, with the network, into a CARGO_HOME of its own.
func TestSpikeRustFetch(t *testing.T) {
	if os.Getenv("RIVUS_SBX_SPIKE") == "" {
		t.Skip()
	}
	home, _ := os.UserHomeDir()
	tc := filepath.Join(home, ".rustup", "toolchains", "nightly-2026-11-15-aarch64-apple-darwin")
	proj := os.Getenv("RIVUS_SBX_SPIKE_PROJECT")
	project, _ := filepath.Abs(proj)
	work := t.TempDir()
	ework := t.TempDir() // the work directory of the extraction: not the one that holds CARGO_HOME
	cargoHome := t.TempDir()
	extra := strings.Fields(os.Getenv("RIVUS_SBX_SPIKE_EXTRA"))
	mach := strings.Fields(os.Getenv("RIVUS_SBX_SPIKE_MACH"))
	res, err := Run(context.Background(), Spec{
		Argv:        []string{filepath.Join(tc, "bin", "cargo"), "fetch", "--locked"},
		Dir:         project,
		Env:         []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "HOME=" + work, "TMPDIR=" + work, "CARGO_HOME=" + cargoHome},
		Read:        append([]string{project}, extra...),
		Exec:        []string{tc, "/Library/Developer/CommandLineTools"},
		ExecFiles:   []string{"/bin/sh", "/usr/bin/cc", "/usr/bin/clang", "/usr/bin/ld", "/usr/bin/ar", "/usr/bin/xcrun", "/usr/bin/xcode-select"},
		Write:       []string{work, cargoHome},
		Network:     true,
		NetworkMach: mach,
		Timeout:     5 * time.Minute,
	})
	fmt.Println("err:", err)
	if res != nil {
		e := string(res.Stderr)
		if len(e) > 2500 {
			e = e[:2500]
		}
		fmt.Println("exit:", res.ExitCode, "stderr:\n"+e)
	}
	// the extraction, with the fetched sources read-only and no network
	extractor, _ := filepath.Abs("../../extractor/rust/target/debug/rivus-rust")
	res2, err2 := Run(context.Background(), Spec{
		Argv: []string{extractor, "--locked", "--offline", project},
		Dir:  project,
		Env: []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "HOME=" + ework, "TMPDIR=" + ework, "CARGO_HOME=" + cargoHome, "RIVUS_RUST_DIRECT=1", "SDKROOT=" + os.Getenv("RIVUS_SBX_SPIKE_SDK"),
			"RUSTUP_TOOLCHAIN=nightly-2026-11-15"},
		Read:      append([]string{project, cargoHome}, extra...),
		Exec:      []string{tc, "/Library/Developer/CommandLineTools"},
		ExecFiles: []string{"/bin/sh", "/usr/bin/cc", "/usr/bin/clang", "/usr/bin/ld", "/usr/bin/ar", "/usr/bin/xcrun", "/usr/bin/xcode-select"},
		Write:     []string{ework},
		Timeout:   20 * time.Minute,
	})
	fmt.Println("extract err:", err2)
	if res2 != nil {
		fmt.Println("extract exit:", res2.ExitCode, "stdout bytes:", len(res2.Stdout))
		if o := os.Getenv("RIVUS_SBX_SPIKE_OUT"); o != "" {
			os.WriteFile(o, res2.Stdout, 0o644)
		}
		e := string(res2.Stderr)
		if len(e) > 1500 {
			e = e[len(e)-1500:]
		}
		fmt.Println("extract stderr tail:\n" + e)
	}
	if ents, err := os.ReadDir(filepath.Join(cargoHome, "registry", "src")); err == nil {
		for _, e := range ents {
			sub, _ := os.ReadDir(filepath.Join(cargoHome, "registry", "src", e.Name()))
			fmt.Println("registry/src entries:", len(sub))
		}
	}
}
