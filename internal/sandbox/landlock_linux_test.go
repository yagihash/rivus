//go:build linux

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The tests of the sandbox of Linux (docs/DESIGN-linux-sandbox.md): each line of what is closed has a test that shows that it holds, and the
// same probe run without the sandbox shows that what it tries is possible (so that "blocked" is not the probe failing to run).

func skipWithoutSandbox(t *testing.T) {
	t.Helper()
	if err := Available(); err != nil {
		if os.Getenv("RIVUS_SBX_RED") == "1" {
			t.Fatalf("the sandbox is not available, and RIVUS_SBX_RED=1 says that it must be: %v", err)
		}
		t.Skipf("no sandbox here: %v", err)
	}
}

// probeBinary builds the probe once (static: it needs nothing of the system, so it runs under the sandbox with no more than its own file).
func probeBinary(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "probe")
	cmd := exec.Command("go", "build", "-o", out, "./probelinux")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the probe: %v\n%s", err, b)
	}
	return out
}

// verdicts parses the lines of the probe: name -> true when it ESCAPED.
func verdicts(t *testing.T, out string) map[string]bool {
	t.Helper()
	m := map[string]bool{}
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && (f[1] == "BLOCKED" || f[1] == "ESCAPED") {
			m[f[0]] = f[1] == "ESCAPED"
		}
	}
	return m
}

