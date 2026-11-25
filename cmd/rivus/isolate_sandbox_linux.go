//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yagihash/rivus/internal/sandbox"
)

// On Linux a Rust project that is not trusted is read under the sandbox of the system (Landlock and seccomp: internal/sandbox,
// docs/DESIGN-linux-sandbox.md), in the same two stages as on macOS (isolate_sandbox_darwin.go): the fetch (cargo fetch --locked, with a network, the
// cargo configuration of the project not read) and the extraction (no network, the project and the fetched sources read-only).
// The dependencies are fetched into a CARGO_HOME of rivus (not ~/.cargo, which is never read nor written): the fetch writes it, the
// extraction only reads it. The toolchain is the pinned nightly, run directly (no rustup), prepared outside the sandbox
// from a trusted source.

// sandboxDesc says which sandbox this system has, for what is said to the user.
const sandboxDesc = "the sandbox of Linux (Landlock and seccomp)"

// rustDataDir is where rivus keeps what it prepares for Rust (the dependencies that were fetched).
// RIVUS_RUST_DATA_DIR names another directory for it (the tests of rivus use it so that they write nothing to the directory of the user).
func rustDataDir() (string, error) {
	if d := os.Getenv("RIVUS_RUST_DATA_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir() // ~/.config
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "rivus", "rust"), nil
}

// pinnedToolchainDir is the directory of the pinned nightly: RIVUS_RUST_TOOLCHAIN_DIR, or the one that rustup made (in RUSTUP_HOME, or ~/.rustup; for the
// architecture of this machine, and whichever C library rustup chose: gnu or musl).
func pinnedToolchainDir() (string, error) {
	if d := os.Getenv("RIVUS_RUST_TOOLCHAIN_DIR"); d != "" {
		return d, nil
	}
	rustup := os.Getenv("RUSTUP_HOME")
	if rustup == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		rustup = filepath.Join(home, ".rustup")
	}
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	if arch == "" {
		return "", fmt.Errorf("the sandbox of Linux is made for x86-64 and arm64 only (this is %s)", runtime.GOARCH)
	}
	ms, _ := filepath.Glob(filepath.Join(rustup, "toolchains", pinnedToolchain+"-"+arch+"-unknown-linux-*"))
	for _, d := range ms {
		if _, err := os.Stat(filepath.Join(d, "bin", "rustc")); err == nil {
			return d, nil
		}
	}
	return "", fmt.Errorf("the pinned toolchain %s is not there (looked for %s in %s): prepare it with `rustup toolchain install %s --profile minimal -c rustc-dev -c llvm-tools -c rust-src` (it is downloaded from the Rust project, outside the sandbox, and never from the code that is read), or name the directory with RIVUS_RUST_TOOLCHAIN_DIR", pinnedToolchain, pinnedToolchain+"-"+arch+"-unknown-linux-*", filepath.Join(rustup, "toolchains"), pinnedToolchain)
}

// sandboxPrograms are the programs that a build may run, each by its path (not the directories /usr/bin and /bin: curl, ssh, python ...
// are there). The C compiler runs the assembler and the linker by their names, and cc1 and collect2 from /usr/lib/gcc (the directory is
// run from: see sandboxExecDirs).
var sandboxPrograms = []string{"/bin/sh", "/bin/bash", "/usr/bin/env", "/usr/bin/cc", "/usr/bin/gcc", "/usr/bin/c++", "/usr/bin/g++", "/usr/bin/clang",
	"/usr/bin/ld", "/usr/bin/ld.bfd", "/usr/bin/ld.gold", "/usr/bin/ld.lld", "/usr/bin/as", "/usr/bin/ar", "/usr/bin/ranlib", "/usr/bin/strip", "/usr/bin/nm", "/usr/bin/objcopy"}

// sandboxExecDirs are the directories (besides the toolchain) that programs are run from: the parts of the C compiler that the driver runs.
// (A directory that this machine does not have is left out by the sandbox.) /usr/lib itself is only read (apart from /usr/lib/gcc and the dynamic loader, which are named): the programs that a system keeps there (the Rust coreutils of Ubuntu are in /usr/lib/cargo/bin) are not run.
var sandboxExecDirs = []string{"/usr/libexec", "/usr/lib/gcc", "/usr/lib64/gcc"}

// sandboxReadDirs are what a build reads besides the project and the dependencies: the headers of the system (the build scripts compile C).
var sandboxReadDirs = []string{"/usr/include", "/usr/local/include", "/usr/share/zoneinfo"}

// sdkRoot: there is no SDK on Linux.
func sdkRoot() string { return "" }

