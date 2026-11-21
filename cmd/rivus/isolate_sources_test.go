package main

import (
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
		"git+https://github.com/someone/repo?rev=abc#0123456789abcdef0123456789abcdef01234567",
		"git+https://gitlab.example.org/a/b.git#0123456789abcdef0123456789abcdef01234567",
	}
	if err := checkFetchSources(lockWith(ok...), nil); err != nil {
		t.Errorf("the default places are refused: %v", err)
	}
	for _, bad := range []string{
		"registry+sparse+http://10.0.0.5:8080/",
		"registry+https://registry.example.com/index",
		"registry+http://registry.corp/",
		"git+http://169.254.169.254/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://127.0.0.1/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://[::1]/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://localhost/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://nas.local/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://git.internal/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://intranet/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://2130706433/x.git#0123456789abcdef0123456789abcdef01234567",
		"git+https://user:pw@github.com/x/y#0123456789abcdef0123456789abcdef01234567",
		"git+ssh://github.com/x/y#0123456789abcdef0123456789abcdef01234567",
	} {
		err := checkFetchSources(lockWith(bad), nil)
		if err == nil || !strings.Contains(err.Error(), bad) {
			t.Errorf("not refused (or not listed): %s: %v", bad, err)
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
