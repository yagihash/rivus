//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

var allChecks = []string{"read-secret", "write-outside", "tcp", "tcp-other-port", "udp", "unix-socket", "exec-other", "proc-environ", "io-uring", "ptrace", "unshare", "clone-newns", "clone3"}

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
	for _, c := range []string{"tcp-other-port", "udp", "unix-socket", "read-secret", "write-outside"} {
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
	if len(c.Exec) != 0 {
		t.Errorf("the baseline runs programs from directories: %v", c.Exec)
	}
	if len(loaders()) == 0 {
		t.Skip("no dynamic loader found on this system")
	}
	for _, l := range loaders() {
		found := false
		for _, f := range c.ExecFiles {
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
