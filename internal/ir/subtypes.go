package ir

import "sort"

// MarkedMethod is a method found through the types: its symbol, and the
// positions (receiver excluded) of the parameters that are what the rule
// is about.
type MarkedMethod struct {
	Symbol string
	Params []int
}

// IsSubtype reports whether the type is the super or has it among its
// ancestors.
func (p *Program) IsSubtype(t, super TypeID) bool {
	return p.subtypes(super)[t]
}

// subtypes returns every type of the program that is the super or a subtype
// of it.
func (p *Program) subtypes(super TypeID) map[TypeID]bool {
	children := map[TypeID][]TypeID{}
	for _, t := range p.Types {
		for _, s := range t.Supers {
			children[s] = append(children[s], t.ID)
		}
	}
	out := map[TypeID]bool{super: true}
	work := []TypeID{super}
	for len(work) > 0 {
		t := work[len(work)-1]
		work = work[:len(work)-1]
		for _, c := range children[t] {
			if !out[c] {
				out[c] = true
				work = append(work, c)
			}
		}
	}
	return out
}

// SubtypeMethods lists the methods of the code being analysed (not of the
// dependencies) that belong to a subtype of super and take a parameter of the
// type with: the handlers a framework calls with what a client sent. The
// parameters that are of the type with, and the receiver, are left out of
// Params: what is left is the input. A method with nothing left is not listed.
func (p *Program) SubtypeMethods(super, with TypeID) []MarkedMethod {
	sub := p.subtypes(super)
	var out []MarkedMethod
	for _, f := range p.Funcs {
		if f.Dep || !f.Recv || f.Symbol == "" || len(f.Params) == 0 || !sub[f.Params[0].Type] {
			continue
		}
		hasWith := false
		for _, prm := range f.Params[1:] {
			if prm.Type == with {
				hasWith = true
			}
		}
		if !hasWith {
			continue
		}
		var params []int
		for i, prm := range f.Params[1:] {
			if prm.Type != with && !prm.Captured {
				params = append(params, i)
			}
		}
		if len(params) > 0 {
			out = append(out, MarkedMethod{Symbol: f.Symbol, Params: params})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

// CallsFunction reports whether the code being analysed calls the function
// with the symbol.
func (p *Program) CallsFunction(symbol string) bool {
	for _, f := range p.Funcs {
		if f.Dep {
			continue
		}
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				if in.Op == OpCall && in.Callee.Symbol == symbol {
					return true
				}
			}
		}
	}
	return false
}