// extractRustSandboxed reads the project under the sandbox and returns the IR. It runs nothing on this machine outside the sandbox.
func (o *isolationOpts) extractRustSandboxed(ctx context.Context, dir string, extractorArgs []string) ([]byte, error) {
	return o.extractRustSandboxedTo(ctx, dir, extractorArgs, nil)
}

// extractRustSandboxedTo is extractRustSandboxed with the standard output of the extractor going to w as it is
// written (nil: it is returned).
func (o *isolationOpts) extractRustSandboxedTo(ctx context.Context, dir string, extractorArgs []string, w io.Writer) ([]byte, error) {
	// whether the sandbox can be made is found out here, before any code of the project runs, by rivus itself (a program that does nothing, under the
	// profile): never from what the project's own programs write to their standard error
	if err := sandbox.Available(); err != nil {
		return nil, noSandboxError(err)
	}
	if err := mountable(dir); err != nil {
		return nil, err
	}
	volume, err := cargoVolume(dir) // the name of the set of dependencies (the hash of Cargo.lock and the configuration)
	if err != nil {
		return nil, err
	}
	if want := projectToolchain(dir); want != "" && want != pinnedToolchain {
		fmt.Fprintf(o.err(), "rivus: the project asks for the toolchain %q; the extractor is pinned to %s and the sandbox does not fetch another one, so it is built with %s (it may not compile)\n", want, pinnedToolchain, pinnedToolchain)
	}
	if above := cargoConfigsAbove(dir); len(above) > 0 {
		return nil, fmt.Errorf("cargo reads the configuration of every directory above the one it is run in, and the sandbox does not let it read %s (a configuration outside the project could change the build: a rustc-wrapper is a program that cargo runs instead of rustc). Nothing was run. Move the project to a place without a .cargo/config(.toml) above it, or, if the code and the configuration are yours, use -unsafe-host-exec", strings.Join(above, ", "))
	}
	tc, err := pinnedToolchainDir()
	if err != nil {
		return nil, err
	}
	extractor, err := o.rustExtractorPath()
	if err != nil {
		return nil, err
	}
	data, err := rustDataDir()
	if err != nil {
		return nil, err
	}
	cargoHome := filepath.Join(data, volume)
	o.developerDir = nil // (no developer directory on Linux)
	env := func(work string) []string {
		e := []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "HOME=" + work, "TMPDIR=" + work, "CARGO_HOME=" + cargoHome,
			"RIVUS_RUST_DIRECT=1", "RUSTUP_TOOLCHAIN=" + pinnedToolchain}
		return append(e, o.env...)
	}
	common := sandbox.Spec{
		Exec:      append([]string{tc}, sandboxExecDirs...),
		ExecFiles: sandboxPrograms,
		Timeout:   o.timeout,
	}
	common.Read = append(append([]string(nil), sandboxReadDirs...), o.read...)

	if !o.noFetch {
		plan, err := planFetch(dir)
		if err != nil {
			return nil, err
		}
		switch {
		case plan.vendored:
			fmt.Fprintf(o.err(), "rivus: the project is vendored (its cargo configuration replaces the sources with a directory of the project): the fetch stage is skipped\n")
		default:
			lock, err := os.ReadFile(filepath.Join(dir, "Cargo.lock"))
			if err != nil {
				return nil, err
			}
			if err := checkFetchSources(string(lock), o.allowHosts); err != nil {
				return nil, err
			}
			if err := checkManifestSources(dir, o.allowHosts); err != nil {
				return nil, err
			}
			if err := dropConfigOfCargoHome(cargoHome, o.err()); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(cargoHome, 0o755); err != nil {
				return nil, err
			}
			work, err := os.MkdirTemp("", "rivus-fetch-")
			if err != nil {
				return nil, err
			}
			defer os.RemoveAll(work)
			fmt.Fprintf(o.err(), "rivus: fetching the dependencies (cargo fetch --locked, under the sandbox, with a network; the cargo configuration of the project is not read in this stage) into %s\n", cargoHome)
			s := fetchSpec(common, tc, dir, work, cargoHome, env(work))
			res, err := sandbox.Run(ctx, s)
			if err := o.sandboxResult("fetch", res, err); err != nil {
				if plan.hasConfig {
					err = fmt.Errorf("%w\n(the project has a cargo configuration, and it is not used in the stage that fetches. If the project needs it to fetch, fetch it yourself or vendor it, then use -no-fetch)", err)
				}
				return nil, err
			}
		}
	}
	if _, err := os.Stat(cargoHome); err != nil {
		if plan, perr := planFetch(dir); perr == nil && plan.vendored {
			// nothing was fetched: the sources are in the project
		} else {
			return nil, fmt.Errorf("the dependencies are not fetched (%s): run without -no-fetch once", cargoHome)
		}
	}
	work, err := os.MkdirTemp("", "rivus-extract-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	s := common
	s.Argv = append([]string{extractor, "--locked", "--offline"}, extractorArgs...)
	s.Dir = dir
	s.Env = env(work)
	s.Read = append(append([]string(nil), common.Read...), dir, cargoHome)
	s.ExecFiles = append(append(append([]string(nil), sandboxPrograms...), extractor), o.execExtra...)
	s.Write = []string{work}
	s.MaxOutput = o.maxOutput
	s.Stdout = w
	if s.MaxOutput <= 0 {
		s.MaxOutput = 4 << 30
	}
	fmt.Fprintf(o.err(), "rivus: extracting rust under the sandbox of Linux (Landlock and seccomp: no network, the project and the dependencies read-only): %s\n", strings.Join(s.Argv, " "))
	res, err := sandbox.Run(ctx, s)
	if err := o.sandboxResult("extract", res, err); err != nil {
		return nil, err
	}
	return res.Stdout, nil
}

