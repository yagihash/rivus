package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/ir"
)

func testIso() *isolationOpts {
	return &isolationOpts{image: "img", memory: "6g", cpus: 2, pids: 100, timeout: time.Minute}
}

// Every container has the limits, and the stage that builds has no network and nothing that is writable of the project.
func TestIsolationArguments(t *testing.T) {
	o := testIso()
	extract := strings.Join(o.extractArgs("n", "/p/proj", "vol", []string{"--locked", "--offline", "/p/proj"}), " ")
	fetch := strings.Join(o.fetchArgs("n", "/p/proj", "vol"), " ")
	for _, want := range []string{"--cap-drop ALL", "--security-opt no-new-privileges", "--read-only", "--user 10001:10001", "--pids-limit 100", "--memory 6g", "--memory-swap 6g", "--cpus 2", "--rm"} {
		for name, args := range map[string]string{"extract": extract, "fetch": fetch} {
			if !strings.Contains(args, want) {
				t.Errorf("%s: %q is missing: %s", name, want, args)
			}
		}
	}
	if !strings.Contains(extract, "--network none") {
		t.Errorf("extract has a network: %s", extract)
	}
	if strings.Contains(fetch, "--network none") {
		t.Errorf("fetch has no network: %s", fetch)
	}
	for name, args := range map[string]string{"extract": extract, "fetch": fetch} {
		if !strings.Contains(args, "type=bind,src=/p/proj,dst=/p/proj,readonly") {
			t.Errorf("%s: the project is not read-only: %s", name, args)
		}
		for _, bad := range []string{"docker.sock", " -e ", "--env", "--privileged", "--network host", "--cap-add", "--pid ", "--volumes-from"} {
			if strings.Contains(args, bad) {
				t.Errorf("%s: %q: %s", name, bad, args)
			}
		}
	}
	if !strings.Contains(extract, "type=volume,src=vol,dst=/cargo-home,readonly") {
		t.Errorf("the dependencies are writable in the stage that builds: %s", extract)
	}
}

func TestMountable(t *testing.T) {
	if err := mountable("/a/b,c"); err == nil {
		t.Error("a comma in the name of the directory")
	}
	if home, err := os.UserHomeDir(); err == nil {
		if err := mountable(home); err == nil {
			t.Error("the home directory is mounted")
		}
		if err := mountable(filepath.Dir(home)); err == nil {
			t.Error("a directory that holds the home directory is mounted")
		}
	}
	if err := mountable(t.TempDir()); err != nil {
		t.Errorf("a directory of the project: %v", err)
	}
}

func TestProjectToolchain(t *testing.T) {
	dir := t.TempDir()
	if projectToolchain(dir) != "" {
		t.Error("no file")
	}
	os.WriteFile(filepath.Join(dir, "rust-toolchain.toml"), []byte("[toolchain]\nchannel = \"1.70.0\"\n"), 0o644)
	if got := projectToolchain(dir); got != "1.70.0" {
		t.Errorf("toml: %q", got)
	}
	dir = t.TempDir()
	os.WriteFile(filepath.Join(dir, "rust-toolchain"), []byte("nightly-2020-01-01\n"), 0o644)
	if got := projectToolchain(dir); got != "nightly-2020-01-01" {
		t.Errorf("plain: %q", got)
	}
}

func TestCargoVolumeNeedsALockFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := cargoVolume(dir); err == nil || !strings.Contains(err.Error(), "Cargo.lock") {
		t.Errorf("no lock file: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "Cargo.lock"), []byte("a"), 0o644)
	a, err := cargoVolume(dir)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "Cargo.lock"), []byte("b"), 0o644)
	if b, _ := cargoVolume(dir); a == b {
		t.Error("two lock files, one volume")
	}
}

