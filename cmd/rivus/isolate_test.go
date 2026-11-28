package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMountable(t *testing.T) {
	t.Parallel()
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

// The cargo configuration and the credentials in the CARGO_HOME are removed also when the path of the CARGO_HOME has [ * or ? in it: they made
// filepath.Glob fail (or match other files), the error was dropped, and the files stayed for the stage that has a network.
func TestDropConfigOfCargoHomeWithMetacharactersInThePath(t *testing.T) {
	for _, name := range []string{"plain", "x[y", "a*b", "q?r"} {
		home := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(filepath.Join(home, "registry"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"config.toml", "config", "credentials.toml", "credentials"} {
			if err := os.WriteFile(filepath.Join(home, f), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		var said strings.Builder
		if err := dropConfigOfCargoHome(home, &said); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, f := range []string{"config.toml", "config", "credentials.toml", "credentials"} {
			if _, err := os.Lstat(filepath.Join(home, f)); err == nil {
				t.Errorf("%s: %s is still there", name, f)
			}
		}
		if _, err := os.Lstat(filepath.Join(home, "registry")); err != nil {
			t.Errorf("%s: the registry is removed: %v", name, err)
		}
	}
	// a CARGO_HOME that is not there yet is nothing
	if err := dropConfigOfCargoHome(filepath.Join(t.TempDir(), "none"), &strings.Builder{}); err != nil {
		t.Errorf("not there: %v", err)
	}
}

func TestCheckRead(t *testing.T) {
	t.Parallel()
	if err := checkRead([]string{t.TempDir()}); err != nil {
		t.Errorf("a directory outside the home directory: %v", err)
	}
	if err := checkRead(nil); err != nil {
		t.Errorf("none: %v", err)
	}
	for _, bad := range []string{"", "  "} {
		if err := checkRead([]string{t.TempDir(), bad}); err == nil {
			t.Errorf("%q (an unset variable) is the current directory, and was let through", bad)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, p := range []string{home, filepath.Dir(home)} {
			if err := checkRead([]string{p}); err == nil {
				t.Errorf("%s holds what is not the project, and was let through", p)
			}
		}
	}
}

func TestProjectToolchain(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	o := analyzeOpts{}
	o.dir = "../../extractor/rust/test/hostile_sandbox"
	o.lang = "rust"
	o.unsafeHostExec = true
	o.rustExtractor = "/nonexistent/rivus-rust"
	_, err := o.runExtractor("rust")
	if err == nil || !strings.Contains(err.Error(), "hostile on purpose") {
		t.Errorf("not refused: %v", err)
	}
}

// TestRustDataDirOfTheTestsIsNotTheUsers: the tests must not write the CARGO_HOME of the sandbox (rivus-cargo-*) to the directory of
// the user (~/Library/Application Support/rivus); TestMain points RIVUS_RUST_DATA_DIR at a temporary directory.
func TestRustDataDirOfTheTestsIsNotTheUsers(t *testing.T) {
	got, err := rustDataDir()
	if err != nil {
		t.Fatal(err)
	}
	base, _ := os.UserConfigDir()
	if base != "" && strings.HasPrefix(got, filepath.Join(base, "rivus")) {
		t.Errorf("rustDataDir() = %s: the tests write to the directory of the user", got)
	}
}

// mountable compares the real paths: a link to the home directory, a relative path, and a link to a directory that holds it are not given to the build.
func TestMountableComparesRealPaths(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "users", "u")
	proj := filepath.Join(root, "elsewhere", "proj")
	for _, d := range []string{home, proj} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	linkHome := filepath.Join(root, "linkhome")
	linkUsers := filepath.Join(root, "linkusers")
	if err := os.Symlink(home, linkHome); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(home), linkUsers); err != nil {
		t.Fatal(err)
	}
	// HOME is given through a link, the directory by its real path (and the other way round)
	homeViaLink := filepath.Join(root, "linkusers", "u")
	t.Setenv("HOME", homeViaLink)
	for name, dir := range map[string]string{"real path of home": home, "link to home": linkHome, "link to a parent": linkUsers, "parent": filepath.Dir(home)} {
		if err := mountable(dir); err == nil {
			t.Errorf("%s: mounted", name)
		}
	}
	t.Setenv("HOME", home)
	if err := mountable(homeViaLink); err == nil {
		t.Error("home through a link: mounted")
	}
	// a relative path
	t.Chdir(filepath.Dir(home))
	for _, rel := range []string{"u", ".", "../users/u"} {
		if err := mountable(rel); err == nil {
			t.Errorf("relative %q: mounted", rel)
		}
	}
	if err := mountable(proj); err != nil {
		t.Errorf("a project outside the home: %v", err)
	}
	if err := mountable(filepath.Join(home, "proj")); err != nil {
		t.Errorf("a project inside the home is a subdirectory, it is not the home or above: %v", err)
	}
	t.Chdir(filepath.Dir(proj))
	if err := mountable("proj"); err != nil {
		t.Errorf("relative project: %v", err)
	}
	// a name that begins with two dots is not "outside"
	t.Setenv("HOME", filepath.Join(root, "..odd"))
	if err := mountable(proj); err != nil {
		t.Errorf("project: %v", err)
	}
}

// The stage that fetches has a network: the values of -isolation-env are given to the stage that extracts only.
func TestStageEnvKeepsUserEnvOutOfTheFetch(t *testing.T) {
	t.Parallel()
	base := []string{"PATH=/usr/bin", "HOME=/w"}
	user := []string{"HTTPS_PROXY=http://x", "LOCAL_ASSETS_PATH=/a"}
	if got := stageEnv(append([]string(nil), base...), user, true); strings.Join(got, " ") != strings.Join(base, " ") {
		t.Errorf("fetch: %v", got)
	}
	if got := stageEnv(append([]string(nil), base...), user, false); strings.Join(got, " ") != "PATH=/usr/bin HOME=/w HTTPS_PROXY=http://x LOCAL_ASSETS_PATH=/a" {
		t.Errorf("extract: %v", got)
	}
}