// rustExtractorVersionSandboxed asks the extractor which source it was built from (`--rivus-version`), under the same sandbox that reads a project, and
// returns what it printed (ADR-126). It is a light call of its own, not the extraction with other arguments: the environment is PATH, HOME and TMPDIR
// and nothing else (RIVUS_RUST_DRIVER is not passed on; the answer does not depend on it), the extractor and the libraries of the toolchain are read,
// nothing is written, there is no network and no standard input, and the working directory is an empty directory that is removed. It does not wait
// longer than rustVersionTimeout. An error is returned for the reasons that the answer is not there (errNoIsolation when the sandbox cannot be made).
func (o *isolationOpts) rustExtractorVersionSandboxed(ctx context.Context) (string, error) {
	if err := sandbox.Available(); err != nil {
		return "", fmt.Errorf("%w (%v)", errNoIsolation, err)
	}
	tc, err := pinnedToolchainDir()
	if err != nil {
		return "", err
	}
	extractor, err := o.rustExtractorPath()
	if err != nil {
		return "", err
	}
	work, err := os.MkdirTemp("", "rivus-version-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	res, err := sandbox.Run(ctx, sandbox.Spec{
		Argv:      []string{extractor, "--rivus-version"},
		Dir:       work,
		Env:       []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "HOME=" + work, "TMPDIR=" + work},
		Read:      []string{filepath.Join(tc, "lib"), work}, // (librustc_driver, which the extractor links to)
		ExecFiles: append([]string{extractor}, o.execExtra...),
		Timeout:   rustVersionTimeout,
		MaxOutput: 4096,
	})
	if err != nil {
		if errors.Is(err, sandbox.ErrUnavailable) {
			return "", fmt.Errorf("%w (%v)", errNoIsolation, err)
		}
		return "", fmt.Errorf("it could not be run under the sandbox: %v", err)
	}
	if res.TimedOut {
		return "", fmt.Errorf("it did not answer in %v and was stopped", rustVersionTimeout)
	}
	if res.OutputTooBig {
		return "", fmt.Errorf("it wrote more than 4096 bytes and was stopped")
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("it failed (exit %d)%s", res.ExitCode, tailOf(string(res.Stderr)))
	}
	return string(res.Stdout), nil
}

// tellAboutRun says what the run did that the user must hear, whether the stage worked or not: the warnings (a kernel that cannot close something), the files outside of
// the work directory whose attributes the build tried to change (refused: this is where the user hears of it, and a build script that tries it and then fails, or runs out of time, did try it all the same), and the processes that may have been left.
func (o *isolationOpts) tellAboutRun(stage string, res *sandbox.Result) {
	if stage != "extract" || res == nil {
		return
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(o.err(), "rivus: warning: %s\n", w)
	}
	if len(res.AttributeChanges) > 0 {
		// Landlock does not stop these, so rivus does them for the build, and only for the files inside of its work directory: these were refused (EPERM)
		fmt.Fprintf(o.err(), "rivus: warning: the build tried to change the mode, the owner, the times or the attributes of files outside of its work directory, and the sandbox refused (EPERM). Look at them if the project is not one you trust:\n")
		for _, c := range res.AttributeChanges {
			fmt.Fprintf(o.err(), "  %s\n", c)
		}
	}
	if res.MayHaveLeftProcesses {
		// said once, not hidden: a build script can start a process in a session of its own, which the end of the run does not reach
		fmt.Fprintf(o.err(), "rivus: a build script may have left a process behind (a program that starts one in a session of its own is not stopped by the end of the run; it stays under the sandbox, with no files and no network). Look with `ps` if the project is not one you trust.\n")
	}
}

