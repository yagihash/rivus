package taint

import (
	"encoding/json"
	"testing"
	"unsafe"

	"github.com/yagihash/rivus/internal/ir"
)

func steps(n int) *rope {
	var r *rope
	for i := 0; i < n; i++ {
		r = cat(r, leaf(stepRec{Desc: string(rune('a' + i%26)), Func: itoa(i)}))
	}
	return r
}

func TestRopeOrderAndLength(t *testing.T) {
	r := cat(steps(3), steps(2))
	if r.len() != 5 {
		t.Fatalf("len = %d", r.len())
	}
	got := r.flatten(100)
	want := []string{"0", "1", "2", "0", "1"}
	for i, s := range got {
		if s.Func != want[i] {
			t.Errorf("step %d = %s, want %s", i, s.Func, want[i])
		}
	}
	var rev []string
	r.eachReverse(func(s Step) bool { rev = append(rev, s.Func); return true })
	for i, s := range rev {
		if s != want[len(want)-1-i] {
			t.Errorf("reverse step %d = %s", i, s)
		}
	}
}

func TestRopeFlattenKeepsBothEnds(t *testing.T) {
	r := steps(1000)
	got := r.flatten(10)
	if len(got) != 10 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Func != "0" || got[4].Func != "4" || got[5].Func != "995" || got[9].Func != "999" {
		t.Errorf("ends not kept: %v", got)
	}
}

func TestRopeSharesInsteadOfCopying(t *testing.T) {
	// Concatenating a rope with itself repeatedly doubles its length but not
	// its memory.
	r := steps(1)
	for i := 0; i < 40; i++ {
		r = cat(r, r)
	}
	if r.len() != 1<<30 {
		t.Errorf("len = %d, want the cap", r.len())
	}
	if got := r.flatten(4); len(got) != 4 {
		t.Errorf("flatten(4) = %d steps", len(got))
	}
}

func TestNilRope(t *testing.T) {
	var r *rope
	if r.len() != 0 || r.flatten(5) != nil || cat(nil, nil) != nil {
		t.Error("nil rope misbehaves")
	}
}

// A record turns back into the Step it was made from: the notes (none, an
// empty list or some), the model and the position are the same, and the JSON
// is the same (a Step has no notes and no model in the output when it has none).
func TestStepRecRoundTrip(t *testing.T) {
	u := newUncertainty(KindDepthLimit, "x", "")
	m := &ModelRef{Role: "sanitizer", Symbol: "s"}
	pos := packPos(ir.Pos{File: "a.go", Offset: 5, Length: 2, Line: 3, Col: 4})
	for _, notes := range [][]*Uncertainty{nil, {}, {u}} {
		for _, model := range []*ModelRef{nil, m} {
			want := Step{Func: "f", Desc: "d", pos: pos.pos(), Model: model}
			if len(notes) > 0 {
				want.Notes = notes
			}
			got := leaf(newRec("f", "d", pos, model, notes)).flatten(10)
			if len(got) != 1 {
				t.Fatalf("len = %d", len(got))
			}
			a, _ := json.Marshal(want)
			b, _ := json.Marshal(got[0])
			if string(a) != string(b) || got[0].pos != want.pos || got[0].Model != want.Model {
				t.Errorf("notes %v model %v: got %s, want %s", notes, model, b, a)
			}
		}
	}
	if leaf(newRec("", "", cpos{}, nil, nil)).flatten(1)[0].pos.File != "" {
		t.Error("an empty position changed")
	}
	if got := unsafe.Sizeof(stepRec{}); got != 72 {
		t.Errorf("stepRec is %d bytes, want 72", got)
	}
}
