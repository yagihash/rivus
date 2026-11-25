//go:build linux && (amd64 || arm64)

package sandbox

import "testing"

// (the filter of seccomp is made for these two architectures only)

func TestSandboxOwnOnlyRulesAreWellMade(t *testing.T) {
	for _, r := range ownOnly {
		p := ownOnlyRule(r.nr, r.args)
		if want := 2*len(r.args) + 3; len(p) != want {
			t.Errorf("call %d: %d instructions, want %d", r.nr, len(p), want)
		}
		// the jump of the first instruction lands just after the block; the jumps of the comparisons land on the refusal or just after it
		if int(p[0].Jf) != len(p)-1 {
			t.Errorf("call %d: the first jump (%d) does not skip the block (%d)", r.nr, p[0].Jf, len(p))
		}
	}
}