// sandboxResult says how a stage ended: what it printed goes to the user, and a stage that failed or was stopped is an error. What the run did that the user must hear is
// said first (tellAboutRun), so that it is said when the stage failed too.
func (o *isolationOpts) sandboxResult(stage string, res *sandbox.Result, err error) error {
	o.tellAboutRun(stage, res)
	if err != nil {
		if errors.Is(err, sandbox.ErrUnavailable) {
			return noSandboxError(err)
		}
		return fmt.Errorf("the %s stage under the sandbox: %v", stage, err)
	}
	if res.TimedOut {
		return fmt.Errorf("the %s stage under the sandbox did not finish in %v and was stopped (-isolation-timeout)", stage, o.timeout)
	}
	if res.OutputTooBig {
		return fmt.Errorf("the %s stage under the sandbox wrote more than the output limit and was stopped (-isolation-max-output)", stage)
	}
	if len(res.Stderr) > 0 {
		o.err().Write(res.Stderr)
	}
	if res.ExitCode != 0 {
		tail := strings.TrimSpace(string(res.Stderr))
		if len(tail) > 4096 {
			tail = tail[len(tail)-4096:]
		}
		return fmt.Errorf("the %s stage under the sandbox failed (exit %d)\n%s", stage, res.ExitCode, tail)
	}
	return nil
}

// noSandboxError is said when the sandbox cannot be made: nothing is run.
func noSandboxError(why error) error {
	return fmt.Errorf("the extractor of Rust runs the build scripts and the macros of the project, and it is run under the sandbox of Linux (Landlock and seccomp) so that they cannot reach the files and the network of this machine; it cannot be made here (%v), so nothing was run. If the code is yours or you trust it, -unsafe-host-exec runs it directly on this machine, without any isolation (the build scripts of the project run here, with your rights)", why)
}

// dropConfigOfCargoHome removes the cargo configuration and the credentials that an earlier run could have left in the CARGO_HOME: cargo
// reads $CARGO_HOME/config.toml in the stage that has a network.
func dropConfigOfCargoHome(cargoHome string, w interface{ Write([]byte) (int, error) }) error {
	ms, _ := filepath.Glob(filepath.Join(cargoHome, "config*"))
	cs, _ := filepath.Glob(filepath.Join(cargoHome, "credentials*"))
	for _, m := range append(ms, cs...) {
		fmt.Fprintf(w, "rivus: %s is removed (cargo would read it in the stage that has a network)\n", m)
		if err := os.RemoveAll(m); err != nil {
			return err
		}
	}
	return nil
}

// rustExtractorPath is the path of the extractor (-rust-extractor, RIVUS_RUST_EXTRACTOR, or rivus-rust on PATH).
func (o *isolationOpts) rustExtractorPath() (string, error) {
	if len(o.extractor) == 0 {
		return "", fmt.Errorf("the Rust extractor is not found: build it (cargo +%s install rivus, see extractor/rust/README.md) or name it with -rust-extractor / RIVUS_RUST_EXTRACTOR", pinnedToolchain)
	}
	p, err := exec.LookPath(o.extractor[0])
	if err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

// cargoConfigsAbove are the cargo configuration files in the directories above the project (the project's own .cargo is read: it is part of the
// project). cargo stops with "Operation not permitted" when it finds one that it may not read, so it is said before anything runs.
func cargoConfigsAbove(dir string) []string {
	var out []string
	for d := filepath.Dir(dir); d != "/" && d != "."; d = filepath.Dir(d) {
		for _, f := range []string{"config.toml", "config"} {
			p := filepath.Join(d, ".cargo", f)
			if _, err := os.Stat(p); err == nil {
				out = append(out, p)
			}
		}
	}
	return out
}

// fetchSpec is the run of the stage that fetches: cargo fetch --locked, with the network, from a work directory that has no cargo
// configuration (cargo looks for it from the directory it is run in, not from the manifest), the project read-only, the CARGO_HOME of rivus
// and the work directory the only places that are written.
func fetchSpec(common sandbox.Spec, tc, dir, work, cargoHome string, env []string) sandbox.Spec {
	s := common
	s.Argv = []string{filepath.Join(tc, "bin", "cargo"), "fetch", "--locked", "--manifest-path", filepath.Join(dir, "Cargo.toml")}
	s.Dir = work
	s.Env = env
	s.Read = append(append([]string(nil), common.Read...), dir)
	s.Write = []string{work, cargoHome}
	s.Network = true
	return s
}
