package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yagihash/rivus/internal/report"
)

// Reading a Rust project builds it: cargo runs its build scripts and its proc macros. For code that is not trusted
// that runs under the sandbox of the system (macOS: Seatbelt; Linux: Landlock and seccomp; internal/sandbox, ADR-093, docs/rust-isolation.md, docs/DESIGN-linux-sandbox.md), in two stages:
//
//  1. fetch (network): `cargo fetch --locked` puts the sources of the dependencies in a CARGO_HOME of rivus. cargo does not build in
//     this stage, so no build.rs and no proc macro runs. But cargo reads the cargo configuration it finds, and the
//     configuration of the project can name a program that cargo runs (a credential provider) and a place to send
//     requests to. So this stage runs from a work directory that has none, and the CARGO_HOME is checked for a configuration
//     that an earlier run could have left. A project that needs its configuration to fetch (its own registry, a replaced
//     source) is stopped with the reason; one that is vendored skips this stage.
//  2. extract (no network): the extractor runs `cargo check --offline --locked`; the project is read-only, so is the CARGO_HOME,
//     and what is written goes to a work directory that is removed after.
//
// Both stages run under the profile of internal/sandbox (everything denied, what is needed allowed) with an environment that holds
// nothing of the host. The sandbox is made in isolate_sandbox_darwin.go and isolate_sandbox_linux.go; on another system nothing is run (isolate_sandbox_other.go).

// pinnedToolchain is the nightly the extractor is pinned to (extractor/rust/rust-toolchain.toml).
const pinnedToolchain = "nightly-2026-11-15"

// hostileMarker is a file in a test project that is hostile on purpose: it is never run on this machine.
const hostileMarker = ".rivus-hostile"

// isolationOpts are the limits of the run and what it is given.
type isolationOpts struct {
	// extractor is the command of the Rust extractor (the sandbox runs it by its path)
	extractor []string
	// execExtra are programs that the extraction may run besides the build's own (a test of the sandbox runs its probe so)
	execExtra []string
	timeout   time.Duration
	noFetch   bool
	// maxOutput is the most the extraction may write to its standard output (the stream of the IR); 0: 4 GiB
	maxOutput int64
	// allowHosts are the hosts (-allow-fetch-host) that the fetch stage may get from although Cargo.lock names them and
	// they are not among the ones that are allowed without a flag
	allowHosts []string
	// env are NAME=VALUE pairs given on the command line (the extract stage only), and read are paths that the build may read
	// besides the project and its dependencies: the only way something of the host gets in. Nothing is passed on from the environment.
	env, read []string
	// verbose prints the DEVELOPER_DIR that the build gets; developerDir is set by the run (nil: none was set) and goes to the summary
	verbose      bool
	developerDir *report.DeveloperDir
	// stderr is where what the stages say goes (the standard error by default)
	stderr io.Writer
}

func (o *isolationOpts) err() io.Writer {
	if o.stderr != nil {
		return o.stderr
	}
	return os.Stderr
}

// mountable says whether the directory can be read by the build: it is not the home directory of the user or a directory
// that holds it (that would hand over what is not the project).
func mountable(dir string) error {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(dir, home); err == nil && !strings.HasPrefix(rel, "..") {
			return fmt.Errorf("%s is the home directory or holds it: it is not given to the build (name the project directory)", dir)
		}
	}
	return nil
}

// projectToolchain is the channel a project asks for in rust-toolchain(.toml), or "".
func projectToolchain(dir string) string {
	if b, err := os.ReadFile(filepath.Join(dir, "rust-toolchain.toml")); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			l = strings.TrimSpace(l)
			if v, ok := strings.CutPrefix(l, "channel"); ok {
				if v, ok = strings.CutPrefix(strings.TrimSpace(v), "="); ok {
					return strings.Trim(strings.TrimSpace(v), "\"'")
				}
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "rust-toolchain")); err == nil {
		s := strings.TrimSpace(string(b))
		if !strings.Contains(s, "[") {
			return strings.Trim(s, "\"'")
		}
	}
	return ""
}

// cargoVolume names the volume the dependencies of this project go to: it is a function of the lock file and of the
// cargo configuration the project brings, so what one project fetched is not what another one reads.
func cargoVolume(dir string) (string, error) {
	h := sha256.New()
	lock, err := os.ReadFile(filepath.Join(dir, "Cargo.lock"))
	if err != nil {
		return "", fmt.Errorf("%s has no Cargo.lock: the isolated run builds with --locked and without a network, so the dependencies have to be named. "+
			"Make one with `cargo generate-lockfile` (it runs no code of the project), or use -unsafe-host-exec for code you trust", dir)
	}
	h.Write(lock)
	for _, f := range []string{".cargo/config.toml", ".cargo/config"} {
		if b, err := os.ReadFile(filepath.Join(dir, f)); err == nil {
			h.Write([]byte("\x00" + f + "\x00"))
			h.Write(b)
		}
	}
	return "rivus-cargo-" + hex.EncodeToString(h.Sum(nil))[:16], nil
}
