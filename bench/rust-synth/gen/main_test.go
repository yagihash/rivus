package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"

	"github.com/yagihash/rivus/bench/synthplan"
)

// digest is the hash of the paths and contents of a generated project.
func digest(files map[string]string) string {
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p + "\x00" + files[p] + "\x00"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func gen(p synthplan.Params) map[string]string { return Render(synthplan.Build(p)) }

// The same parameters write the same code, every time.
func TestDeterministic(t *testing.T) {
	for _, size := range []string{"S", "M"} {
		if digest(gen(synthplan.Sizes[size])) != digest(gen(synthplan.Sizes[size])) {
			t.Errorf("size %s: two runs wrote different code", size)
		}
	}
}

// A change of the seed changes the code.
func TestSeedMatters(t *testing.T) {
	p := synthplan.Sizes["S"]
	a := gen(p)
	p.Seed++
	if digest(a) == digest(gen(p)) {
		t.Error("the seed does not change the code")
	}
}

// The code of S is fixed: if this fails on purpose (the generator changed), run
// "go test ./bench/rust-synth/gen -run TestSnapshot -v", copy the new digest here and
// refresh expected/ with measure.py snapshot (see README.md).
const snapshotS = "7f9f1ea09480a3d873c1b82ba5bcefdd95b01514f073ec02f80918a69b0cc23f"

func TestSnapshot(t *testing.T) {
	got := digest(gen(synthplan.Sizes["S"]))
	t.Logf("digest of S: %s", got)
	if got != snapshotS {
		t.Errorf("the code of S changed: digest %s", got)
	}
}
