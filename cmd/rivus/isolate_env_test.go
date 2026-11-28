package main

import (
	"reflect"
	"strings"
	"testing"
)

// -isolation-env gives the build variables, and a name that rivus sets for the sandbox (PATH, HOME, TMPDIR, CARGO_HOME, RIVUS_RUST_DIRECT,
// RUSTUP_TOOLCHAIN) is refused: with it in the list twice, the one that was used depended on the system (Linux: the first, by getenv; macOS: the
// last, after the dedup of os/exec), and these names are what keeps the build to the work directory and to the pinned toolchain.
func TestCheckIsolationEnv(t *testing.T) {
	for _, ok := range [][]string{nil, {"LOCAL_ASSETS_PATH=/x"}, {"A=", "B_2=a=b", "DEVELOPER_DIR=/opt/dev", "SDKROOT=/sdk"}} {
		if err := checkIsolationEnv(ok); err != nil {
			t.Errorf("%v: %v", ok, err)
		}
	}
	for _, bad := range []struct{ in, why string }{
		{"FOO", "no ="},
		{"=x", "no name"},
		{"1A=x", "starts with a digit"},
		{"A B=x", "a space in the name"},
		{"A-B=x", "a dash in the name"},
		{"A=x\x00y", "a NUL in the value"},
		{"PATH=/usr/bin", "set by rivus"},
		{"HOME=/", "set by rivus"},
		{"TMPDIR=/", "set by rivus"},
		{"CARGO_HOME=/x", "set by rivus"},
		{"RIVUS_RUST_DIRECT=0", "set by rivus"},
		{"RUSTUP_TOOLCHAIN=stable", "set by rivus"},
	} {
		err := checkIsolationEnv([]string{"OK=1", bad.in})
		if err == nil {
			t.Errorf("%q (%s) was let through", bad.in, bad.why)
			continue
		}
		if !strings.Contains(err.Error(), "-isolation-env") {
			t.Errorf("%q: the message does not name the flag: %v", bad.in, err)
		}
	}
}

// The environment of a stage has each name once, and the same list on every system: a name that the user gives (DEVELOPER_DIR, SDKROOT, which
// rivus also sets by itself on macOS) takes the place of the one of rivus; the user's own repeats are decided by the last.
func TestStageEnvEachNameOnce(t *testing.T) {
	base := []string{"PATH=/tc/bin", "HOME=/w", "SDKROOT=/default-sdk", "DEVELOPER_DIR=/default-dev"}
	user := []string{"SDKROOT=/mine", "X=1", "X=2", "DEVELOPER_DIR=/user-dev"}
	got := stageEnv(base, user, false)
	want := []string{"PATH=/tc/bin", "HOME=/w", "SDKROOT=/mine", "X=2", "DEVELOPER_DIR=/user-dev"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
	// the stage that fetches has none of the user's
	if got := stageEnv(base, user, true); !reflect.DeepEqual(got, base) {
		t.Errorf("fetch: %v", got)
	}
	// the input is not changed (append on a slice that has room)
	b2 := make([]string, 2, 10)
	copy(b2, []string{"A=1", "B=2"})
	_ = stageEnv(b2, []string{"C=3"}, false)
	if len(b2) != 2 || cap(b2) != 10 {
		t.Errorf("base: %v", b2)
	}
	b3 := stageEnv(b2, []string{"C=3"}, false)
	b4 := stageEnv(b2, []string{"D=4"}, false)
	if b3[2] != "C=3" || b4[2] != "D=4" {
		t.Errorf("the results share the memory of base: %v %v", b3, b4)
	}
}