// The project that is hostile on purpose is not run on this machine, whatever the flags are.
func TestHostileProjectIsRefusedOnTheHost(t *testing.T) {
	o := analyzeOpts{}
	o.dir = "../../extractor/rust/test/hostile"
	o.lang = "rust"
	o.unsafeHostExec = true
	o.rustExtractor = "/nonexistent/rivus-rust"
	_, err := o.runExtractor("rust")
	if err == nil || !strings.Contains(err.Error(), "hostile on purpose") {
		t.Errorf("not refused: %v", err)
	}
}

// Without Docker the code is not run: the error says so (the daemon is made unreachable).
func TestNoDockerIsNotARunOnTheHost(t *testing.T) {
	skipDockerPathOnMac(t)
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.lock"), []byte("version = 4\n"), 0o644)
	o := analyzeOpts{}
	o.dir = dir
	o.lang = "rust"
	o.containerMemory, o.containerPids, o.containerTimeout = "1g", 100, time.Minute
	_, err := o.runExtractor("rust")
	if err == nil || !strings.Contains(err.Error(), "-unsafe-host-exec") {
		t.Errorf("without docker: %v", err)
	}
}

func dockerForTests(t *testing.T) *isolationOpts {
	t.Helper()
	if os.Getenv("RIVUS_UNSAFE_HOST_EXEC") == "1" {
		t.Skip("RIVUS_UNSAFE_HOST_EXEC=1: the project that is hostile on purpose is not run here")
	}
	o := &isolationOpts{image: os.Getenv("RIVUS_RUST_IMAGE"), memory: "6g", pids: 4096, timeout: 20 * time.Minute}
	if o.image == "" {
		o.image = rustImageName
	}
	if err := o.checkDocker(context.Background()); err != nil {
		if os.Getenv("RIVUS_REQUIRE_DOCKER") == "1" {
			t.Fatalf("RIVUS_REQUIRE_DOCKER=1 says that the test must run: %v", err)
		}
		t.Skip(err)
	}
	return o
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	if out, err := exec.Command("cp", "-R", from, to).CombinedOutput(); err != nil {
		t.Fatalf("cp: %v: %s", err, out)
	}
	// the user of the container is not the user of this process (on Linux): the project is readable by anyone
	if out, err := exec.Command("chmod", "-R", "a+rX", filepath.Join(to)).CombinedOutput(); err != nil {
		t.Fatalf("chmod: %v: %s", err, out)
	}
}

var hostileReport = regexp.MustCompile(`HOSTILE (\w[\w.]*)=(BLOCKED|ESCAPED)`)

