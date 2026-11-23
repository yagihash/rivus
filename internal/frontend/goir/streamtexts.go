package goir

import (
	"sort"

	"github.com/yagihash/rivus/internal/ir"
)

// AddTexts writes into the program the texts the core would otherwise ask
// the extractor for, so that the program can leave the process (an extractor
// of another language has to do the same: ARCHITECTURE 2.7.7). It is the
// reference for what has to be written: the snippet of every position an
// instruction has (any of them can be a step of a path), the normalized
// expression of the places where sites are, the condition around the
// instructions that follow a branch, the arguments of calls, and the body and
// the source of every function.
func AddTexts(t *Texts, prog *ir.Program) {
	addTexts(t, prog, func(fn func(*ir.Func)) {
		for _, f := range prog.Funcs {
			fn(f)
		}
	})
}

// AddTextsOf is AddTexts for a program whose functions are in a Source (the program is its Header). It reads every
// function once.
func AddTextsOf(t *Texts, src ir.Source) {
	addTexts(t, src.Header(), func(fn func(*ir.Func)) {
		ir.Walk(src, func(_ ir.Ref, f *ir.Func) { fn(f) })
	})
}

func addTexts(t *Texts, prog *ir.Program, funcs func(func(*ir.Func))) {
	type key struct {
		kind, file, text string
	}
	groups := map[key]map[int]bool{}
	add := func(kind string, p ir.Pos, text string) {
		if text == "" || p.File == "" {
			return
		}
		k := key{kind, p.File, text}
		if groups[k] == nil {
			groups[k] = map[int]bool{}
		}
		groups[k][p.Offset] = true
	}
	snippet := func(p ir.Pos) {
		if !p.IsZero() {
			add(ir.TextSnippet, p, t.Snippet(p))
		}
	}
	expr := func(p ir.Pos) {
		if !p.IsZero() {
			add(ir.TextExpr, p, t.Expr(p))
		}
	}
	for _, g := range prog.Globals {
		snippet(g.Pos)
	}
	funcs(func(f *ir.Func) {
		snippet(f.Pos)
		prog.FuncTexts = append(prog.FuncTexts, ir.FuncText{Func: f.ID, Body: t.Body(f.ID), Source: t.FuncSource(f.ID, ir.SourceLines)})
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				snippet(in.Pos)
				snippet(in.AddrPos)
				switch in.Op {
				case ir.OpCall:
					expr(in.Pos)
					ct := ir.CallText{File: in.Pos.File, Offset: in.Pos.Offset}
					for i := range in.Args {
						arg, call := t.CallArg(in.Pos, i)
						ct.Args = append(ct.Args, arg)
						ct.Call = call
					}
					if ct.File != "" && (ct.Call != "" || len(ct.Args) > 0) {
						prog.CallTexts = append(prog.CallTexts, ct)
					}
				case ir.OpFieldLoad, ir.OpFieldStore, ir.OpAddrOf, ir.OpParam:
					expr(in.Pos)
					expr(in.AddrPos)
				}
			}
			snippet(b.Term.Pos)
			// The condition around the instructions that follow a branch.
			if b.Term.Kind == ir.TermBranch {
				for _, e := range b.Succs {
					for _, in := range f.Blocks[e.To].Instrs {
						if !in.Pos.IsZero() {
							add(ir.TextCond, in.Pos, t.Cond(in.Pos))
						}
					}
				}
			}
		}
	})
	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.file != b.file {
			return a.file < b.file
		}
		return a.text < b.text
	})
	for _, k := range keys {
		g := ir.TextGroup{Kind: k.kind, File: k.file, Text: k.text}
		for o := range groups[k] {
			g.Offsets = append(g.Offsets, o)
		}
		sort.Ints(g.Offsets)
		prog.Texts = append(prog.Texts, g)
	}
	sort.Slice(prog.CallTexts, func(i, j int) bool {
		a, b := prog.CallTexts[i], prog.CallTexts[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Offset < b.Offset
	})
	prog.LoadErrors = append(prog.LoadErrors, t.LoadErrors()...)
}

// LoadErrors are the packages that could not be read.
func (t *Texts) LoadErrors() []ir.LoadError { return t.p.LoadErrors }
