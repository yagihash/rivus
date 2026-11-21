package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Reading a Rust project builds it: cargo runs its build scripts and its proc macros. For code that is not trusted
// that runs in a container (ADR-064, docs/rust-isolation.md), in two stages:
//
//  1. fetch (network): `cargo fetch --locked` puts the sources of the dependencies in a volume. cargo does not build in
//     this stage, so no build.rs and no proc macro runs. But cargo reads the cargo configuration it finds, and the
//     configuration of the project can name a program that cargo runs (a credential provider) and a place to send
//     requests to. So this stage does not see the configuration of the project (.cargo is covered with an empty
//     tmpfs), and the volume is checked for a configuration that an earlier run could have left. A project that
//     needs its configuration to fetch (its own registry, a replaced source) is stopped with the reason; one that
//     is vendored skips this stage.
//  2. extract (no network): the extractor runs `cargo check --offline --locked`; the project is mounted read-only,
//     so is the volume of the dependencies, and what is written goes to volumes that are removed with the container.
//
// Both stages run as a user that is not root, with no capability, no new privileges, a root file system that is read-only, limits
// of processes, memory and CPUs, and an environment that holds nothing of the host.

// rustImageName is the default image: the extractor with the toolchain it is pinned to (docker/rust-extractor).
const rustImageName = "rivus-rust-extractor:nightly-2026-11-15"

// pinnedToolchain is the nightly the image has, and the extractor is pinned to (extractor/rust/rust-toolchain.toml).
const pinnedToolchain = "nightly-2026-11-15"

// hostileMarker is a file in a test project that is hostile on purpose: it is never run on this machine.
const hostileMarker = ".rivus-hostile"

// isolationOpts are the limits of the container and where it is found.
type isolationOpts struct {
	image   string
	memory  string
	cpus    int
	pids    int
	timeout time.Duration
	noFetch bool
	// maxOutput is the most the container may write to its standard output (the stream of the IR); 0: 4 GiB
	maxOutput int64
	// allowHosts are the hosts (-allow-fetch-host) that the fetch stage may get from although Cargo.lock names them and
	// they are not among the ones that are allowed without a flag
	allowHosts []string
	// env are NAME=VALUE pairs given on the command line (the extract stage only), and mounts are SRC:DST read-only
	// mounts: the only way something of the host gets in besides the project. Nothing is passed on from the environment.
	env, mounts []string
	// stderr is where what the container says goes (the standard error by default)
	stderr io.Writer
}

func (o *isolationOpts) cpuCount() int {
	if o.cpus > 0 {
		return o.cpus
	}
	return min(runtime.NumCPU(), 8)
}

func (o *isolationOpts) err() io.Writer {
	if o.stderr != nil {
		return o.stderr
	}
	return os.Stderr
}

// noDockerError is what is said when the container cannot be used, and the code is not to be run here.
func noDockerError(why error) error {
	return fmt.Errorf("the Rust extractor runs the build scripts and the proc macros of the project it reads. To isolate them rivus uses Docker, and it cannot be used (%v). "+
		"Start Docker, or add -unsafe-host-exec (or set RIVUS_UNSAFE_HOST_EXEC=1) to run it directly on this machine, which is for code you trust", why)
}

// checkDocker finds out that the daemon answers and that the image is there.
func (o *isolationOpts) checkDocker(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return noDockerError(errors.New("the docker command is not found"))
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(cctx, "docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput(); err != nil {
		return noDockerError(fmt.Errorf("docker info: %v: %s", err, strings.TrimSpace(string(out))))
	}
	if out, err := exec.CommandContext(cctx, "docker", "image", "inspect", "--format", "{{.Id}}", o.image).CombinedOutput(); err != nil {
		return fmt.Errorf("the image %s is not there (%s). Build it from this repository, which is the source that is trusted:\n"+
			"  docker build -f docker/rust-extractor/Dockerfile -t %s extractor/rust", o.image, strings.TrimSpace(string(out)), o.image)
	}
	return nil
}