// hostileRun copies the hostile project next to a secret of the host, puts the secrets in the environment of this
// process, and extracts it in the container. mode is what the build script does at the end ("", "forkbomb", "hang", "memory").
func hostileRun(t *testing.T, o *isolationOpts, mode string) (proj, secret string, out []byte, stderr string, err error) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proj = filepath.Join(base, "proj")
	copyTree(t, "../../extractor/rust/test/hostile", proj)
	secret = "rivus-canary-" + randomName("")
	hostFile := filepath.Join(t.TempDir(), "host-secret")
	sibling := filepath.Join(base, "sibling-secret")
	for _, f := range []string{hostFile, sibling} {
		if err := os.WriteFile(f, []byte(secret), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(proj, "canary.txt"), []byte(hostFile+"\n"+sibling+"\n"+secret+"\n"+mode+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// what the build script looks for in the environment of the process that runs rivus
	for _, n := range []string{"GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "RIVUS_HOSTILE_CANARY", "ANTHROPIC_API_KEY", "CARGO_REGISTRY_TOKEN", "SSH_AUTH_SOCK"} {
		t.Setenv(n, secret)
	}
	var buf bytes.Buffer
	o.stderr = &buf
	out, err = o.extractRustIsolated(context.Background(), proj, []string{proj})
	return proj, secret, out, buf.String(), err
}

func noContainerLeft(t *testing.T) {
	t.Helper()
	out, _ := exec.Command("docker", "ps", "-q", "--filter", "name=rivus-extract-").Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("a container is left running: %s", out)
	}
}

// The build that does not stop, the one that forks without end, and the one that takes all the memory, stop in
// the container; this machine goes on.
func TestIsolationResourceLimits(t *testing.T) {
	skipDockerPathOnMac(t)
	o := dockerForTests(t)
	o.pids, o.memory, o.timeout = 64, "384m", 20*time.Second
	for _, c := range []struct{ mode, want string }{
		{"forkbomb", "did not finish|FORKBOMB stopped after"},
		{"memory", "signal: 9"},
		{"hang", "did not finish"},
	} {
		t.Run(c.mode, func(t *testing.T) {
			start := time.Now()
			_, _, _, stderr, err := hostileRun(t, o, c.mode)
			if err == nil {
				t.Fatalf("the build went through\n%s", stderr)
			}
			if !regexp.MustCompile(c.want).MatchString(err.Error() + stderr) {
				t.Errorf("want %q in: %v\n%s", c.want, err, stderr)
			}
			if d := time.Since(start); d > 3*time.Minute {
				t.Errorf("it took %v", d)
			}
			noContainerLeft(t)
		})
	}
}

func TestLimitWriter(t *testing.T) {
	var b bytes.Buffer
	w := &limitWriter{w: &b, max: 10}
	if _, err := w.Write([]byte("12345")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("678901")); err == nil {
		t.Error("the limit is not kept")
	}
}

// A build script and a proc macro that are hostile on purpose run in the container, and what they try fails.
func TestIsolationHostileBuild(t *testing.T) {
	skipDockerPathOnMac(t)
	o := dockerForTests(t)
	proj, secret, out, stderr, err := hostileRun(t, o, "")
	if err != nil {
		t.Fatalf("the extraction failed: %v\n%s", err, stderr)
	}
	seen := map[string]string{}
	for _, m := range hostileReport.FindAllStringSubmatch(stderr+string(out), -1) {
		seen[m[1]] = m[2]
	}
	for name, v := range seen {
		if v != "BLOCKED" {
			t.Errorf("%s: %s", name, v)
		}
	}
	for _, want := range []string{"read_host_file", "read_sibling_of_project", "list_parent_of_project", "network_1.1.1.1", "network_dns_name", "write_project", "write_root", "write_cargo_home",
		"env_value_of_host", "env_GITHUB_TOKEN", "env_home_is_the_hosts", "running_as_root", "has_capabilities", "macro_read_host_file", "macro_network", "macro_write_root"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("the attempt %s was not reported (the test says nothing about it)\n%s", want, stderr)
		}
	}
	// nothing of it reached this machine
	if _, err := os.Stat(filepath.Join("/tmp", "rivus-hostile-"+secret)); err == nil {
		t.Error("a file of the build script is in /tmp of the host")
	}
	for _, f := range []string{"pwned", "macro-wrote-here"} {
		if _, err := os.Stat(filepath.Join(proj, f)); err == nil {
			t.Errorf("%s is written in the project", f)
		}
	}
	prog, err := readExtracted(langRust, out)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range prog.Funcs {
		if f.ID == ir.FuncID("hostile::passes_through") {
			found = true
		}
	}
	if !found {
		t.Error("the function of the project is not in the IR")
	}
	if strings.Contains(string(out), secret) {
		t.Error("the secret is in the IR")
	}
	// what the extractor reads from the directory of the fragments (links, a big file, a name it does not know) is
	// the container's own: nothing of it comes back, and the extraction goes on
	if strings.Contains(string(out), "root:x:0:0") {
		t.Error("a link in the fragments read a file into the IR")
	}
	checkWhatIsSeen(t, string(out))
}

