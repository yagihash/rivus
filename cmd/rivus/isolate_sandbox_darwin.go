//go:build darwin

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/sandbox"
)

// On macOS a Rust project that is not trusted is read under the sandbox of the system (Seatbelt: internal/sandbox,
// docs/DESIGN-macos-sandbox.md), in the two stages that the container had: the fetch (cargo fetch --locked, with a network, the
// cargo configuration of the project not read) and the extraction (no network, the project and the fetched sources read-only).
// The dependencies are fetched into a CARGO_HOME of rivus (not ~/.cargo, which is never read nor written): the fetch writes it, the
// extraction only reads it. The toolchain is the pinned nightly, run directly (no rustup), prepared outside the sandbox
// from a trusted source.

// rustDataDir is where rivus keeps what it prepares for Rust (the dependencies that were fetched).
// RIVUS_RUST_DATA_DIR names another directory for it (the tests of rivus use it so that they write nothing to the directory of the user).
func rustDataDir() (string, error) {
	if d := os.Getenv("RIVUS_RUST_DATA_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir() // ~/Library/Application Support
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "rivus", "rust"), nil
}

// pinnedToolchainDir is the directory of the pinned nightly: RIVUS_RUST_TOOLCHAIN_DIR, or the one that rustup made.
func pinnedToolchainDir() (string, error) {
	if d := os.Getenv("RIVUS_RUST_TOOLCHAIN_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	arch := "aarch64"
	if runtime.GOARCH == "amd64" {
		arch = "x86_64"
	}
	d := filepath.Join(home, ".rustup", "toolchains", pinnedToolchain+"-"+arch+"-apple-darwin")
	if _, err := os.Stat(filepath.Join(d, "bin", "rustc")); err != nil {
		return "", fmt.Errorf("the pinned toolchain %s is not there (looked in %s): prepare it with `rustup toolchain install %s --profile minimal -c rustc-dev -c llvm-tools -c rust-src` (it is downloaded from the Rust project, outside the sandbox, and never from the code that is read), or name the directory with RIVUS_RUST_TOOLCHAIN_DIR", pinnedToolchain, d, pinnedToolchain)
	}
	return d, nil
}

// sandboxPrograms are the programs that a build may run, each by its path (not the directories /usr/bin and /bin: osascript,
// security, curl, ssh ... are there).
var sandboxPrograms = []string{"/bin/sh", "/bin/bash", "/usr/bin/cc", "/usr/bin/clang", "/usr/bin/ld", "/usr/bin/ar", "/usr/bin/xcrun", "/usr/bin/xcode-select"}

// sdkRoot is the SDK that the linker uses, asked outside the sandbox (xcrun cannot run in it, and the linker would pick another one).
func sdkRoot() string { return sdkRootIn("") }

// sdkRootIn is sdkRoot with DEVELOPER_DIR set to dev when dev is not empty, so that the SDK is the one of the developer directory the build gets.
func sdkRootIn(dev string) string {
	cmd := exec.Command("/usr/bin/xcrun", "--show-sdk-path")
	if dev != "" {
		cmd.Env = append(os.Environ(), "DEVELOPER_DIR="+dev)
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// selectedDeveloperDir is what `xcode-select -p` says, asked outside the sandbox ("" when it cannot be asked).
func selectedDeveloperDir() string {
	out, err := exec.Command("/usr/bin/xcode-select", "-p").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// developerDirFromHost collects the facts of this machine and decides (see decideDeveloperDir).
func (o *isolationOpts) developerDirFromHost() (value, source string, err error) {
	f := developerDirFacts{goos: runtime.GOOS, userEnv: o.env}
	if st, err := os.Stat(cltDir); err == nil && st.IsDir() {
		f.cltExists = true
	}
	if !f.cltExists {
		f.selected = selectedDeveloperDir()
	}
	return decideDeveloperDir(f)
}

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
	devDir, devSource, err := o.developerDirFromHost()
	if err != nil {
		return nil, err
	}
	o.developerDir = nil
	if devSource != "" {
		o.developerDir = &report.DeveloperDir{Value: devDir, Source: devSource}
		if o.verbose {
			fmt.Fprintf(o.err(), "rivus: DEVELOPER_DIR=%s for the build (%s)\n", devDir, devSource)
		}
	}
	sdk := sdkRootIn(devDir)
	env := func(work string) []string {
		e := []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "HOME=" + work, "TMPDIR=" + work, "CARGO_HOME=" + cargoHome,
			"RIVUS_RUST_DIRECT=1", "RUSTUP_TOOLCHAIN=" + pinnedToolchain}
		if sdk != "" {
			e = append(e, "SDKROOT="+sdk)
		}
		if devSource == "default" {
			e = append(e, "DEVELOPER_DIR="+devDir)
		}
		return append(e, o.env...)
	}
	common := sandbox.Spec{
		Exec:      []string{tc, "/Library/Developer/CommandLineTools"},
		ExecFiles: sandboxPrograms,
		// the system's configuration of TLS: without it the cargo that is not bundled with its own cannot start
		Timeout: o.timeout,
	}
	common.Read = append(common.Read, o.read...)

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
	s.Read = append(append([]string(nil), common.Read...), dir, cargoHome, "/private/etc/ssl")
	s.ExecFiles = append(append(append([]string(nil), sandboxPrograms...), extractor), o.execExtra...)
	s.Write = []string{work}
	s.MaxOutput = o.maxOutput
	s.Stdout = w
	if s.MaxOutput <= 0 {
		s.MaxOutput = 4 << 30
	}
	fmt.Fprintf(o.err(), "rivus: extracting rust under the sandbox of macOS (no network, the project and the dependencies read-only): %s\n", strings.Join(s.Argv, " "))
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
		if err == sandbox.ErrUnavailable {
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

// sandboxResult says how a stage ended: what it printed goes to the user, and a stage that failed or was stopped is an error.
func (o *isolationOpts) sandboxResult(stage string, res *sandbox.Result, err error) error {
	if err != nil {
		if err == sandbox.ErrUnavailable {
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
	if res.MayHaveLeftProcesses && stage == "extract" {
		// said once, not hidden: a build script can start a process in a session of its own, which the end of the run does not reach
		fmt.Fprintf(o.err(), "rivus: a build script may have left a process behind (a program that starts one in a session of its own is not stopped by the end of the run; it stays under the sandbox, with no files and no network). Look with `ps` if the project is not one you trust.\n")
	}
	return nil
}

// noSandboxError is said when the sandbox cannot be made: nothing is run.
func noSandboxError(why error) error {
	return fmt.Errorf("the extractor of Rust runs the build scripts and the macros of the project, and it is run under the sandbox of macOS so that they cannot reach the files and the network of this machine; it cannot be made here (%v), so nothing was run. If the code is yours or you trust it, -unsafe-host-exec runs it directly on this machine, without any isolation (the build scripts of the project run here, with your rights)", why)
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
	s.Read = append(append([]string(nil), common.Read...), dir, "/private/etc/ssl")
	s.Write = []string{work, cargoHome}
	s.Network = true
	return s
}
