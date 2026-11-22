//go:build darwin

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yagihash/rivus/internal/sandbox"
)

// On macOS a Rust project that is not trusted is read under the sandbox of the system (Seatbelt: internal/sandbox,
// docs/DESIGN-macos-sandbox.md), in the two stages that the container had: the fetch (cargo fetch --locked, with a network, the
// cargo configuration of the project not read) and the extraction (no network, the project and the fetched sources read-only).
// The dependencies are fetched into a CARGO_HOME of rivus (not ~/.cargo, which is never read nor written): the fetch writes it, the
// extraction only reads it. The toolchain is the pinned nightly, run directly (no rustup), prepared outside the sandbox
// from a trusted source.

// rustDataDir is where rivus keeps what it prepares for Rust (the dependencies that were fetched).
func rustDataDir() (string, error) {
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
var sandboxPrograms = []string{"/bin/sh", "/usr/bin/cc", "/usr/bin/clang", "/usr/bin/ld", "/usr/bin/ar", "/usr/bin/xcrun", "/usr/bin/xcode-select"}

// sdkRoot is the SDK that the linker uses, asked outside the sandbox (xcrun cannot run in it, and the linker would pick another one).
func sdkRoot() string {
	out, err := exec.Command("/usr/bin/xcrun", "--show-sdk-path").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// extractRustSandboxed reads the project under the sandbox and returns the IR. It runs nothing on this machine outside the sandbox.
func (o *isolationOpts) extractRustSandboxed(ctx context.Context, dir string, extractorArgs []string) ([]byte, error) {
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
	sdk := sdkRoot()
	env := func(work string) []string {
		e := []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "HOME=" + work, "TMPDIR=" + work, "CARGO_HOME=" + cargoHome,
			"RIVUS_RUST_DIRECT=1", "RUSTUP_TOOLCHAIN=" + pinnedToolchain}
		if sdk != "" {
			e = append(e, "SDKROOT="+sdk)
		}
		return append(e, o.env...)
	}
	common := sandbox.Spec{
		Exec:      []string{tc, "/Library/Developer/CommandLineTools"},
		ExecFiles: sandboxPrograms,
		// the system's configuration of TLS: without it the cargo that is not bundled with its own cannot start
		Timeout: o.timeout,
	}
	for _, m := range o.mounts {
		src, _, _ := strings.Cut(m, ":")
		common.Read = append(common.Read, src)
	}

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
			s := common
			// cargo looks for its configuration from the directory it is run in, not from the manifest: the work directory has none
			s.Argv = []string{filepath.Join(tc, "bin", "cargo"), "fetch", "--locked", "--manifest-path", filepath.Join(dir, "Cargo.toml")}
			s.Dir = work
			s.Env = env(work)
			s.Read = append(append([]string(nil), common.Read...), dir, "/private/etc/ssl")
			s.Write = []string{work, cargoHome}
			s.Network = true
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

// sandboxResult says how a stage ended: what it printed goes to the user, and a stage that failed or was stopped is an error.
func (o *isolationOpts) sandboxResult(stage string, res *sandbox.Result, err error) error {
	if err != nil {
		if err == sandbox.ErrUnavailable {
			return noSandboxError(err)
		}
		return fmt.Errorf("the %s stage under the sandbox: %v", stage, err)
	}
	if res.TimedOut {
		return fmt.Errorf("the %s stage under the sandbox did not finish in %v and was stopped (-container-timeout)", stage, o.timeout)
	}
	if res.OutputTooBig {
		return fmt.Errorf("the %s stage under the sandbox wrote more than the output limit and was stopped (-container-max-output)", stage)
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
