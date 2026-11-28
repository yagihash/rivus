package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yagihash/rivus/internal/extractorhash"
)

// rivus setup rust: prepares what the extraction of Rust needs, outside the sandbox, from trusted sources: the pinned toolchain (from the Rust
// project, with rustup) and the extractor (built from the source of rivus). It is a command of its own, run on purpose: the commands that
// read a project never download or build anything for themselves.
// setupRustCmd is "rivus setup rust".
type setupRustCmd struct {
	source *string
	yes    *bool
}

func setupRustFlags(fs *flag.FlagSet) *setupRustCmd {
	c := &setupRustCmd{}
	c.source = fs.String("source", "", "the directory of the source of the Rust extractor (extractor/rust of the repository of rivus). Required: it is built and run on this machine, outside the sandbox, so it is never taken from the directory you are in (that may be a project that you are reading)")
	c.yes = fs.Bool("y", false, "do not ask before the download")
	usual(fs, "source", "y")
	kind(fs, "dir", "source")
	return c
}

func runSetupRust(argv []string) error {
	fs := flag.NewFlagSet("setup rust", flag.ContinueOnError)
	c := setupRustFlags(fs)
	prepareFlags(fs, argv)
	if err := fs.Parse(argv); err != nil {
		return err
	}
	source, yes := c.source, c.yes
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("rivus setup rust prepares the extraction under the sandbox of macOS or Linux, which this system does not have: a Rust project that you trust is read with -unsafe-host-exec, with rivus-rust that you built yourself (extractor/rust/README.md)")
	}
	data, err := rustDataDir()
	if err != nil {
		return err
	}
	// 1. the pinned toolchain
	if _, err := pinnedToolchainDir(); err != nil {
		rustup, lerr := exec.LookPath("rustup")
		if lerr != nil {
			return fmt.Errorf("the pinned toolchain is not there, and rustup is not (%v): install rustup first (brew install rustup, or https://rustup.rs), then run this again", lerr)
		}
		fmt.Fprintf(os.Stderr, "rivus: the pinned toolchain %s is not there. It will be downloaded from the Rust project by rustup (about 1 GB, into ~/.rustup): %s toolchain install %s --profile minimal -c rustc-dev -c llvm-tools -c rust-src\n", pinnedToolchain, rustup, pinnedToolchain)
		if !*yes && !confirm("download it now? [y/N] ") {
			return fmt.Errorf("not downloaded: run rivus setup rust again with -y when you want it")
		}
		cmd := exec.Command(rustup, "toolchain", "install", pinnedToolchain, "--profile", "minimal", "-c", "rustc-dev", "-c", "llvm-tools", "-c", "rust-src")
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("rustup toolchain install %s: %v", pinnedToolchain, err)
		}
	}
	tc, err := pinnedToolchainDir()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "rivus: the toolchain is there: %s\n", tc)
	// 2. the extractor, built from the source of rivus (trusted), into the directory of rivus
	src := *source
	if src == "" {
		return fmt.Errorf("-source is required: the directory of extractor/rust of the repository of rivus. It is built and run here, outside the sandbox, so it is not taken from the current directory (which may hold a project that you are reading)")
	}
	if _, err := os.Stat(filepath.Join(src, "Cargo.toml")); err != nil {
		return fmt.Errorf("the source of the Rust extractor is not at %s: name it with -source (the extractor/rust directory of the repository of rivus)", src)
	}
	src, _ = filepath.Abs(src)
	root := filepath.Join(data, "extractor")
	hash, err := extractorhash.Compute("rust", src)
	if err != nil {
		return fmt.Errorf("the hash of the source of the extractor: %v", err)
	}
	fmt.Fprintf(os.Stderr, "rivus: building the extractor from %s into %s (cargo install --locked)\n", src, root)
	cmd := exec.Command(filepath.Join(tc, "bin", "cargo"), "install", "--path", src, "--locked", "--root", root, "--force")
	// cargo reads .cargo/config.toml of the directory it runs in and of its parents (a rustc-wrapper or a linker in it runs here,
	// outside the sandbox): run it in the source of rivus, not in the directory that the user is in (it may be a project that is read)
	cmd.Dir = src
	// what cargo needs and nothing else of this environment (not the variables that a repository you are in could have set up)
	home, _ := os.UserHomeDir()
	cmd.Env = []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "HOME=" + home, "RUSTUP_TOOLCHAIN=" + pinnedToolchain, "TMPDIR=" + os.TempDir(), "RIVUS_EXTRACTOR_SRC_HASH=" + hash}
	if sdk := sdkRoot(); sdk != "" {
		cmd.Env = append(cmd.Env, "SDKROOT="+sdk)
	}
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cargo install: %v", err)
	}
	fmt.Fprintf(os.Stderr, "rivus: done: %s\n", filepath.Join(root, "bin", "rivus-rust"))
	return nil
}

func confirm(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y")
}
