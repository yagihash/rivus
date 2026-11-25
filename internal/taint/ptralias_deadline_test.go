package taint

import (
	"fmt"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// aliasHeavyState is a state whose buildPtrAlias has a long way to go: n whole-struct stores from one
// source, and m slots of that source that point somewhere; every store copies every slot.
func aliasHeavyState(n, m int) *state {
	s := &state{eng: &Engine{opts: DefaultOptions()}}
	src := &sg.Value{Kind: sg.KParam, Name: "src"}
	target := &sg.Value{Kind: sg.KParam, Name: "target"}
	for i := 0; i < m; i++ {
		slot := Loc{own(src), fmt.Sprintf("/f%d", i)}
		s.addAlias(slot, Loc{own(target), ""})
	}
	for i := 0; i < n; i++ {
		dst := &sg.Value{Kind: sg.KParam, Name: fmt.Sprintf("dst%d", i)}
		s.valStores = append(s.valStores, &sg.Instr{Op: sg.Op(ir.OpStore), Args: []*sg.Value{dst, src}})
	}
	return s
}

// buildPtrAlias answers to the time limit (-timeout, -func-timeout): with a limit that has passed, it
// returns at once and says that it did not finish, however long its loops would have been.
func TestBuildPtrAliasStopsAtTheLimit(t *testing.T) {
	s := aliasHeavyState(3000, 3000)
	start := time.Now()
	_, inTime := s.buildPtrAlias(time.Now().Add(-time.Second))
	if took := time.Since(start); took > time.Second {
		t.Errorf("buildPtrAlias took %v after the limit had passed", took)
	}
	if inTime {
		t.Errorf("buildPtrAlias says it was done in time")
	}
}

// Without a limit in the way, it finishes and says so, and the result is the same as it was.
func TestBuildPtrAliasFinishesBeforeTheLimit(t *testing.T) {
	s := aliasHeavyState(5, 7)
	changed, inTime := s.buildPtrAlias(time.Now().Add(time.Hour))
	if !changed || !inTime {
		t.Errorf("changed %v, inTime %v", changed, inTime)
	}
	if len(s.ptrAlias) != 7+5*7 {
		t.Errorf("%d slots", len(s.ptrAlias))
	}
}
