package ir

// The stream repeats a path in every position. A function writes the file its
// positions are in once (Func.File), and leaves it out of the positions that
// are in it. The reader puts it back, and shares the strings that the stream
// repeats (the names of types, functions and symbols), so that a large program
// does not hold a copy of each in every instruction.

// eachPos calls f for every position of the function.
func (fn *Func) eachPos(f func(*Pos)) {
	f(&fn.Pos)
	for bi := range fn.Blocks {
		b := &fn.Blocks[bi]
		f(&b.Term.Pos)
		f(&b.Term.CondPos)
		for ii := range b.Instrs {
			in := &b.Instrs[ii]
			f(&in.Pos)
			f(&in.AddrPos)
			for k := range in.ArgPos {
				f(&in.ArgPos[k])
			}
		}
	}
}

// compact leaves the file of the function out of its positions, and returns
// what restores them.
func (fn *Func) compact() (restore func()) {
	if fn.File != "" || fn.Pos.File == "" {
		return func() {}
	}
	file := fn.Pos.File
	fn.File = file
	var blanked []*Pos
	fn.eachPos(func(p *Pos) {
		if p.File == file {
			p.File = ""
			blanked = append(blanked, p)
		}
	})
	return func() {
		for _, p := range blanked {
			p.File = file
		}
		fn.File = ""
	}
}

// expand puts the file of the function back into the positions that have
// none (a position that is not set at all stays unset).
func (fn *Func) expand() {
	if fn.File == "" {
		return
	}
	fn.eachPos(func(p *Pos) {
		if p.File == "" && (p.Offset != 0 || p.Line != 0 || p.Col != 0 || p.Length != 0) {
			p.File = fn.File
		}
	})
	fn.File = ""
}

// interner shares equal strings.
type interner map[string]string

func (in interner) of(s string) string {
	if s == "" {
		return s
	}
	if t, ok := in[s]; ok {
		return t
	}
	in[s] = s
	return s
}

// intern makes the strings of the function the shared ones.
func (fn *Func) intern(in interner) {
	fn.ID = FuncID(in.of(string(fn.ID)))
	fn.Symbol = in.of(fn.Symbol)
	fn.Pos.File = in.of(fn.Pos.File)
	fn.File = in.of(fn.File)
	for i := range fn.Params {
		fn.Params[i].Type = TypeID(in.of(string(fn.Params[i].Type)))
		fn.Params[i].Display = in.of(fn.Params[i].Display)
	}
	for i := range fn.Locals {
		fn.Locals[i].Type = TypeID(in.of(string(fn.Locals[i].Type)))
		fn.Locals[i].Name = in.of(fn.Locals[i].Name)
	}
	for i := range fn.Results {
		fn.Results[i] = TypeID(in.of(string(fn.Results[i])))
	}
	fn.eachPos(func(p *Pos) { p.File = in.of(p.File) })
	for bi := range fn.Blocks {
		for ii := range fn.Blocks[bi].Instrs {
			x := &fn.Blocks[bi].Instrs[ii]
			x.Operator = in.of(x.Operator)
			x.Type = TypeID(in.of(string(x.Type)))
			x.Field = in.of(x.Field)
			x.Global = in.of(x.Global)
			x.Func = FuncID(in.of(string(x.Func)))
			if c := x.Callee; c != nil {
				c.Func = FuncID(in.of(string(c.Func)))
				c.Symbol = in.of(c.Symbol)
				for k := range c.Candidates {
					c.Candidates[k] = FuncID(in.of(string(c.Candidates[k])))
				}
			}
		}
	}
}

// PruneDeps drops the functions of dependencies that nothing the project's own
// functions do can reach (through calls, the candidates of calls through
// interfaces and function values, and the functions named as values). A
// dependency is read for what the project uses of it; the rest of it cannot
// change the analysis.
func (p *Program) PruneDeps() {
	any := false
	for _, f := range p.Funcs {
		if f.Dep {
			any = true
			break
		}
	}
	if !any {
		return
	}
	byID := make(map[FuncID][]*Func, len(p.Funcs))
	for _, f := range p.Funcs {
		byID[f.ID] = append(byID[f.ID], f)
	}
	keep := map[FuncID]bool{}
	var work []*Func
	visit := func(id FuncID) {
		if id == "" || keep[id] {
			return
		}
		if fs := byID[id]; fs != nil {
			keep[id] = true
			work = append(work, fs...)
		}
	}
	for _, f := range p.Funcs {
		if !f.Dep && !keep[f.ID] {
			visit(f.ID)
		}
	}
	for _, im := range p.Impls {
		for _, id := range im.Impls {
			visit(id)
		}
	}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				visit(in.Func)
				// a variable that is read has its initializer run (yagihash/rivus #60)
				if in.Global != "" {
					visit(FuncID(in.Global + "$init"))
				}
				if c := in.Callee; c != nil {
					visit(c.Func)
					for _, id := range c.Candidates {
						visit(id)
					}
				}
			}
		}
	}
	out := p.Funcs[:0]
	for _, f := range p.Funcs {
		if keep[f.ID] {
			out = append(out, f)
		}
	}
	for i := len(out); i < len(p.Funcs); i++ {
		p.Funcs[i] = nil
	}
	p.Funcs = out
}
