package goir

import (
	"go/token"

	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
)

// Texts is the ir.Texts of Go code: it cuts the text from the syntax trees
// that the program was loaded with.
type Texts struct {
	p *gossa.Program
	// spans is where each function is in the source. Only that is kept of the
	// go/ssa program, so that the program, and the syntax trees of the
	// functions of the dependencies, can be let go.
	spans map[ir.FuncID]span
}

type span struct{ start, end token.Pos }

// NewTexts returns the text source of a loaded program.
func NewTexts(p *gossa.Program) *Texts {
	t := &Texts{p: p, spans: map[ir.FuncID]span{}}
	for _, fn := range p.Funcs {
		if n := fn.Syntax(); n != nil {
			t.spans[ir.FuncID(gossa.FuncID(fn))] = span{n.Pos(), n.End()}
		}
	}
	return t
}

var _ ir.Texts = (*Texts)(nil)

func (t *Texts) tok(p ir.Pos) token.Pos { return t.p.TokenPosAt(p.File, p.Offset, p.Line) }

// Snippet implements ir.Texts.
func (t *Texts) Snippet(p ir.Pos) string { return t.p.Snippet(t.tok(p)) }

// Expr implements ir.Texts.
func (t *Texts) Expr(p ir.Pos) string { return t.p.ExprText(t.tok(p)) }

// Body implements ir.Texts.
func (t *Texts) Body(id ir.FuncID) string {
	return t.p.BodyTextRange(t.spans[id].start, t.spans[id].end)
}

// Cond implements ir.Texts.
func (t *Texts) Cond(p ir.Pos) string { return t.p.CondText(t.tok(p)) }

// CallArg implements ir.Texts.
func (t *Texts) CallArg(p ir.Pos, i int) (string, string) { return t.p.CallArgText(t.tok(p), i) }

// FuncSource implements ir.Texts.
func (t *Texts) FuncSource(id ir.FuncID, maxLines int) string {
	return t.p.FuncSourceRange(t.spans[id].start, t.spans[id].end, maxLines)
}
