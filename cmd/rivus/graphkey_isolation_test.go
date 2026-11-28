package main

import (
	"strings"
	"testing"
)

// The settings of the isolation (-isolation-env, -isolation-read) change what the extraction reads (a cfg that RUSTFLAGS sets, a file that a build
// script reads), so an index made with other settings is not this one: they are in the key. The values of the variables can be secret (a token that a
// build script needs), and the key is written into the graph file: the name is kept, the value is its hash.
func TestIsolationSettingsAreInTheKey(t *testing.T) {
	plain := map[string]string{}
	isolationConfig(plain, nil, nil)
	if len(plain) != 0 {
		t.Errorf("an index made without them has the key that it had: %v", plain)
	}
	a, b, c := map[string]string{}, map[string]string{}, map[string]string{}
	isolationConfig(a, []string{"RUSTFLAGS=--cfg x"}, nil)
	isolationConfig(b, []string{"RUSTFLAGS=--cfg y"}, nil)
	isolationConfig(c, nil, []string{"/opt/data"})
	if len(a) == 0 || a["flags.isolation-env"] == b["flags.isolation-env"] {
		t.Errorf("another value is another key: %v %v", a, b)
	}
	if c["flags.isolation-read"] != "/opt/data" {
		t.Errorf("the path that may be read: %v", c)
	}
	if !strings.HasPrefix(a["flags.isolation-env"], "RUSTFLAGS=") {
		t.Errorf("the name is kept: %q", a["flags.isolation-env"])
	}
	if strings.Contains(a["flags.isolation-env"], "cfg x") {
		t.Errorf("the value is written into the key: %q", a["flags.isolation-env"])
	}
}

// The order and the repeats do not matter.
func TestIsolationSettingsOrderDoesNotMatter(t *testing.T) {
	a, b := map[string]string{}, map[string]string{}
	isolationConfig(a, []string{"A=1", "B=2", "A=1"}, []string{"/y", "/x"})
	isolationConfig(b, []string{"B=2", "A=1"}, []string{"/x", "/y", "/x"})
	for _, k := range []string{"flags.isolation-env", "flags.isolation-read"} {
		if a[k] != b[k] {
			t.Errorf("%s: %q and %q", k, a[k], b[k])
		}
	}
}
