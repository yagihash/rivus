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
)

// rivus setup rust: prepares what the extraction of Rust needs, outside the sandbox, from trusted sources: the pinned toolchain (from the Rust
// project, with rustup) and the extractor (built from the source of rivus). It is a command of its own, run on purpose: the commands that
// read a project never download or build anything for themselves.
func runSetup(argv []string) error {
	if len(argv) == 0 || argv[0] != "rust" {
		return fmt.Errorf("usage: rivus setup rust [-source <directory of extractor/rust>] [-y]")
	}
	fs := flag.NewFlagSet("setup rust", flag.ContinueOnError)
	source := fs.String("source", "", "the directory of the source of the Rust extractor (extractor/rust of the repository of rivus). Required: it is built and run on this machine, outside the sandbox, so it is never taken from the directory you are in (that may be a project that you are reading)")
	yes := fs.Bool("y", false, "do not ask before the download")
	if err := fs.Parse(argv[1:]); err != nil {
		return err
	}
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("rivus setup rust prepares the extraction under the sandbox of macOS, which this system does not have: a Rust project that you trust is read with -unsafe-host-exec, with rivus-rust that you built yourself (extractor/rust/README.md)")
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
	fmt.Fprintf(os.Stderr, "rivus: building the extractor from %s into %s (cargo install --locked)\n", src, root)
	cmd := exec.Command(filepath.Join(tc, "bin", "cargo"), "install", "--path", src, "--locked", "--root", root, "--force")
	// what cargo needs and nothing else of this environment (not the variables that a repository you are in could have set up)
	home, _ := os.UserHomeDir()
	cmd.Env = []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "HOME=" + home, "RUSTUP_TOOLCHAIN=" + pinnedToolchain, "TMPDIR=" + os.TempDir()}
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