// mountable says whether the directory can be handed to the container: its name has to survive a --mount option,
// and it is not the home directory of the user or a directory that holds it (that would hand over what is not the project).
func mountable(dir string) error {
	if strings.ContainsAny(dir, ",\"'\n\r") {
		return fmt.Errorf("%s: the name has a comma, a quote or a line break, which a mount cannot carry", dir)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(dir, home); err == nil && !strings.HasPrefix(rel, "..") {
			return fmt.Errorf("%s is the home directory or holds it: it is not mounted into the container (name the project directory)", dir)
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

func randomName(prefix string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}

// limits are the arguments that every container gets.
func (o *isolationOpts) limits(name string) []string {
	return []string{
		"run", "--rm", "--name", name,
		"--user", "10001:10001",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--read-only",
		"--pids-limit", strconv.Itoa(o.pids),
		"--memory", o.memory, "--memory-swap", o.memory,
		"--cpus", strconv.Itoa(o.cpuCount()),
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=256m",
		"--label", "rivus=rust-extractor",
	}
}

// fetchArgs are the arguments of `docker` for the stage that has the network. cargo fetch does not build, but it reads the
// cargo configuration it finds, which can make it run a program of the project (see the comment at the top): the
// .cargo of the project, the only place in the container where cargo looks for one that comes from the project, is
// covered with an empty tmpfs.
func (o *isolationOpts) fetchArgs(name, dir, volume string) []string {
	a := o.limits(name)
	a = append(a, "--mount", "type=bind,src="+dir+",dst="+dir+",readonly")
	if _, err := os.Lstat(filepath.Join(dir, ".cargo")); err == nil {
		a = append(a, "--mount", "type=tmpfs,dst="+filepath.Join(dir, ".cargo")+",tmpfs-size=1048576")
	}
	a = append(a,
		"--mount", "type=volume,src="+volume+",dst=/cargo-home",
		"--tmpfs", "/work:rw,noexec,nosuid,nodev,size=64m",
		"-w", dir,
		"--entrypoint", "cargo",
		o.image,
		"fetch", "--locked")
	return a
}

// extractArgs are the arguments for the stage that builds: no network, the project and the dependencies are read-only.
func (o *isolationOpts) extractArgs(name, dir, volume string, extractor []string) []string {
	a := o.limits(name)
	a = append(a,
		"--network", "none",
		"--mount", "type=bind,src="+dir+",dst="+dir+",readonly",
		"--mount", "type=volume,src="+volume+",dst=/cargo-home,readonly",
		// what the build writes (the build, the fragments of the IR): a volume that goes with the container
		"--mount", "type=volume,dst=/work",
		"-w", dir)
	for _, e := range o.env {
		a = append(a, "-e", e)
	}
	for _, m := range o.mounts {
		src, dst, _ := strings.Cut(m, ":")
		a = append(a, "--mount", "type=bind,src="+src+",dst="+dst+",readonly")
	}
	a = append(a, o.image)
	return append(a, extractor...)
}

// runContainer runs docker with the arguments; at the end of the time, or when the context is cancelled, the container
// is removed (stopping the docker command would leave it running).
func (o *isolationOpts) runContainer(ctx context.Context, name string, args []string, stdout io.Writer, stage string) error {
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Cancel = func() error {
		_ = exec.Command("docker", "rm", "-f", name).Run()
		return cmd.Process.Kill()
	}
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "DOCKER_HOST=" + os.Getenv("DOCKER_HOST"), "DOCKER_CONTEXT=" + os.Getenv("DOCKER_CONTEXT"), "DOCKER_CONFIG=" + os.Getenv("DOCKER_CONFIG")}
	var tail tailBuffer
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(o.err(), &tail)
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("the %s stage in the container did not finish in %v and was stopped (-container-timeout)", stage, o.timeout)
	}
	if err != nil {
		return fmt.Errorf("the %s stage in the container failed: %v\n%s", stage, err, strings.TrimSpace(tail.String()))
	}
	return nil
}

// tailBuffer keeps the last 8 KiB of what is written to it.
type tailBuffer struct{ b []byte }

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > 8192 {
		t.b = t.b[len(t.b)-8192:]
	}
	return len(p), nil
}
func (t *tailBuffer) String() string { return string(t.b) }

