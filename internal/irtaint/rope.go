package irtaint

// rope is an immutable sequence of steps with O(1) concatenation. Summaries
// hold one path per element; copying them at every call site made the memory
// grow with the square of the call depth. A rope shares the common parts.
type rope struct {
	l, r *rope
	s    *Step
	n    int
}

const maxRopeLen = 1 << 30

func leaf(st Step) *rope { return &rope{s: &st, n: 1} }

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
	if r == nil {
		return
	}
	stack := []*rope{r}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if x.s != nil {
			if !f(*x.s) {
				return
			}
			continue
		}
		stack = append(stack, x.r, x.l)
	}
}

// eachReverse calls f on the steps from the last to the first.
func (r *rope) eachReverse(f func(Step) bool) {
	if r == nil {
		return
	}
	stack := []*rope{r}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if x.s != nil {
			if !f(*x.s) {
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
	if r == nil {
		return nil
	}
	if r.n <= max {
		out := make([]Step, 0, r.n)
		r.each(func(s Step) bool { out = append(out, s); return true })
		return out
	}
	half := max / 2
	head := make([]Step, 0, half)
	r.each(func(s Step) bool { head = append(head, s); return len(head) < half })
	tail := make([]Step, 0, half)
	r.eachReverse(func(s Step) bool { tail = append(tail, s); return len(tail) < half })
	for i, j := 0, len(tail)-1; i < j; i, j = i+1, j-1 {
		tail[i], tail[j] = tail[j], tail[i]
	}
	return append(head, tail...)
}
