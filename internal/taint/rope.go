package taint

// rope is an immutable sequence of steps with O(1) concatenation. Summaries
// hold one path per element; copying them at every call site made the memory
// grow with the square of the call depth. A rope shares the common parts.
type rope struct {
	l, r *rope
	s    *stepRec
	n    int
}

// stepRec is what a leaf of a rope keeps of a Step: 72 bytes. A Step also has
// a position and a snippet (empty in a rope: they are made when a finding is
// made), a copy of the guards of the hop (read nowhere) and the position in
// 48 bytes. The notes are rare, so they are kept apart.
type stepRec struct {
	Func  string
	Desc  string
	pos   cpos
	Model *ModelRef
	rare  *stepRare
}

type stepRare struct {
	notes []*Uncertainty
}

// newRec makes a record; notes may be nil or empty.
func newRec(fn, desc string, pos cpos, model *ModelRef, notes []*Uncertainty) stepRec {
	rec := stepRec{Func: fn, Desc: desc, pos: pos, Model: model}
	if len(notes) > 0 {
		rec.rare = &stepRare{notes: notes}
	}
	return rec
}

func (rec *stepRec) notes() []*Uncertainty {
	if rec.rare == nil {
		return nil
	}
	return rec.rare.notes
}

// step is the Step that the record stands for.
func (rec *stepRec) step() Step {
	return Step{Func: rec.Func, Desc: rec.Desc, pos: rec.pos.pos(), Model: rec.Model, Notes: rec.notes()}
}

// leafNode is a leaf and its record in one allocation.
type leafNode struct {
	rope
	rec stepRec
}

const maxRopeLen = 1 << 30

func leaf(rec stepRec) *rope {
	ln := &leafNode{rec: rec}
	ln.n = 1
	ln.s = &ln.rec
	return &ln.rope
}

func (r *rope) len() int {
	if r == nil {
		return 0
	}
	return r.n
}

func cat(a, b *rope) *rope {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	n := a.n + b.n
	if n > maxRopeLen || n < 0 {
		n = maxRopeLen
	}
	return &rope{l: a, r: b, n: n}
}

// each calls f on the steps in order until f returns false.
func (r *rope) each(f func(Step) bool) {
	r.eachRec(func(rec *stepRec) bool { return f(rec.step()) })
}

// eachRec is each on the records.
func (r *rope) eachRec(f func(*stepRec) bool) {
	if r == nil {
		return
	}
	stack := []*rope{r}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if x.s != nil {
			if !f(x.s) {
				return
			}
			continue
		}
		stack = append(stack, x.r, x.l)
	}
}

// eachReverse calls f on the steps from the last to the first.
func (r *rope) eachReverse(f func(Step) bool) {
	r.eachRecReverse(func(rec *stepRec) bool { return f(rec.step()) })
}

func (r *rope) eachRecReverse(f func(*stepRec) bool) {
	if r == nil {
		return
	}
	stack := []*rope{r}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if x.s != nil {
			if !f(x.s) {
				return
			}
			continue
		}
		stack = append(stack, x.l, x.r)
	}
}

// flatten returns the steps. A path longer than max keeps both ends: where it
// started and where it arrived.
func (r *rope) flatten(max int) []Step {
	out, _ := r.flattenOmitted(max)
	return out
}

// flattenOmitted is flatten that also says how many steps it left out of the middle (0: none).
func (r *rope) flattenOmitted(max int) ([]Step, int) {
	if r == nil {
		return nil, 0
	}
	if r.n <= max {
		out := make([]Step, 0, r.n)
		r.each(func(s Step) bool { out = append(out, s); return true })
		return out, 0
	}
	half := max / 2
	head := make([]Step, 0, half)
	r.each(func(s Step) bool { head = append(head, s); return len(head) < half })
	tail := make([]Step, 0, half)
	r.eachReverse(func(s Step) bool { tail = append(tail, s); return len(tail) < half })
	for i, j := 0, len(tail)-1; i < j; i, j = i+1, j-1 {
		tail[i], tail[j] = tail[j], tail[i]
	}
	return append(head, tail...), r.n - len(head) - len(tail)
}
