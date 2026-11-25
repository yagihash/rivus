package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
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
	for _, size := range []string{"S", "M", "F"} {
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
// "go test ./bench/dart-synth/gen -run TestSnapshot -v", copy the new digest here and
// refresh expected/ with measure.py snapshot (see README.md).
const snapshotS = "b638bfd67bdacfc43e2e47955edd49a5303893651db7179fd6b5540c041f95fc"

func TestSnapshot(t *testing.T) {
	got := digest(gen(synthplan.Sizes["S"]))
	t.Logf("digest of S: %s", got)
	if got != snapshotS {
		t.Errorf("the code of S changed: digest %s", got)
	}
}

// Size F gives closures of the project to a dependency package, which is written under .dart_tool and named in
// package_config.json; the others do not have one.
func TestSizeFHasDependency(t *testing.T) {
	f := gen(synthplan.Sizes["F"])
	for _, p := range []string{".dart_tool/dep/lib/dep.dart", ".dart_tool/dep/pubspec.yaml"} {
		if f[p] == "" {
			t.Errorf("size F: no %s", p)
		}
	}
	if !strings.Contains(f["lib/m3.dart"], "dep.d") || !strings.Contains(f[".dart_tool/package_config.json"], `"dep"`) {
		t.Error("size F: the code does not call the dependency")
	}
	if s := gen(synthplan.Sizes["S"]); s[".dart_tool/dep/lib/dep.dart"] != "" || strings.Contains(s[".dart_tool/package_config.json"], `"dep"`) {
		t.Error("size S has a dependency")
	}
}
