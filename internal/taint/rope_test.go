package taint

import "testing"

func steps(n int) *rope {
	var r *rope
	for i := 0; i < n; i++ {
		r = cat(r, leaf(Step{Desc: string(rune('a' + i%26)), Func: itoa(i)}))
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
