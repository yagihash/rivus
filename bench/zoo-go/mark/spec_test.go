//go:build zooexec

package mark

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestSpec holds the constants of mark_exec.go to bench/zoo/spec.json, and runs its probes.
func TestSpec(t *testing.T) {
	b, err := os.ReadFile("../../zoo/spec.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Prefix  string
		Order   []string
		Classes map[string]struct {
			Segment string
			Danger  [2]string
			Set     string
		}
		Probes []struct {
			Class   string
			Text    string
			Base    string
			Reached bool
		}
	}
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Prefix != "Z" || len(spec.Order) != len(order) {
		t.Fatalf("prefix or order differ: %v", spec.Order)
	}
	for i, c := range order {
		if string(c) != spec.Order[i] {
			t.Errorf("order[%d]: %s, spec %s", i, c, spec.Order[i])
		}
		s := spec.Classes[string(c)]
		if s.Segment != specs[c].segment || s.Danger != specs[c].danger || s.Set != specs[c].set {
			t.Errorf("%s differs from the spec: %+v, %+v", c, specs[c], s)
		}
		for v := 0; v < 2; v++ {
			if len(s.Danger[v]) != len(s.Danger[0]) {
				t.Errorf("%s: the dangers differ in length", c)
			}
			for _, r := range s.Danger[v] {
				if !strings.ContainsRune(s.Set, r) {
					t.Errorf("%s: %q of the danger is not in the set", c, r)
				}
			}
		}
	}
	// the set of a class holds no character of the danger of another class (a sanitizer must not touch it)
	for _, c := range order {
		for _, o := range order {
			if o == c {
				continue
			}
			for _, r := range specs[c].set {
				if strings.ContainsRune(specs[o].danger[0]+specs[o].danger[1], r) {
					t.Errorf("the set of %s has %q, which is in the danger of %s", c, r, o)
				}
			}
		}
	}
	for _, p := range spec.Probes {
		d := Danger(Class(p.Class))
		text := strings.ReplaceAll(strings.ReplaceAll(p.Text, "{DQ}", d[:1]+d), "{D}", d)
		if got := Judge(Class(p.Class), p.Base, text); got != p.Reached {
			t.Errorf("probe %s %q (variant %d): reached %v, the spec says %v", p.Class, text, Variant, got, p.Reached)
		}
	}
	// a sanitizer gives the same text for both variants, and no danger is left
	for _, c := range order {
		a, b := clean(c, specs[c].danger[0]), clean(c, specs[c].danger[1])
		if a != b {
			t.Errorf("%s: the sanitized dangers differ: %q %q", c, a, b)
		}
	}
}
