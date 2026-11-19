package taint

import (
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// `add` and the two look-aheads that avoid making a reason (`plainExists`, `addsNothing`) decide from
// the same `targetOf`. Whatever the look-aheads call "nothing to add", `add` must not change
// anything by (yagihash/rivus #21).
func TestAddAndItsLookAheadsAgree(t *testing.T) {
	str := &sg.Value{Class: ir.ClassString}
	num := &sg.Value{Class: ir.ClassNumber}
	real1 := RealElem("user_input", "site-1")
	wide := ParamElem(1, "/a/b")
	deep := strings.Repeat("/f", DefaultOptions().K+2)

	type scene struct {
		name  string
		root  Root
		path  string
		e     Elem
		setup func(s *state, l Loc)
	}
	plainAt := func(s *state, l Loc, e Elem) {
		cm := s.cellsOf(l.Root)
		p, _ := pathTrunc(l.Path, s.eng.opts.K)
		if cm[p] == nil {
			cm[p] = Set{}
		}
		cm[p][e] = &why{}
	}
	scenes := []scene{
		{"fresh cell", Root{str, -1}, "", real1, func(s *state, l Loc) {}},
		{"a plain element is there", Root{str, -1}, "/x", real1, func(s *state, l Loc) { plainAt(s, l, real1) }},
		{"an element with other guards", Root{str, -1}, "/x", real1, func(s *state, l Loc) {
			plainAt(s, l, real1)
			s.cells[l.Root]["/x"][real1].mk().guards = []Guard{{}}
		}},
		{"a number is left out", Root{num, -1}, "", real1, func(s *state, l Loc) {}},
		{"a number below a path is not", Root{num, -1}, "/x", real1, func(s *state, l Loc) {}},
		{"a path cut at K, the element is there", Root{str, -1}, deep, real1, func(s *state, l Loc) { plainAt(s, l, real1) }},
		{"a path cut at K, nothing there", Root{str, -1}, deep, real1, func(s *state, l Loc) {}},
		{"a cell that is full widens", Root{str, -1}, "/x", wide, func(s *state, l Loc) {
			for i := 0; i <= maxCellElems; i++ {
				plainAt(s, l, ParamElem(i+2, "/p"))
			}
			plainAt(s, l, ParamElem(1, ""))
		}},
		{"a full cell, widened element is there", Root{str, -1}, "/x", wide, func(s *state, l Loc) {
			for i := 0; i <= maxCellElems; i++ {
				plainAt(s, l, ParamElem(i+2, "/p"))
			}
		}},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			s := &state{eng: &Engine{opts: DefaultOptions()}, cells: map[Root]map[string]Set{}}
			l := Loc{sc.root, sc.path}
			sc.setup(s, l)
			from := ref{Loc{Root{str, -1}, "/src"}, sc.e}

			nothing := s.addsNothing([]Loc{l}, sc.e, from, true)
			exists := s.plainExists(l, sc.e, from, true)

			adds, trunc, widened, changed := s.adds, s.truncated, s.widened, s.changed
			s.add(l, sc.e, &why{})
			unchanged := s.adds == adds && s.truncated == trunc && s.widened == widened && s.changed == changed
			if nothing && !unchanged {
				t.Errorf("addsNothing says nothing to add, but add changed the state")
			}
			if exists && s.adds != adds {
				t.Errorf("plainExists says the element is there, but add put it")
			}
			// the numeric root is the one place where add has something to say that puts nothing
			if sc.root.V == num && sc.path == "" && (nothing != true || exists) {
				t.Errorf("numeric: addsNothing=%v plainExists=%v", nothing, exists)
			}
		})
	}
}