func listen(t *testing.T) (net.Listener, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("no loopback: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return l, l.Addr().String()
}

type fixture struct {
	probe, canary, outside, work, sock, udp, tcp, tcp2, prog string
}

func setup(t *testing.T) fixture {
	t.Helper()
	f := fixture{probe: probeBinary(t)}
	root := t.TempDir()
	f.canary = filepath.Join(root, "canary")
	os.WriteFile(f.canary, []byte("FAKE-SECRET"), 0o600)
	f.outside = filepath.Join(root, "outside")
	os.Mkdir(f.outside, 0o755)
	f.work = filepath.Join(root, "work")
	os.Mkdir(f.work, 0o755)
	_, f.tcp = listen(t)
	_, f.tcp2 = listen(t)
	f.sock = filepath.Join(root, "s.sock")
	ul, err := net.Listen("unix", f.sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ul.Close() })
	go func() {
		for {
			c, err := ul.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	f.udp = pc.LocalAddr().String()
	f.prog, _ = exec.LookPath("id")
	return f
}

func (f fixture) args() []string {
	a := []string{f.probe, "-canary", f.canary, "-dir", f.outside, "-tcp", f.tcp, "-tcp2", f.tcp2, "-sock", f.sock, "-udp", f.udp}
	if f.prog != "" {
		a = append(a, "-exec", f.prog)
	}
	return a
}

var allChecks = []string{"read-secret", "write-outside", "tcp", "tcp-other-port", "udp", "unix-socket", "exec-other", "proc-environ", "io-uring", "ptrace", "unshare", "clone-newns", "clone3", "sctp", "mptcp", "sysv-shm", "sysv-msg", "sysv-sem", "posix-mq", "kcmp"}

// Without the sandbox the probe gets out of everything that the test set up: otherwise a "blocked" under the sandbox shows nothing.
func TestSandboxProbeEscapesOnTheHost(t *testing.T) {
	f := setup(t)
	out, err := exec.Command(f.args()[0], f.args()[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	v := verdicts(t, string(out))
	checks := []string{"read-secret", "write-outside", "tcp", "tcp-other-port", "udp", "unix-socket", "proc-environ", "unshare", "clone-newns", "clone3"}
	if os.Geteuid() == 0 {
		checks = append(checks, "capabilities", "chown")
	}
	for _, c := range checks {
		if esc, ok := v[c]; !ok || !esc {
			t.Errorf("%s: on the host the probe must get out, and it did not (%v)\n%s", c, v[c], out)
		}
	}
}

// Under the sandbox without a network the probe gets out of nothing.
func TestSandboxClosesEveryWayOut(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	a := f.args()
	res, err := Run(context.Background(), Spec{Argv: a, Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("probe under the sandbox:\n%s", res.Stdout)
	v := verdicts(t, string(res.Stdout))
	checks := append([]string(nil), allChecks...)
	if os.Geteuid() == 0 {
		// run as root: the capabilities are taken away, and a file cannot be given to another user
		checks = append(checks, "capabilities", "chown")
	}
	if ABI() < 6 {
		// signals and abstract sockets are scoped from ABI 6 (Linux 6.12): with less, the signal to the parent is possible
		t.Logf("Landlock ABI %d: the signal to a process outside is not closed", ABI())
	} else {
		checks = append(checks, "signal-outside")
	}
	for _, c := range checks {
		esc, ok := v[c]
		if !ok {
			if c == "exec-other" && f.prog == "" {
				continue
			}
			t.Errorf("%s: the probe did not say (exit %d)\n%s\n%s", c, res.ExitCode, res.Stdout, res.Stderr)
			continue
		}
		if esc {
			t.Errorf("%s: ESCAPED under the sandbox\n%s", c, res.Stdout)
		}
	}
}

func TestSandboxWritesOnlyInTheWorkDirectory(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	res, err := Run(context.Background(), Spec{Argv: []string{"/bin/sh", "-c", fmt.Sprintf("echo a > %s/in && ! (echo b > %s/out) 2>/dev/null", f.work, f.outside)},
		Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s", res.ExitCode, res.Stderr)
	}
	if _, err := os.Stat(filepath.Join(f.work, "in")); err != nil {
		t.Error("what is allowed was not written")
	}
	if _, err := os.Stat(filepath.Join(f.outside, "out")); err == nil {
		t.Error("what is not allowed was written")
	}
}

// Only the programs that are named run (the others of /usr/bin do not).
func TestSandboxRunsOnlyNamedPrograms(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	if f.prog == "" {
		t.Skip("no id")
	}
	res, err := Run(context.Background(), Spec{Argv: []string{"/bin/sh", "-c", f.prog + " >/dev/null"}, Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode == 0 {
		t.Errorf("a program that was not named ran")
	}
	res, err = Run(context.Background(), Spec{Argv: []string{"/bin/sh", "-c", f.prog + " >/dev/null"}, Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, ExecFiles: []string{f.prog}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Errorf("a program that was named did not run: %s", res.Stderr)
	}
}

// With a proxy port, the port can be connected to and no other, and nothing but TCP is open.
func TestSandboxNetworkProxyOnly(t *testing.T) {
	skipWithoutSandbox(t)
	if ABI() < 4 {
		t.Skipf("Landlock ABI %d: the port of a connection is checked from ABI 4 (Linux 6.7)", ABI())
	}
	f := setup(t)
	_, port, _ := net.SplitHostPort(f.tcp)
	var p int
	fmt.Sscan(port, &p)
	res, err := Run(context.Background(), Spec{Argv: f.args(), Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, ProxyPort: p, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	v := verdicts(t, string(res.Stdout))
	if !v["tcp"] {
		t.Errorf("the port of the proxy cannot be reached\n%s", res.Stdout)
	}
	for _, c := range []string{"tcp-other-port", "udp", "unix-socket", "read-secret", "write-outside", "sctp", "mptcp"} {
		if esc, ok := v[c]; !ok || esc {
			t.Errorf("%s: %v\n%s", c, v[c], res.Stdout)
		}
	}
}

// A proxy port on a kernel that cannot check it is not run (and nothing runs).
func TestSandboxProxyNeedsABI4(t *testing.T) {
	skipWithoutSandbox(t)
	if ABI() >= 4 {
		t.Skip("this kernel can")
	}
	_, err := Run(context.Background(), Spec{Argv: []string{"/bin/true"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, ProxyPort: 8080})
	if err == nil {
		t.Fatal("it ran")
	}
}

func TestSandboxTimeAndOutput(t *testing.T) {
	skipWithoutSandbox(t)
	res, err := Run(context.Background(), Spec{Argv: []string{"/bin/sh", "-c", "sleep 30"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, ExecFiles: []string{"/bin/sleep"}, Timeout: time.Second})
	if err != nil || !res.TimedOut {
		t.Fatalf("not stopped by the time: %v %+v", err, res)
	}
	res, err = Run(context.Background(), Spec{Argv: []string{"/bin/sh", "-c", "yes"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, ExecFiles: []string{"/usr/bin/yes"}, Timeout: time.Minute, MaxOutput: 1 << 16})
	if err != nil || !res.OutputTooBig {
		t.Fatalf("not stopped by the output: %v %+v", err, res)
	}
}

// The environment of the program is the one that was given: nothing is taken from the caller.
func TestSandboxEnvironment(t *testing.T) {
	skipWithoutSandbox(t)
	t.Setenv("RIVUS_SBX_SECRET", "leak")
	res, err := Run(context.Background(), Spec{Argv: []string{"/usr/bin/env"}, Dir: "/", Env: []string{"A=1"}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(res.Stdout)); got != "A=1" {
		t.Errorf("the environment is %q", got)
	}
}

// A build script is code of the project that runs when it is read: it may run the C compiler (which a build needs), and it may do nothing else
// that the probe tries. The crate is made here; cargo and a toolchain have to be on this machine (the test is left out when they are not).
func TestSandboxBuildScriptIsContained(t *testing.T) {
	skipWithoutSandbox(t)
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("no cargo")
	}
	rustc, err := exec.LookPath("rustc")
	if err != nil {
		t.Skip("no rustc")
	}
	b, err := exec.Command(rustc, "--print", "sysroot").Output()
	if err != nil {
		t.Skipf("no sysroot: %v", err)
	}
	tc := strings.TrimSpace(string(b))
	cargo = filepath.Join(tc, "bin", "cargo")
	if _, err := os.Stat(cargo); err != nil {
		t.Skip("the toolchain has no cargo in it")
	}
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	os.MkdirAll(filepath.Join(proj, "src"), 0o755)
	canary := filepath.Join(root, "secret")
	os.WriteFile(canary, []byte("FAKE"), 0o600)
	outside := filepath.Join(root, "outside")
	os.Mkdir(outside, 0o755)
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(proj, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Cargo.toml", "[package]\nname = \"hostile\"\nversion = \"0.1.0\"\nedition = \"2021\"\nbuild = \"build.rs\"\n")
	write("Cargo.lock", "version = 4\n\n[[package]]\nname = \"hostile\"\nversion = \"0.1.0\"\n")
	write("src/main.rs", "fn main() {}\n")
	write("build.rs", fmt.Sprintf(`use std::{env, fs, net::TcpStream, process::Command};
fn main() {
    let out = env::var("OUT_DIR").unwrap();
    fs::write(format!("{out}/a.c"), "int f(void){return 1;}").unwrap();
    let cc = Command::new("cc").args(["-c", &format!("{out}/a.c"), "-o", &format!("{out}/a.o")]).status().map(|s| s.success()).unwrap_or(false);
    let r = [
        format!("cc={cc}"),
        format!("read_secret={}", fs::read_to_string(%q).is_ok()),
        format!("write_outside={}", fs::write(%q, "x").is_ok()),
        format!("net={}", TcpStream::connect("127.0.0.1:%s").is_ok()),
        format!("curl={}", Command::new("curl").arg("--version").output().is_ok()),
    ];
    println!("cargo:warning=RESULT {}", r.join(","));
}
`, canary, filepath.Join(outside, "x"), strings.Split(listenAddr(t), ":")[1]))
	work := t.TempDir()
	cargoHome := t.TempDir()
	res, err := Run(context.Background(), Spec{
		Argv: []string{cargo, "check", "--offline", "--locked", "--manifest-path", filepath.Join(proj, "Cargo.toml"), "--target-dir", filepath.Join(work, "target")},
		Dir:  proj,
		Env:  []string{"PATH=" + filepath.Join(tc, "bin") + ":/usr/bin:/bin", "HOME=" + work, "TMPDIR=" + work, "CARGO_HOME=" + cargoHome, "RUSTUP_TOOLCHAIN=" + filepath.Base(tc)},
		Exec: []string{tc, "/usr/libexec", "/usr/lib/gcc"}, Read: []string{proj, cargoHome, "/usr/include"},
		ExecFiles: []string{"/bin/sh", "/usr/bin/cc", "/usr/bin/gcc", "/usr/bin/ld", "/usr/bin/as", "/usr/bin/ar", "/usr/bin/env"},
		Write:     []string{work}, Timeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := string(res.Stderr)
	if res.ExitCode != 0 {
		t.Fatalf("cargo failed (exit %d):\n%s", res.ExitCode, out)
	}
	const want = "RESULT cc=true,read_secret=false,write_outside=false,net=false,curl=false"
	if !strings.Contains(out, want) {
		t.Errorf("the build script was not held as it should be: want %q in\n%s", want, out)
	}
}

// listenAddr starts a listener that the test closes, and returns its host:port.
func listenAddr(t *testing.T) string {
	t.Helper()
	_, a := listen(t)
	return a
}

// The baseline does not let anything be run from the directories of the libraries (the programs of a system can be there: the Rust coreutils of
// Ubuntu are in /usr/lib/cargo/bin), and the dynamic loader is run by its path.
func TestSandboxBaselineRunsOnlyTheLoader(t *testing.T) {
	var c helperConfig
	c.addBaseline()
	if len(c.Exec) != 0 || len(c.SoftExecFiles) != len(loaders()) {
		t.Errorf("the baseline runs programs from directories, or something besides the loaders: %v %v", c.Exec, c.SoftExecFiles)
	}
	if len(loaders()) == 0 {
		t.Skip("no dynamic loader found on this system")
	}
	for _, l := range loaders() {
		found := false
		for _, f := range c.SoftExecFiles {
			found = found || f == l
		}
		if !found {
			t.Errorf("the loader %s is not allowed to run", l)
		}
	}
}

// The changes of attributes that Landlock does not stop are reported (not refused): the ones outside of the places that may be written, and not
// the ones inside.
func TestSandboxReportsAttributeChanges(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	victim := filepath.Join(f.outside, "victim")
	os.WriteFile(victim, []byte("data"), 0o600)
	inside := filepath.Join(f.work, "mine")
	os.WriteFile(inside, []byte("data"), 0o600)
	res, err := Run(context.Background(), Spec{Argv: []string{f.probe, "-attr", victim, "-attr-in", inside}, Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s %s", res.ExitCode, res.Stdout, res.Stderr)
	}
	// the calls are let through (it is a report): the file is changed, and the program says that it worked
	if st, err := os.Stat(victim); err != nil || st.Mode().Perm() != 0o640 {
		t.Errorf("the call was not let through: %v %v", st, err)
	}
	real, _ := filepath.EvalSymlinks(victim)
	got := strings.Join(res.AttributeChanges, "\n")
	for _, want := range []string{"chmod " + real, "utimensat " + real} {
		if !strings.Contains(got, want) && !strings.Contains(got, strings.Replace(want, "chmod", "fchmodat", 1)) {
			t.Errorf("not reported: %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "mine") {
		t.Errorf("a change inside the work directory is reported:\n%s", got)
	}
}

func TestSandboxABIPolicy(t *testing.T) {
	for _, c := range []struct {
		abi   int
		fatal bool
		warn  bool
	}{{1, true, false}, {2, true, false}, {3, false, true}, {5, false, true}, {6, false, false}, {7, false, false}} {
		err, warn := abiPolicy(c.abi)
		if (err != nil) != c.fatal || (len(warn) > 0) != c.warn {
			t.Errorf("ABI %d: err=%v warn=%v", c.abi, err, warn)
		}
	}
}

// The limits of resources are on what runs: loose, and not "unlimited".
func TestSandboxLimitsOfResources(t *testing.T) {
	skipWithoutSandbox(t)
	res, err := Run(context.Background(), Spec{Argv: []string{"/bin/bash", "-c", "echo core=$(ulimit -c) procs=$(ulimit -u) data=$(ulimit -d) file=$(ulimit -f)"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	out := strings.TrimSpace(string(res.Stdout))
	t.Log(out)
	if !strings.Contains(out, "core=0") {
		t.Errorf("core files are not off: %s", out)
	}
	for _, k := range []string{"procs=", "data=", "file="} {
		i := strings.Index(out, k)
		if i < 0 {
			t.Fatalf("no %s in %q", k, out)
		}
		v := strings.Fields(out[i+len(k):])
		if len(v) == 0 || v[0] == "unlimited" || strings.HasPrefix(v[0], "core=") || strings.Contains(v[0], "=") {
			t.Errorf("%s is not limited: %s", k, out)
		}
	}
}

// An old kernel, made on a new one (RIVUS_SBX_FAKE_ABI): the run is refused below ABI 3, goes on with a warning below ABI 6, and what the old kernel
// cannot close is open (a signal to another process), which is why the warning is there.
func TestSandboxOldKernels(t *testing.T) {
	skipWithoutSandbox(t)
	if ABI() < 6 {
		t.Skipf("Landlock ABI %d: the signal is open here anyway", ABI())
	}
	f := setup(t)
	run := func(abi string) (*Result, error) {
		t.Setenv("RIVUS_SBX_FAKE_ABI", abi)
		return Run(context.Background(), Spec{Argv: []string{f.probe, "-only", "signal-outside"}, Dir: f.work, Env: []string{"PATH=/usr/bin:/bin", "RIVUS_SBX_FAKE_ABI=" + abi}, Write: []string{f.work}, Timeout: time.Minute})
	}
	if _, err := run("2"); err == nil || !errors.Is(err, ErrUnavailable) {
		t.Errorf("ABI 2 was not refused: %v", err)
	}
	for _, abi := range []string{"3", "5"} {
		res, err := run(abi)
		if err != nil {
			t.Fatalf("ABI %s: %v", abi, err)
		}
		if len(res.Warnings) == 0 {
			t.Errorf("ABI %s: no warning", abi)
		}
		if !verdicts(t, string(res.Stdout))["signal-outside"] {
			t.Errorf("ABI %s: the signal was stopped, so the kernel is not the old one that the test makes: %s", abi, res.Stdout)
		}
	}
	// the port of a connection is checked from ABI 4 only: with less, a proxy port is refused
	t.Setenv("RIVUS_SBX_FAKE_ABI", "3")
	if _, err := Run(context.Background(), Spec{Argv: []string{"/bin/true"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin", "RIVUS_SBX_FAKE_ABI=3"}, ProxyPort: 8080}); err == nil || !errors.Is(err, ErrUnavailable) {
		t.Errorf("a proxy port at ABI 3 was not refused: %v", err)
	}
	res, err := run("6")
	if err != nil || len(res.Warnings) != 0 || verdicts(t, string(res.Stdout))["signal-outside"] {
		t.Errorf("ABI 6: %v %v %s", err, res.Warnings, res.Stdout)
	}
}

// A path that the caller named and that cannot be opened (a directory above it that this user may not enter) is an error that says so: it is not
// left out in silence (the program would meet "permission denied" with nothing to say that the sandbox left it out), and it is not "the sandbox is not
// there" (-unsafe-host-exec is not the answer to it). Root can open everything: the test is for anyone else.
func TestSandboxUnusablePathIsSaid(t *testing.T) {
	skipWithoutSandbox(t)
	if os.Geteuid() == 0 {
		t.Skip("root can open any path")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	os.MkdirAll(filepath.Join(locked, "inner"), 0o755)
	os.Chmod(locked, 0)
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	_, err := Run(context.Background(), Spec{Argv: []string{"/bin/true"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Read: []string{filepath.Join(locked, "inner")}, Timeout: time.Minute})
	if err == nil || errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "inner") {
		t.Errorf("want an error that names the path and is not ErrUnavailable: %v", err)
	}
	// a path that is not there is left out, as before
	if _, err := Run(context.Background(), Spec{Argv: []string{"/bin/true"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Read: []string{filepath.Join(root, "not-there")}, Timeout: time.Minute}); err != nil {
		t.Errorf("a path that is not there was an error: %v", err)
	}
}

// A program that cannot be started is not "the sandbox is not available": the sandbox was made (the answer of the helper is told apart).
func TestSandboxProgramThatCannotStart(t *testing.T) {
	skipWithoutSandbox(t)
	_, err := Run(context.Background(), Spec{Argv: []string{"/nonexistent/program"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Timeout: time.Minute})
	if err == nil || errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "could not be started") {
		t.Errorf("want an error that says that the program could not be started, and is not ErrUnavailable: %v", err)
	}
	// an error that wraps ErrUnavailable is found by errors.Is (the callers compare with it)
	if err := helperFailure("unavailable: no Landlock"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("not ErrUnavailable: %v", err)
	}
}

// The path of a change is the file that the call is about: the link itself for a call that does not follow links (lchown), and the file of the
// descriptor for an empty path with AT_EMPTY_PATH.
func TestSandboxReportsTheRightFile(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	victim := filepath.Join(f.outside, "victim")
	os.WriteFile(victim, []byte("data"), 0o600)
	link := filepath.Join(f.work, "link")
	if err := os.Symlink(victim, link); err != nil {
		t.Fatal(err)
	}
	readable := filepath.Join(f.outside, "readable")
	os.WriteFile(readable, []byte("data"), 0o600)
	res, err := Run(context.Background(), Spec{Argv: []string{f.probe, "-attr-link", link, "-attr-fd", readable}, Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"},
		Read: []string{f.outside}, Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit %d: %s %s", res.ExitCode, res.Stdout, res.Stderr)
	}
	realVictim, _ := filepath.EvalSymlinks(victim)
	realReadable, _ := filepath.EvalSymlinks(readable)
	got := strings.Join(res.AttributeChanges, "\n")
	if strings.Contains(got, realVictim) {
		t.Errorf("lchown of a link in the work directory is reported as a change of what it points to:\n%s\n%s", got, res.Stdout)
	}
	if !strings.Contains(got, "fchownat "+realReadable) {
		t.Errorf("fchownat with AT_EMPTY_PATH is not reported as the file of the descriptor (%s):\n%s\n%s", realReadable, got, res.Stdout)
	}
}

// The reading of the notifications that fails is tried again, and not for ever: the descriptor is not closed at the first failure (the filter stays in
// the program, and with nobody holding the descriptor its calls fail with ENOSYS), and it is not kept open for ever either (nothing answers the calls
// that wait, and the build would wait for its time to be over): after maxReadFailures in a row the reading is given up, and the report says so.
func TestSandboxNotificationsAreTriedAgainThenGivenUp(t *testing.T) {
	var p [2]int
	if err := unix.Pipe(p[:]); err != nil {
		t.Fatal(err)
	}
	defer unix.Close(p[0])
	defer unix.Close(p[1])
	unix.Write(p[1], []byte("x")) // readable, and not a descriptor of notifications: every receive fails (ENOTTY)
	w := newAttrWatcher(nil)
	done := make(chan struct{}) // (never closed: the run goes on)
	returned := make(chan struct{})
	go func() { w.serve(p[0], done); close(returned) }()
	select {
	case <-returned:
		t.Fatal("the reading was given up at the first failures")
	case <-time.After(60 * time.Millisecond):
	}
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("the reading was never given up: the calls that wait would wait for the end of the run")
	}
	got := strings.Join(w.warnings(), "\n")
	for _, want := range []string{"receive", "given up", "ENOSYS"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is not in the warning: %q", want, got)
		}
	}
	if strings.Contains(strings.Join(w.changes(), "\n"), "given up") {
		t.Error("the failure is among the changes of files")
	}
}

// The failures that count are the ones in a row: a read that works begins the count again.
func TestSandboxFailuresInARow(t *testing.T) {
	var s streak
	for round := 0; round < 3; round++ {
		for i := 0; i < maxReadFailures-1; i++ {
			if s.fail() {
				t.Fatalf("given up after %d failures in round %d", i+1, round)
			}
		}
		s.ok()
	}
	for i := 0; i < maxReadFailures-1; i++ {
		s.fail()
	}
	if !s.fail() {
		t.Errorf("not given up after %d failures in a row", maxReadFailures)
	}
}

// A read that works begins the count again: failures that are many in all, and never maxReadFailures in a row, do not end the reading.
func TestSandboxFailuresThatAreNotInARowDoNotEndTheReading(t *testing.T) {
	var p [2]int
	if err := unix.Pipe(p[:]); err != nil {
		t.Fatal(err)
	}
	defer unix.Close(p[0])
	defer unix.Close(p[1])
	unix.Write(p[1], []byte("x")) // always readable
	w := newAttrWatcher(nil)
	calls := make(chan struct{}, 1000)
	k := 0
	w.recv = func(fd int, req *notifReq) unix.Errno {
		k++
		calls <- struct{}{}
		if k%maxReadFailures == 0 {
			return 0 // one that works (a number that no call of attrCalls has: it is looked at and nothing is found)
		}
		return unix.ENOTTY
	}
	done := make(chan struct{})
	returned := make(chan struct{})
	go func() { w.serve(p[0], done); close(returned) }()
	for i := 0; i < 5*maxReadFailures; i++ { // five rounds of failures, each one short of the limit
		select {
		case <-calls:
		case <-returned:
			t.Fatalf("the reading ended after %d calls, with no more than %d failures in a row", i, maxReadFailures-1)
		case <-time.After(5 * time.Second):
			t.Fatal("no progress")
		}
	}
	close(done)
	<-returned
	if got := w.warnings(); len(got) != 0 {
		t.Errorf("the reading was given up: %v", got)
	}
}

// The path of a change is resolved as the kernel does: `..` after a link is the parent of what the link points to, not of the link. (A path that is
// cleaned as a string first reads work/link/../victim as work/victim, which is inside, and the change of a file outside is not reported.)
func TestSandboxReportsTheFileAfterDotDotOfALink(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	sub := filepath.Join(f.outside, "sub")
	os.Mkdir(sub, 0o755)
	victim := filepath.Join(f.outside, "victim")
	os.WriteFile(victim, []byte("data"), 0o600)
	decoy := filepath.Join(f.work, "victim") // what the cleaned string would be
	os.WriteFile(decoy, []byte("data"), 0o600)
	if err := os.Symlink(sub, filepath.Join(f.work, "link")); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), Spec{Argv: []string{f.probe, "-attr-dotdot", f.work + "/link/../victim"}, Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"},
		Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(victim); err != nil || st.Mode().Perm() != 0o640 {
		t.Fatalf("the call did not reach the file outside (the test does not test what it says): %v %v\n%s", st, err, res.Stdout)
	}
	realVictim, _ := filepath.EvalSymlinks(victim)
	got := strings.Join(res.AttributeChanges, "\n")
	reported := false
	for _, l := range res.AttributeChanges {
		reported = reported || strings.HasSuffix(l, " "+realVictim) // (chmod, fchmodat, fchmodat2: it is the one of the library)
	}
	if !reported {
		t.Errorf("the change of %s is not reported:\n%s", realVictim, got)
	}
	if strings.Contains(got, decoy) {
		t.Errorf("the file that a cleaned string would name (%s) is reported:\n%s", decoy, got)
	}
}

// The descriptor of the notifications, received over the socket, is close-on-exec: another process that rivus starts while the run goes on must not
// get it (it would keep the listener open after the end of the serving, and could answer the notifications).
func TestSandboxReceivedDescriptorIsCloseOnExec(t *testing.T) {
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(pair[0])
	defer unix.Close(pair[1])
	var p [2]int
	if err := unix.Pipe(p[:]); err != nil { // (without O_CLOEXEC)
		t.Fatal(err)
	}
	defer unix.Close(p[0])
	defer unix.Close(p[1])
	if err := unix.Sendmsg(pair[1], []byte{0}, unix.UnixRights(p[0]), nil, 0); err != nil {
		t.Fatal(err)
	}
	fd, ok := receiveFd(pair[0])
	if !ok {
		t.Fatal("no descriptor")
	}
	defer unix.Close(fd)
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Errorf("the descriptor is not close-on-exec: %v %v", flags, err)
	}
}

// A call on a descriptor that is not a file (a pipe, a socket, a memfd) is not a change of a file: nothing is reported for it, and it is not "a path
// that could not be read" (which would blame the ptrace of the child, for a call that touched no file).
func TestSandboxCallsOnDescriptorsThatAreNotFilesAreNotReported(t *testing.T) {
	var p [2]int
	if err := unix.Pipe(p[:]); err != nil {
		t.Fatal(err)
	}
	defer unix.Close(p[0])
	defer unix.Close(p[1])
	mem, err := unix.MemfdCreate("x", 0)
	if err != nil {
		t.Skipf("no memfd: %v", err)
	}
	defer unix.Close(mem)
	w := newAttrWatcher([]string{"/nowhere"})
	var call attrCall
	for _, c := range attrCalls {
		if c.name == "fchmod" {
			call = c
		}
	}
	for name, fd := range map[string]int{"pipe": p[0], "memfd": mem} {
		var req notifReq
		req.data.args[0] = uint64(fd)
		if _, what := w.pathOf(os.Getpid(), &call, &req); what != pathNone {
			t.Errorf("%s: the descriptor is taken for %d, not for something that is not a file", name, what)
		}
	}
	if got := w.changes(); len(got) != 0 {
		t.Errorf("reported: %v", got)
	}
}

// What the run may write includes /dev/null (the baseline lets it): a change of its attributes is not a change outside.
func TestSandboxDevNullIsInside(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	res, err := Run(context.Background(), Spec{Argv: []string{f.probe, "-attr-null"}, Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Stdout), "attr-null-chmod") {
		t.Fatalf("the probe did not try: %s", res.Stdout)
	}
	if got := strings.Join(res.AttributeChanges, "\n"); strings.Contains(got, "/dev/null") {
		t.Errorf("/dev/null is reported:\n%s", got)
	}
}

// resolve is the standard library's EvalSymlinks, with the last name kept for the calls that are about a link itself.
func TestSandboxResolve(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "work"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "outside", "sub"), 0o755))
	must(os.WriteFile(filepath.Join(root, "outside", "victim"), nil, 0o600))
	must(os.WriteFile(filepath.Join(root, "work", "victim"), nil, 0o600))
	must(os.Symlink(filepath.Join(root, "outside", "sub"), filepath.Join(root, "work", "link")))
	must(os.Symlink(filepath.Join(root, "outside", "victim"), filepath.Join(root, "work", "vlink")))
	must(os.Symlink("vlink", filepath.Join(root, "work", "vlink2"))) // a link to a link, relative
	must(os.Symlink("loop", filepath.Join(root, "work", "loop")))
	outVictim := filepath.Join(root, "outside", "victim")
	for _, c := range []struct {
		name     string
		path     string
		nofollow bool
		want     string
	}{
		{"dotdot after a link", root + "/work/link/../victim", false, outVictim},
		{"dot and dotdot", root + "/work/link/./../victim", false, outVictim},
		{"the directory of a link", root + "/work/link/..", false, filepath.Join(root, "outside")},
		{"a link is followed", root + "/work/vlink", false, outVictim},
		{"a link to a link", root + "/work/vlink2", false, outVictim},
		{"the link itself", root + "/work/vlink", true, filepath.Join(root, "work", "vlink")},
		{"the link itself after a dotdot of a link", root + "/work/link/../vlink", true, filepath.Join(root, "outside", "vlink")},
		{"a name that is not there", root + "/work/link/nothing", false, filepath.Join(root, "outside", "sub", "nothing")},
		{"a loop does not hang", root + "/work/loop", false, filepath.Join(root, "work", "loop")},
	} {
		if got := resolve(c.path, c.nofollow); got != c.want {
			t.Errorf("%s: resolve(%q, %v) = %q, want %q", c.name, c.path, c.nofollow, got, c.want)
		}
	}
}

// A limit that the caller has set is never loosened: the new soft and hard limits are the smaller of the sandbox's and the caller's.
func TestSandboxLimitsAreNeverLoosened(t *testing.T) {
	const inf = ^uint64(0)
	for _, c := range []struct {
		name       string
		cur        unix.Rlimit
		v          uint64
		soft, hard uint64
	}{
		{"unlimited", unix.Rlimit{Cur: inf, Max: inf}, 100, 100, 100},
		{"the soft limit is stricter", unix.Rlimit{Cur: 10, Max: inf}, 100, 10, 100},
		{"both are stricter", unix.Rlimit{Cur: 10, Max: 50}, 100, 10, 50},
		{"the hard limit is stricter", unix.Rlimit{Cur: 50, Max: 60}, 100, 50, 60},
		{"the sandbox is stricter", unix.Rlimit{Cur: 500, Max: 600}, 100, 100, 100},
	} {
		got := lowered(c.cur, c.v)
		if got.Cur != c.soft || got.Max != c.hard {
			t.Errorf("%s: lowered(%+v, %d) = %+v, want soft %d, hard %d", c.name, c.cur, c.v, got, c.soft, c.hard)
		}
	}
}

// A relative path of the Spec is relative to the directory of the caller, not to the directory of the project (the helper opens the paths after it is in the
// project's directory).
func TestSandboxRelativePathsAreRelativeToTheCaller(t *testing.T) {
	skipWithoutSandbox(t)
	root, _ := filepath.EvalSymlinks(t.TempDir())
	os.MkdirAll(filepath.Join(root, "data"), 0o755)
	os.MkdirAll(filepath.Join(root, "proj", "data"), 0o755) // (another one, with the same name, in the project)
	os.WriteFile(filepath.Join(root, "data", "f"), []byte("caller"), 0o644)
	os.WriteFile(filepath.Join(root, "proj", "data", "f"), []byte("project"), 0o644)
	wd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	res, err := Run(context.Background(), Spec{Argv: []string{"/bin/sh", "-c", "cat " + filepath.Join(root, "data", "f")}, Dir: filepath.Join(root, "proj"),
		Env: []string{"PATH=/usr/bin:/bin"}, Read: []string{"data"}, ExecFiles: []string{"/bin/cat"}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Stdout) != "caller" {
		t.Errorf("the path 'data' is not the one of the caller: %q %q", res.Stdout, res.Stderr)
	}
}

// The calls whose path could not be read are not changes of files: they are said as a warning of their own, and the run that has only those does not
// say that files outside were changed.
func TestSandboxUnreadPathsAreAWarningNotAChange(t *testing.T) {
	w := newAttrWatcher(nil)
	w.unread = 3
	if got := w.changes(); len(got) != 0 {
		t.Errorf("the calls that could not be read are among the changes: %v", got)
	}
	got := strings.Join(w.warnings(), "\n")
	if !strings.Contains(got, "3") || !strings.Contains(got, "could not be checked") {
		t.Errorf("not said as a warning: %q", got)
	}
}

// The calls that Landlock does not mediate and that newer kernels have are watched too.
func TestSandboxWatchesTheNewerCallsToo(t *testing.T) {
	have := map[string]bool{}
	for _, c := range attrCalls {
		have[c.name] = true
	}
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
		for _, n := range []string{"setxattrat", "removexattrat", "file_setattr"} {
			if !have[n] {
				t.Errorf("%s is not watched", n)
			}
		}
	}
}

// A descriptor that the process which started rivus left open (not close-on-exec) is not passed to the program: Landlock and seccomp do not stop the use of
// a descriptor that is already open (a socket, a file opened for writing), so it must not be there.
func TestSandboxInheritedDescriptorsAreNotPassedOn(t *testing.T) {
	skipWithoutSandbox(t)
	f := setup(t)
	var p [2]int
	if err := unix.Pipe(p[:]); err != nil { // (flags 0: not close-on-exec)
		t.Fatal(err)
	}
	defer unix.Close(p[0])
	defer unix.Close(p[1])
	arg := []string{f.probe, "-only", "inherited-fd", "-inherited-fd", fmt.Sprint(p[1])}
	// without the sandbox the probe has it (the descriptor does leak: the test shows something)
	out, err := exec.Command(arg[0], arg[1:]...).CombinedOutput()
	if err != nil || !verdicts(t, string(out))["inherited-fd"] {
		t.Fatalf("on the host the probe does not see the descriptor (the test does not test what it says): %v %s", err, out)
	}
	res, err := Run(context.Background(), Spec{Argv: arg, Dir: f.work, Env: []string{"PATH=/usr/bin:/bin"}, Write: []string{f.work}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	v := verdicts(t, string(res.Stdout))
	if esc, ok := v["inherited-fd"]; !ok || esc {
		t.Errorf("the descriptor %d is passed on to the program: %v\n%s", p[1], v, res.Stdout)
	}
}