// limitWriter fails when more than max bytes are written: what the container says is data of the project that is
// read, and its size is not left to it.
type limitWriter struct {
	w   io.Writer
	max int64
	n   int64
}

func (l *limitWriter) Write(p []byte) (int, error) {
	l.n += int64(len(p))
	if l.n > l.max {
		return 0, fmt.Errorf("the container wrote more than %d bytes (-container-max-output)", l.max)
	}
	return l.w.Write(p)
}

// extractRustIsolated reads the Rust project in dir in the container and returns the stream of the IR.
func (o *isolationOpts) extractRustIsolated(ctx context.Context, dir string, extractorArgs []string) ([]byte, error) {
	if err := mountable(dir); err != nil {
		return nil, err
	}
	volume, err := cargoVolume(dir)
	if err != nil {
		return nil, err
	}
	if err := o.checkDocker(ctx); err != nil {
		return nil, err
	}
	if want := projectToolchain(dir); want != "" && want != pinnedToolchain {
		fmt.Fprintf(o.err(), "rivus: the project asks for the toolchain %q; the extractor is pinned to %s and the container does not fetch another one, so it is built with %s (it may not compile)\n", want, pinnedToolchain, pinnedToolchain)
	}
	if !o.noFetch {
		plan, err := planFetch(dir)
		if err != nil {
			return nil, err
		}
		if plan.vendored {
			fmt.Fprintf(o.err(), "rivus: the project is vendored (its cargo configuration replaces the sources with a directory of the project): the fetch stage is skipped\n")
			if out, err := exec.CommandContext(ctx, "docker", "volume", "create", volume).CombinedOutput(); err != nil {
				return nil, fmt.Errorf("docker volume create %s: %v: %s", volume, err, strings.TrimSpace(string(out)))
			}
		} else {
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
			if err := o.dropConfigOfVolume(ctx, volume); err != nil {
				return nil, err
			}
			fmt.Fprintf(o.err(), "rivus: fetching the dependencies (cargo fetch --locked, with a network; the cargo configuration of the project is not read in this stage) into the volume %s\n", volume)
			name := randomName("rivus-fetch-")
			if err := o.runContainer(ctx, name, o.fetchArgs(name, dir, volume), io.Discard, "fetch"); err != nil {
				if plan.hasConfig {
					err = fmt.Errorf("%w\n(the project has a cargo configuration, and it is not used in the stage that fetches. If the project needs it to fetch, fetch it yourself or vendor it, then use -no-fetch)", err)
				}
				return nil, err
			}
		}
	}
	// without the network a volume that was never filled would be an empty one: it is said as that
	if out, err := exec.CommandContext(ctx, "docker", "volume", "inspect", volume).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("the dependencies are not fetched (%s): run without -no-fetch once", strings.TrimSpace(string(out)))
	}
	name := randomName("rivus-extract-")
	args := o.extractArgs(name, dir, volume, append([]string{"--locked", "--offline"}, extractorArgs...))
	fmt.Fprintf(o.err(), "rivus: extracting rust in a container (no network, the project read-only): docker %s\n", strings.Join(args, " "))
	var out bytes.Buffer
	maxOut := o.maxOutput
	if maxOut <= 0 {
		maxOut = 4 << 30
	}
	if err := o.runContainer(ctx, name, args, &limitWriter{w: &out, max: maxOut}, "extract"); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
