package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"
)

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

// The same parameters write the same code, every time, for every shape alone and for all of them.
func TestDeterministic(t *testing.T) {
	sets := [][]string{{}, allShapes}
	for _, s := range append(append([]string{}, allShapes...), namedShapes...) {
		sets = append(sets, []string{s})
	}
	for _, set := range sets {
		mk := func() map[string]string {
			p := Default()
			p.Funcs = 200
			for _, s := range set {
				p.Shapes[s] = true
			}
			files, err := Generate(p)
			if err != nil {
				t.Fatalf("%v: %v", set, err)
			}
			return files
		}
		if digest(mk()) != digest(mk()) {
			t.Errorf("shapes %v: two runs wrote different code", set)
		}
	}
}
