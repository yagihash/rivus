package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lockWith(sources ...string) string {
	var b strings.Builder
	b.WriteString("version = 4\n")
	for i, s := range sources {
		b.WriteString("\n[[package]]\nname = \"p" + string(rune('a'+i)) + "\"\nversion = \"1.0.0\"\n")
		if s != "" {
			b.WriteString("source = \"" + s + "\"\n")
		}
	}
	return b.String()
}

func TestCheckFetchSources(t *testing.T) {
	ok := []string{
		"", // a crate of the project itself has no source
		"registry+https://github.com/rust-lang/crates.io-index",
		"registry+sparse+https://index.crates.io/",
	}
	if err := checkFetchSources(lockWith(ok...), nil); err != nil {
		t.Errorf("crates.io is refused: %v", err)
	}
	// everything else is refused by default: a git dependency on any host (a public one too), a registry, any scheme
	for _, bad := range []string{
		"git+https://github.com/someone/repo?rev=abc#0123456789abcdef0123456789abcdef01234567",
		"git+https://gitlab.example.org/a/b.git#0123456789abcdef0123456789abcdef01234567",
		"registry+sparse+http://10.0.0.5:8080/",
		"registry+https://registry.example.com/index",
		"registry+http://registry.corp/",
		"git+http://169.254.169.254/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://127.0.0.1/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://[::1]/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://localhost/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://nas.local/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://2130706433/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://user:pw@github.com/x/y#0123456789abcdef0123456789abcdef01234567",
		"git+ssh://github.com/x/y#0123456789abcdef0123456789abcdef01234567",
	} {
		err := checkFetchSources(lockWith(bad), nil)
		if err == nil || !strings.Contains(err.Error(), strings.TrimPrefix(strings.SplitN(bad, "#", 2)[0], "git+")) {
			t.Errorf("not refused (or not listed): %s: %v", bad, err)
		}
	}
	// the message says which repository at which commit, and how to allow it, in one line
	err := checkFetchSources(lockWith("git+https://github.com/someone/repo?rev=abc#0123456789abcdef0123456789abcdef01234567"), nil)
	for _, want := range []string{"pa 1.0.0: https://github.com/someone/repo?rev=abc at commit 0123456789abcdef0123456789abcdef01234567", "-allow-fetch-host github.com", "submodules", "redirects"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("the message lacks %q: %v", want, err)
		}
	}
	// named by the user: allowed, by host
	if err := checkFetchSources(lockWith("registry+https://registry.example.com/index", "git+http://10.0.0.5/x.git#0123456789abcdef0123456789abcdef01234567"),
		[]string{"registry.example.com", "10.0.0.5"}); err != nil {
		t.Errorf("hosts that were named are refused: %v", err)
	}
	if err := checkFetchSources(lockWith("git+http://10.0.0.5/x.git#0123456789abcdef0123456789abcdef01234567"), []string{"registry.example.com"}); err == nil {
		t.Error("another host was allowed")
	}
}

// The manifests name places too, and cargo asks them before it finds that Cargo.lock does not match (--locked): a manifest that
// names a git repository which Cargo.lock lacks sends the fetch there. The check reads the manifests.
func TestCheckManifestSources(t *testing.T) {
	const pkg = "[package]\nname = \"t\"\nversion = \"0.1.0\"\nedition = \"2021\"\n"
	cases := []struct {
		name  string
		files map[string]string
		allow []string
		bad   string // part of the message; "" means allowed
	}{
		{"crates.io only", map[string]string{"Cargo.toml": pkg + "[dependencies]\nitoa = \"1\"\nlocal = { path = \"../x\" }\n[package.metadata.docs]\ngit = \"https://example.com/\"\n"}, nil, ""},
		{"git dependency", map[string]string{"Cargo.toml": pkg + "[dependencies]\ng = { git = \"https://github.com/a/b\" }\n"}, nil, `git = "https://github.com/a/b"`},
		{"git dependency, host allowed", map[string]string{"Cargo.toml": pkg + "[dependencies]\ng = { git = \"https://github.com/a/b\" }\n"}, []string{"github.com"}, ""},
		{"in a member of the workspace", map[string]string{"Cargo.toml": "[workspace]\nmembers = [\"m\"]\n", "m/Cargo.toml": pkg + "[target.'cfg(unix)'.dependencies]\ng = { git = \"http://10.0.0.5/x\" }\n"}, nil, "m/Cargo.toml"},
		{"quoted key with an escape", map[string]string{"Cargo.toml": pkg + "[dependencies]\ng = { \"\\u0067it\" = \"http://10.0.0.5/x\" }\n"}, nil, "10.0.0.5"},
		{"dotted key", map[string]string{"Cargo.toml": pkg + "[dependencies]\ng.git = \"http://10.0.0.5/x\"\n"}, nil, "10.0.0.5"},
		{"patch of a source", map[string]string{"Cargo.toml": pkg + "[patch.\"https://evil.example/x\"]\ny = { path = \"p\" }\n"}, nil, "evil.example"},
		{"patch with git", map[string]string{"Cargo.toml": pkg + "[patch.crates-io]\nitoa = { git = \"http://10.0.0.5/x\" }\n"}, nil, "10.0.0.5"},
		{"registry-index", map[string]string{"Cargo.toml": pkg + "[dependencies]\ng = { version = \"1\", registry-index = \"http://10.0.0.5/\" }\n"}, nil, "registry-index"},
		{"not TOML", map[string]string{"Cargo.toml": pkg + "[dependencies\n"}, nil, "cannot be read as TOML"},
		{"build directories are skipped", map[string]string{"Cargo.toml": pkg, "target/x/Cargo.toml": "[dependencies]\ng = { git = \"http://10.0.0.5/x\" }\n"}, nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for f, body := range c.files {
				p := filepath.Join(dir, f)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			err := checkManifestSources(dir, c.allow)
			switch {
			case c.bad == "" && err != nil:
				t.Errorf("refused: %v", err)
			case c.bad != "" && (err == nil || !strings.Contains(err.Error(), c.bad)):
				t.Errorf("not refused (or not listed: %q): %v", c.bad, err)
			}
		})
	}
}
