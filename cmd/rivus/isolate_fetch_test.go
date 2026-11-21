package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The stage that fetches has a network. What the project says in its own cargo configuration must not make cargo run
// a program of the project in that stage (a credential provider, git) or send it somewhere the project chose.
//
// The tests below run in the container only. The "program of the project" is a script that writes a marker file and
// does nothing else; the "registry" is a small server in a second container of the same image that answers every
// request with a demand for credentials, and that is reached on the bridge network of Docker (never on this host).

const fakeRegistryPerl = `$|=1; use IO::Socket::INET; my $s=IO::Socket::INET->new(Listen=>5,LocalPort=>8080,ReuseAddr=>1) or die;
while (my $c=$s->accept) { my $l=<$c>; print "REQ $l"; while (<$c>) { last if /^\r?\n$/ }
print $c "HTTP/1.1 401 Unauthorized\r\nWWW-Authenticate: Cargo login_url=\"http://registry.invalid/me\"\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"; close $c }`

const markerScript = "#!/bin/sh\necho ran > /cargo-home/PROVIDER-RAN\nexit 1\n"

// the lock file of a crate that has one dependency of crates.io (itoa 1.0.11); the proxy test needs a real checksum
const lockCratesIO = `version = 4

[[package]]
name = "itoa"
version = "1.0.11"
source = "registry+https://github.com/rust-lang/crates.io-index"
checksum = "49f1f14873335454500d59611f1cf4a4b0f786f9ac11f4312a78e4cf2566695b"

[[package]]
name = "t"
version = "0.1.0"
dependencies = [
 "itoa",
]
`

const manifestCratesIO = "[package]\nname = \"t\"\nversion = \"0.1.0\"\nedition = \"2021\"\n[dependencies]\nitoa = \"=1.0.11\"\n"

const lockEvilRegistry = `version = 4

[[package]]
name = "evil-dep"
version = "1.0.0"
source = "registry+sparse+http://@IP@:8080/"
checksum = "0000000000000000000000000000000000000000000000000000000000000000"

[[package]]
name = "t"
version = "0.1.0"
dependencies = [
 "evil-dep",
]
`

const manifestEvilRegistry = "[package]\nname = \"t\"\nversion = \"0.1.0\"\nedition = \"2021\"\n[dependencies]\nevil-dep = { version = \"1\", registry = \"evil\" }\n"

type fetchCase struct {
	name     string
	manifest string
	lock     string
	config   string // .cargo/config.toml; @IP@ is the fake registry, @DIR@ the project
	// proxy: the case is seen as a request to the fake registry (the project points the proxy at it)
	proxy bool
}

var fetchCases = []fetchCase{
	{
		name: "credential-provider", manifest: manifestEvilRegistry, lock: lockEvilRegistry,
		config: "[registries.evil]\nindex = \"sparse+http://@IP@:8080/\"\ncredential-provider = [\"@DIR@/prov.sh\"]\n",
	},
	{
		name: "global-credential-providers-and-source-replacement", manifest: manifestCratesIO, lock: lockCratesIO,
		config: "[registry]\nglobal-credential-providers = [\"@DIR@/prov.sh\"]\n[source.crates-io]\nreplace-with = \"evil\"\n[source.evil]\nregistry = \"sparse+http://@IP@:8080/\"\n",
	},
	{
		name: "credential-alias", manifest: manifestEvilRegistry, lock: lockEvilRegistry,
		config: "[credential-alias]\nevilcred = \"@DIR@/prov.sh\"\n[registries.evil]\nindex = \"sparse+http://@IP@:8080/\"\ncredential-provider = [\"evilcred\"]\n",
	},
	{
		name: "git-fetch-with-cli", manifest: "[package]\nname = \"t\"\nversion = \"0.1.0\"\nedition = \"2021\"\n[dependencies]\ngdep = { git = \"http://@IP@:8080/g.git\" }\n",
		lock:   "version = 4\n\n[[package]]\nname = \"gdep\"\nversion = \"0.1.0\"\nsource = \"git+http://@IP@:8080/g.git#0123456789abcdef0123456789abcdef01234567\"\n\n[[package]]\nname = \"t\"\nversion = \"0.1.0\"\ndependencies = [\n \"gdep\",\n]\n",
		config: "[net]\ngit-fetch-with-cli = true\n[credential-alias]\nx = \"@DIR@/prov.sh\"\n",
	},
	{
		name: "http-proxy", manifest: manifestCratesIO, lock: lockCratesIO, proxy: true,
		config: "[http]\nproxy = \"http://@IP@:8080\"\n",
	},
}

