package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// -func-values (docs/DESIGN-callouts.md, 5): the analysis is run once more with the function values as elements, and
// the table says, for each call of a function value, what the elements say it may call. It is a diagnostic: the
// output of the analysis is the same with it and without it. The acceptance of the first step: every candidate that
// valuecalls or the extractor has for a call of a function value is among those that the elements carry, and the calls
// that had no candidate get some (the zoo and the curated target of Dart: the numbers of 5.4).
func TestFuncValuesDart(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	zooDartRoot(t) // writes the pubspec.yaml of bench/zoo-dart/cells (generated, not committed)
	extractor, err := filepath.Abs("../../extractor/dart/bin/rivus_dart.dart")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		dir         string
		minZeroGot  int // calls with no candidate that the elements give some
		zeroCalls   int
		minWithCand int
	}{
		{"../../bench/zoo-dart/cells", 38, 38, 46}, // the cells dyn/sink-as-value-map and -field added 8 calls with no candidate (#127)
		{"../../bench/dart-curated", 20, 21, 51},
	} {
		run := func(args ...string) []byte {
			cmd := exec.Command(binary, append([]string{"analyze"}, args...)...)
			cmd.Dir = c.dir
			cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+extractor)
			var out bytes.Buffer
			cmd.Stdout = &out
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: %v", c.dir, err)
			}
			return out.Bytes()
		}
		table := filepath.Join(t.TempDir(), "fv.jsonl")
		with := run("-func-values", table, "-rules", "rules.yaml", "./...")
		without := run("-rules", "rules.yaml", "./...")
		var a, b struct {
			Findings json.RawMessage `json:"findings"`
		}
		if err := json.Unmarshal(with, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(without, &b); err != nil {
			t.Fatal(err)
		}
		if string(a.Findings) != string(b.Findings) {
			t.Errorf("%s: the findings differ with -func-values", c.dir)
		}
		raw, err := os.ReadFile(table)
		if err != nil {
			t.Fatal(err)
		}
		zero, zeroGot, withCand, contained := 0, 0, 0, 0
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			var r funcValueRow
			if err := json.Unmarshal([]byte(line), &r); err != nil {
				t.Fatal(err)
			}
			have := map[string]bool{}
			for _, id := range r.Carried {
				have[id] = true
			}
			existing := r.ValueCalls
			if len(existing) == 0 {
				existing = r.Extractor
			}
			switch {
			case len(existing) == 0 && r.Limit == "":
				zero++
				if len(r.Carried) > 0 {
					zeroGot++
				}
			case len(existing) > 0:
				withCand++
				ok := true
				for _, id := range existing {
					ok = ok && have[id]
				}
				if ok {
					contained++
				} else {
					t.Errorf("%s: %s (%s): the candidates %v are not all carried: %v", c.dir, r.Func, r.Pos, existing, r.Carried)
				}
			}
		}
		if zeroGot < c.minZeroGot || zero > c.zeroCalls+2 {
			t.Errorf("%s: calls with no candidate %d (of which got some %d), want %d / at least %d", c.dir, zero, zeroGot, c.zeroCalls, c.minZeroGot)
		}
		if withCand < c.minWithCand {
			t.Errorf("%s: calls with candidates %d, want at least %d", c.dir, withCand, c.minWithCand)
		}
		t.Logf("%s: no candidate %d (got some %d); with candidates %d (all carried %d)", filepath.Base(c.dir), zero, zeroGot, withCand, contained)
	}
}