// What can be seen from the inside: the names of the environment, the mounts, and the files that can be read.
func checkWhatIsSeen(t *testing.T, report string) {
	t.Helper()
	field := func(key string) []string {
		var vals []string
		for _, part := range strings.Split(report, ";") {
			if i := strings.Index(part, key+" "); i >= 0 {
				v := part[i+len(key)+1:]
				if j := strings.IndexAny(v, "\"\\"); j >= 0 {
					v = v[:j]
				}
				vals = append(vals, v)
			}
		}
		return vals
	}
	// (1) mounts: the project and the dependencies, read-only; the work volume; nothing of the host's home
	okMount := map[string]bool{"/": true, "/proc": true, "/dev": true, "/sys": true, "/etc/hosts": true, "/etc/hostname": true, "/etc/resolv.conf": true, "/tmp": true,
		"/work": true, "/cargo-home": true}
	for _, m := range field("MOUNT") {
		dst, mode, _ := strings.Cut(m, " ")
		switch {
		case okMount[dst], strings.HasPrefix(dst, "/proc/"), strings.HasPrefix(dst, "/dev/"), strings.HasPrefix(dst, "/sys/"):
		case strings.Contains(dst, "/proj"):
			if mode != "ro" {
				t.Errorf("the project is writable: %s", m)
			}
		default:
			t.Errorf("a mount that is not expected: %s", m)
		}
		if dst == "/cargo-home" && mode != "ro" {
			t.Errorf("the dependencies are writable in the stage that builds: %s", m)
		}
		for _, bad := range []string{"/Users", "/home", "/root", "docker.sock", ".ssh", ".cargo"} {
			if strings.Contains(dst, bad) {
				t.Errorf("a mount of the host: %s", m)
			}
		}
	}
	if len(field("MOUNT")) == 0 {
		t.Error("the mounts were not listed (the test says nothing)")
	}
	// (2) the environment: the names that the image and cargo set, and nothing of the host
	for _, list := range field("ENV_NAMES") {
		for _, n := range strings.Split(list, ",") {
			ok := false
			for _, pre := range []string{"CARGO", "RUSTUP", "RUSTC", "RUSTDOC", "RUSTFLAGS", "RIVUS_", "CFG_", "OUT_DIR", "TARGET", "HOST", "PROFILE", "OPT_LEVEL", "DEBUG", "NUM_JOBS", "LD_LIBRARY_PATH", "DYLD_", "PATH", "HOME", "TMPDIR", "HOSTNAME", "HOSTILE", "PWD", "SHLVL", "_", "NUM_", "LLVM", "DOCS_RS", "MAKEFLAGS", "CLIPPY", "RUST_", "SSL_CERT_"} {
				if strings.HasPrefix(n, pre) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("an environment variable that is not expected: %s", n)
			}
			for _, bad := range []string{"TOKEN", "SECRET", "SSH_", "AWS_", "ANTHROPIC", "GITHUB", "KEY", "PASSWORD"} {
				if strings.Contains(n, bad) {
					t.Errorf("an environment variable of the host: %s", n)
				}
			}
		}
	}
	if len(field("ENV_NAMES")) == 0 {
		t.Error("the environment was not listed (the test says nothing)")
	}
	// (3) the files that can be read: none of the kind that holds secrets
	if s := field("SUSPICIOUS"); len(s) == 0 {
		t.Error("the files were not listed (the test says nothing)")
	} else {
		for _, v := range s {
			if strings.TrimSpace(v) != "" {
				t.Errorf("files of the kind that hold secrets can be read: %s", v)
			}
		}
	}
	for _, n := range field("FILES_READABLE") {
		t.Logf("files that can be read from the container: %s", n)
	}
}

// skipDockerPathOnMac: on macOS the extraction of Rust runs under the sandbox (isolate_sandbox_darwin.go), not in a container, so a test of the
// container path has nothing to run. The container path goes (docs/DESIGN-macos-sandbox.md), and the sandbox has tests of its own
// (internal/sandbox, TestSandboxedBuildScriptCannotEscape).
func skipDockerPathOnMac(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("the container path is not the isolation on macOS any more: the sandbox is (see internal/sandbox)")
	}
}
