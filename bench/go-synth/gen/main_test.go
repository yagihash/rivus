package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"
)

// digest is the hash of the paths and contents of a generated module.
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

// The same parameters write the same code, every time.
func TestDeterministic(t *testing.T) {
	for _, size := range []string{"S", "M"} {
		a, err := Generate(sizes[size])
		if err != nil {
			t.Fatal(err)
		}
		b, err := Generate(sizes[size])
		if err != nil {
			t.Fatal(err)
		}
		if digest(a) != digest(b) {
			t.Errorf("size %s: two runs wrote different code", size)
		}
	}
}

// A change of the seed changes the code.
func TestSeedMatters(t *testing.T) {
	p := sizes["S"]
	a, _ := Generate(p)
	p.Seed++
	b, _ := Generate(p)
	if digest(a) == digest(b) {
		t.Error("the seed does not change the code")
	}
}

// The code of S is fixed: if this fails on purpose (the generator changed), run
// "go test ./bench/go-synth/gen -run TestSnapshot -v", copy the new digest here and
// refresh expected/ with measure.py snapshot (see README.md).
const snapshotS = "ebdca5a38bf344c60e5549656944a4eec2d4d4e34b6720db6f57d872b62a3649"

func TestSnapshot(t *testing.T) {
	files, err := Generate(sizes["S"])
	if err != nil {
		t.Fatal(err)
	}
	got := digest(files)
	t.Logf("digest of S: %s", got)
	if got != snapshotS {
		t.Errorf("the code of S changed: %s", got)
	}
}
