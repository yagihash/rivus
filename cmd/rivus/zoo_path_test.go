package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// TestZooPathDart holds the rows of package:path (internal/model/builtin/dart-path.yaml) to a run: the package of
// bench/zoo-dart/pure calls each function with a marker, in two variants (two markers of the same length), and writes
// what came out. A function whose row says that its arguments flow to its result has to give another text for the
// other marker; the part of the call that has the marker is in the result. The answer is the run, not the model.
// It needs the Dart SDK (RIVUS_DART) and the packages of bench/zoo-dart/pure as its lock file names them
// (`dart pub get --enforce-lockfile`: the package is fetched, no code of it runs).
func TestZooPathDart(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "bench", "zoo-dart", "pure"))
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("RIVUS_DART") == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	if out, err := zooDartCmd(t, dir, nil, "pub", "get", "--enforce-lockfile").CombinedOutput(); err != nil {
		t.Fatalf("pub get --enforce-lockfile: %v\n%s", err, out)
	}
	type part struct{ Symbol, Part, Text string }
	run := func(marker string) []part {
		out, err := zooDartCmd(t, dir, []string{"ZOO_MARKER=" + marker}, "run", "bin/observe.dart").Output()
		if err != nil {
			t.Fatalf("running observe.dart (%s): %v", marker, err)
		}
		var ps []part
		if err := json.Unmarshal(out, &ps); err != nil {
			t.Fatal(err)
		}
		return ps
	}
	a, a2, b := run("ZmarkAAAA"), run("ZmarkAAAA"), run("ZmarkBBBB")
	if len(a) != len(b) || len(a) != len(a2) || len(a) == 0 {
		t.Fatalf("the runs have a different number of parts: %d %d %d", len(a), len(a2), len(b))
	}
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]model.Propagator{}
	for _, rs := range sets {
		for _, p := range rs.Propagators {
			rows[p.Symbol] = p
		}
	}
	seen := map[string]bool{}
	for i := range a {
		if a[i].Text != a2[i].Text {
			t.Errorf("%s %s is not the same in two runs of one marker: do not use it here", a[i].Symbol, a[i].Part)
			continue
		}
		seen[a[i].Symbol] = true
		row, ok := rows[a[i].Symbol]
		if !ok {
			t.Errorf("%s is called here and has no row", a[i].Symbol)
			continue
		}
		carries := a[i].Text != b[i].Text
		// the part that has the marker is an argument (named from, the base of relative, it is the second one)
		arg := a[i].Part
		if arg == "from" {
			arg = "arg1"
		}
		claimed := false
		for _, f := range row.Flows {
			if (f.From == "args" || f.From == arg) && strings.HasPrefix(f.To, "result") {
				claimed = true
			}
		}
		switch {
		case row.Pure && carries:
			t.Errorf("%s %s: the row is pure and the result depends on the marker", a[i].Symbol, a[i].Part)
		case claimed && !carries:
			t.Errorf("%s %s: the row carries this argument to the result and the result does not depend on the marker", a[i].Symbol, a[i].Part)
		case !claimed && carries:
			t.Errorf("%s %s: the result depends on this argument and the row does not carry it", a[i].Symbol, a[i].Part)
		case claimed && !strings.Contains(a[i].Text, "ZmarkAAAA"):
			t.Errorf("%s %s: the marker is not in the result %q", a[i].Symbol, a[i].Part, a[i].Text)
		}
	}
	for sym := range rows {
		if strings.HasPrefix(sym, "package:path/") && !seen[sym] {
			t.Errorf("%s has a row and the run does not call it: add it to bench/zoo-dart/pure/lib/observe.dart", sym)
		}
	}
}
