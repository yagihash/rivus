package taint

import (
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// chainOf makes a chain of hops, the nearest to the element first: hop i came from hop i+1 (the last has no reason before it).
// Each hop is the element e in a cell of its own. A hop with guards[i] != nil holds that guard; a hop with marked[i] is unguarded
// (the element came to it by other ways too, so the guards of that hop and of the ones before it do not describe the element).
func chainOf(s *state, e Elem, guards []*Guard, marked []bool) []Loc {
	locs := make([]Loc, len(guards))
	for i := range locs {
		locs[i] = Loc{Root{&sg.Value{Class: ir.ClassString}, -1}, ""}
	}
	for i := range locs {
		w := &why{}
		if i+1 < len(locs) {
			w = newWhyFrom(nil, "hop", locs[i+1], e)
		}
		if guards[i] != nil {
			w.setGuards([]Guard{*guards[i]})
		}
		if marked[i] {
			w.markUnguarded()
		}
		s.cellsOf(locs[i].Root)[""] = Set{e: w}
	}
	return locs
}

func guardAt(block int, pol bool) *Guard { return &Guard{Func: "f", block: block, Polarity: pol} }

func newChainState() *state {
	return &state{eng: &Engine{opts: DefaultOptions()}, cells: map[Root]map[string]Set{}}
}

func guardKeys(gs []Guard) map[string]bool {
	m := map[string]bool{}
	for _, g := range gs {
		m[g.key()] = true
	}
	return m
}

// The guards of a chain stop at the hop that is marked unguarded: the guards of the hops that the element went through to get
// there (nearer to it) hold, and the ones of that hop and beyond do not. Before, one marked hop took all of them away
// (the return that is decided after a loop that applies a call again, with the hop of the call marked, was thrown away).
func TestGuardsOfChainStopAtTheNearestMarkedHop(t *testing.T) {
	g1, g2, g3, g4, g5 := guardAt(1, true), guardAt(2, true), guardAt(3, true), guardAt(4, true), guardAt(5, true)
	cases := []struct {
		name   string
		guards []*Guard
		marked []bool
		want   []*Guard
	}{
		{"none is marked: all of them", []*Guard{g1, g2, g3}, []bool{false, false, false}, []*Guard{g1, g2, g3}},
		{"a marked hop: the ones before it only", []*Guard{g1, g2, g3}, []bool{false, true, false}, []*Guard{g1}},
		{"two marked hops: the nearest stops it", []*Guard{g1, g2, g3, g4, g5}, []bool{false, true, false, true, false}, []*Guard{g1}},
		{"two marked hops, the guard between them is beyond", []*Guard{nil, g2, g3, g4, g5}, []bool{false, false, true, false, true}, []*Guard{g2}},
		{"the first hop is marked: none", []*Guard{g1, g2}, []bool{true, false}, nil},
		{"a marked hop that has no guard of its own", []*Guard{g1, nil, g3}, []bool{false, true, false}, []*Guard{g1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newChainState()
			e := RealElem("user_input", "site-1")
			locs := chainOf(s, e, c.guards, c.marked)
			got, over := s.guardsOfChain(locs[0], e)
			if over {
				t.Fatal("overflow")
			}
			want := map[string]bool{}
			for _, g := range c.want {
				want[g.key()] = true
			}
			if len(got) != len(want) {
				t.Fatalf("guards = %d, want %d: %v", len(got), len(want), guardKeys(got))
			}
			for k := range guardKeys(got) {
				if !want[k] {
					t.Errorf("guard %s is beyond the nearest marked hop", k)
				}
			}
		})
	}
}

// A chain that is marked beyond a guard and a chain that is not, with the same guard in the part that is nearer to the element:
// the element comes to the cell under that guard on both, so it is not marked unguarded (it was: the marked chain had no guards
// at all, and the other one had the guard). The guard then still takes the flow away where it is false.
func TestAddKeepsTheGuardWhenTheNearGuardsAreTheSame(t *testing.T) {
	e := RealElem("user_input", "site-1")
	g1, g2 := guardAt(1, true), guardAt(2, true)

	run := func(newGuard *Guard) *why {
		s := newChainState()
		// the chain the element is at already: x (g1), beyond it a marked hop (g2)
		old := locAndWhy(s, e, []*Guard{g1, g2}, []bool{false, true})
		// the new chain: y (newGuard) and nothing marked
		nw := locAndWhy(s, e, []*Guard{newGuard, nil}, []bool{false, false})
		target := Loc{Root{&sg.Value{Class: ir.ClassString}, -1}, ""}
		oldW := newWhyFrom(nil, "old", old, e)
		s.cellsOf(target.Root)[""] = Set{e: oldW}
		s.add(target, e, newWhyFrom(nil, "new", nw, e))
		return oldW
	}
	if w := run(g1); w.unguarded() {
		t.Error("the two chains have the same guard near the element, and the element was marked unguarded")
	}
	if w := run(g2); !w.unguarded() {
		t.Error("the two chains have other guards near the element, and the element was not marked unguarded")
	}
	if w := run(nil); !w.unguarded() {
		t.Error("one chain has a guard near the element and the other has none, and the element was not marked unguarded")
	}
}

// locAndWhy makes the chain and gives the nearest hop of it.
func locAndWhy(s *state, e Elem, guards []*Guard, marked []bool) Loc {
	return chainOf(s, e, guards, marked)[0]
}