func dockerOut(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// startFakeRegistry runs the server in a container and returns its name and its address on the bridge network.
func startFakeRegistry(t *testing.T, o *isolationOpts) (name, ip string) {
	t.Helper()
	name = randomName("rivus-fakereg-")
	if out, err := dockerOut(t, "run", "-d", "--rm", "--name", name, "--user", "10001:10001", "--cap-drop", "ALL", "--read-only",
		"--security-opt", "no-new-privileges", "--memory", "128m", "--pids-limit", "64", "--label", "rivus=test-fake-registry",
		"--entrypoint", "perl", o.image, "-e", fakeRegistryPerl); err != nil {
		t.Fatalf("the fake registry did not start: %v: %s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	ip, err := dockerOut(t, "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", name)
	if err != nil || ip == "" {
		t.Fatalf("no address for the fake registry: %v: %s", err, ip)
	}
	time.Sleep(time.Second)
	return name, ip
}

// fetchProject writes the project of the case, runs the stage that fetches on it (as rivus does) and returns
// what is in the volume afterwards, what the fake registry was asked, and what the container said.
func fetchProject(t *testing.T, o *isolationOpts, c fetchCase, fetch func(name, dir, volume string) []string) (volumeFiles, registryLog, said string) {
	t.Helper()
	regName, ip := startFakeRegistry(t, o)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "proj")
	for f, body := range map[string]string{
		"Cargo.toml":         c.manifest,
		"Cargo.lock":         c.lock,
		"src/lib.rs":         "",
		".cargo/config.toml": c.config,
		"prov.sh":            markerScript,
	} {
		body = strings.NewReplacer("@IP@", ip, "@DIR@", dir).Replace(body)
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if f == "prov.sh" {
			mode = 0o755
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("chmod", "-R", "a+rX", base).CombinedOutput(); err != nil {
		t.Fatalf("chmod: %v: %s", err, out)
	}
	volume, err := cargoVolume(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "volume", "rm", "-f", volume).Run() })
	var buf strings.Builder
	o.stderr = &buf
	o.timeout = 2 * time.Minute
	name := randomName("rivus-fetch-")
	_ = o.runContainer(context.Background(), name, fetch(name, dir, volume), io.Discard, "fetch")
	said = buf.String()
	volumeFiles, _ = dockerOut(t, "run", "--rm", "--network", "none", "--user", "10001:10001", "--mount", "type=volume,src="+volume+",dst=/v,readonly",
		"--entrypoint", "ls", o.image, "-a", "/v")
	registryLog, _ = dockerOut(t, "logs", regName)
	return
}

// Before the fix this is red: the script of the project runs in the stage that has a network.
func TestIsolationFetchRunsNoCodeOfTheProject(t *testing.T) {
	o := dockerForTests(t)
	for _, c := range fetchCases {
		t.Run(c.name, func(t *testing.T) {
			files, reglog, said := fetchProject(t, o, c, func(name, dir, volume string) []string { return o.fetchArgs(name, dir, volume) })
			if strings.Contains(files, "PROVIDER-RAN") {
				t.Errorf("a program of the project ran in the stage that fetches (the credential provider wrote its marker in the volume)\n%s", said)
			}
			if c.proxy && strings.Contains(reglog, "CONNECT") {
				t.Errorf("the proxy the project chose got a request: %s", reglog)
			}
		})
	}
}

// The image has no git: `net.git-fetch-with-cli` has no program to run. If git is ever put in the image, this fails
// and the fetch stage has to be looked at again.
func TestIsolationImageHasNoGit(t *testing.T) {
	o := dockerForTests(t)
	out, err := exec.Command("docker", "run", "--rm", "--network", "none", "--entrypoint", "sh", o.image, "-c", "command -v git").CombinedOutput()
	if err == nil {
		t.Errorf("the image has git (%s): the cargo configuration of the project could make the fetch stage run it", strings.TrimSpace(string(out)))
	}
}
