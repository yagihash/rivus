package ssabuild

// This file holds an analysis that shares nothing with the construction of
// SSA: classical reaching definitions, computed by iterating over the graph
// until nothing changes. It exists to check Build against: the two are
// different algorithms for the same question.

// DefSet is a set of definitions that may reach a point. Only ByInstr
// definitions occur in it; Undefined says that some path reaches the point
// without defining the variable.
type DefSet struct {
	Defs      map[UseKey]bool // the Def and Zero instructions
	Undefined bool
}

func (s DefSet) equal(o DefSet) bool {
	if s.Undefined != o.Undefined || len(s.Defs) != len(o.Defs) {
		return false
	}
	for k := range s.Defs {
		if !o.Defs[k] {
			return false
		}
	}
	return true
}

// Equal reports whether the two sets hold the same definitions.
func (s DefSet) Equal(o DefSet) bool { return s.equal(o) }

// ReachingDefinitions finds, for every use, the definitions that may reach it
// along some path. It is a forward analysis over sets of definitions, run to
// a fixed point.
func ReachingDefinitions(f *Func) map[UseKey]DefSet {
	n := len(f.Blocks)
	// in[b][v] is the set of definitions of v that reach the start of b.
	in := make([][]DefSet, n)
	out := make([][]DefSet, n)
	reachable := make([]bool, n)
	var mark func(b int)
	mark = func(b int) {
		if reachable[b] {
			return
		}
		reachable[b] = true
		for _, s := range f.Blocks[b].Succs {
			mark(s)
		}
	}
	for _, r := range f.Roots {
		mark(r)
	}
	newSets := func() []DefSet {
		s := make([]DefSet, f.NVars)
		for v := range s {
			s[v] = DefSet{Defs: map[UseKey]bool{}}
		}
		return s
	}
	for b := range in {
		in[b], out[b] = newSets(), newSets()
	}
	isRoot := map[int]bool{}
	for _, r := range f.Roots {
		isRoot[r] = true
		for v := 0; v < f.NVars; v++ {
			in[r][v].Undefined = true
		}
	}
	transfer := func(b int) {
		cur := newSets()
		for v := range cur {
			for k := range in[b][v].Defs {
				cur[v].Defs[k] = true
			}
			cur[v].Undefined = in[b][v].Undefined
		}
		for i, ins := range f.Blocks[b].Instrs {
			if ins.Op != Use {
				cur[ins.Var] = DefSet{Defs: map[UseKey]bool{{Block: b, Instr: i}: true}}
			}
		}
		out[b] = cur
	}
	for changed := true; changed; {
		changed = false
		for b := 0; b < n; b++ {
			if !reachable[b] {
				continue
			}
			for v := 0; v < f.NVars; v++ {
				merged := DefSet{Defs: map[UseKey]bool{}, Undefined: isRoot[b]}
				for _, p := range predecessors(f, b) {
					if !reachable[p] {
						continue
					}
					for k := range out[p][v].Defs {
						merged.Defs[k] = true
					}
					merged.Undefined = merged.Undefined || out[p][v].Undefined
				}
				if !merged.equal(in[b][v]) {
					in[b][v] = merged
					changed = true
				}
			}
			transfer(b)
		}
	}
	// Walk each block once more to read off the set at every use.
	res := map[UseKey]DefSet{}
	for b := 0; b < n; b++ {
		if !reachable[b] {
			continue
		}
		cur := newSets()
		for v := range cur {
			for k := range in[b][v].Defs {
				cur[v].Defs[k] = true
			}
			cur[v].Undefined = in[b][v].Undefined
		}
		for i, ins := range f.Blocks[b].Instrs {
			switch ins.Op {
			case Use:
				cp := DefSet{Defs: map[UseKey]bool{}, Undefined: cur[ins.Var].Undefined}
				for k := range cur[ins.Var].Defs {
					cp.Defs[k] = true
				}
				res[UseKey{b, i}] = cp
			default:
				cur[ins.Var] = DefSet{Defs: map[UseKey]bool{{Block: b, Instr: i}: true}}
			}
		}
	}
	return res
}

func predecessors(f *Func, b int) []int {
	var out []int
	for i, blk := range f.Blocks {
		for _, s := range blk.Succs {
			if s == b {
				out = append(out, i)
			}
		}
	}
	return out
}

// Expand follows φ functions back to the definitions that can reach through
// them. For an SSA definition it returns the set that the oracle should
// agree with.
func (r *Result) Expand(d Definition, v int) DefSet {
	out := DefSet{Defs: map[UseKey]bool{}}
	seen := map[PhiKey]bool{}
	var walk func(d Definition)
	walk = func(d Definition) {
		switch d.Kind {
		case Undefined:
			out.Undefined = true
		case ByInstr:
			out.Defs[UseKey{d.Block, d.Instr}] = true
		case ByPhi:
			k := PhiKey{d.Block, v}
			if seen[k] {
				return
			}
			seen[k] = true
			for _, a := range r.PhiArgs[k] {
				walk(a)
			}
		}
	}
	walk(d)
	return out
}
