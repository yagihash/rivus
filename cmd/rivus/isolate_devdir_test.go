package main

import (
	"strings"
	"testing"
)

func TestSandboxDeveloperDirDecision(t *testing.T) {
	t.Parallel()
	xcode := "/Applications/Xcode.app/Contents/Developer"
	cases := []struct {
		name       string
		f          developerDirFacts
		wantValue  string
		wantSource string
		wantErr    bool
	}{
		{"user set is respected", developerDirFacts{goos: "darwin", userEnv: []string{"A=b", "DEVELOPER_DIR=/opt/dev"}, cltExists: true, selected: xcode}, "/opt/dev", "user", false},
		{"user set is respected without the tools", developerDirFacts{goos: "darwin", userEnv: []string{"DEVELOPER_DIR=/opt/dev"}, selected: xcode}, "/opt/dev", "user", false},
		{"tools exist: set", developerDirFacts{goos: "darwin", cltExists: true, selected: xcode}, cltDir, "default", false},
		{"tools missing, Xcode selected: error", developerDirFacts{goos: "darwin", selected: xcode}, "", "", true},
		{"tools missing, nothing selected: nothing", developerDirFacts{goos: "darwin"}, "", "", false},
		{"tools missing, other dir selected: nothing", developerDirFacts{goos: "darwin", selected: "/opt/dev"}, "", "", false},
		{"not macOS: unchanged", developerDirFacts{goos: "linux", cltExists: true, selected: xcode}, "", "", false},
	}
	for _, c := range cases {
		v, s, err := decideDeveloperDir(c.f)
		if (err != nil) != c.wantErr || v != c.wantValue || s != c.wantSource {
			t.Errorf("%s: got (%q, %q, %v), want (%q, %q, err=%v)", c.name, v, s, err, c.wantValue, c.wantSource, c.wantErr)
		}
		if err != nil {
			for _, w := range []string{"xcode-select --install", "-isolation-env DEVELOPER_DIR", "Nothing was run"} {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("%s: the message lacks %q: %v", c.name, w, err)
				}
			}
		}
	}
}
