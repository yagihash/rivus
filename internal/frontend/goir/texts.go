package goir

import (
	"go/ast"
	"go/token"

	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
)

// Texts is the ir.Texts of Go code: it cuts the text from the syntax trees
// that the program was loaded with.
type Texts struct {
	p *gossa.Program
	// syntax is the syntax of each function. Only that is kept of the go/ssa
	// program, so that the program itself can be let go.
	syntax map[ir.FuncID]ast.Node
}

// NewTexts returns the text source of a loaded program.
func NewTexts(p *gossa.Program) *Texts {
	t := &Texts{p: p, syntax: map[ir.FuncID]ast.Node{}}
	for _, fn := range p.Funcs {
		if n := fn.Syntax(); n != nil {
			t.syntax[ir.FuncID(gossa.FuncID(fn))] = n
		}
	}
	return t
}

var _ ir.Texts = (*Texts)(nil)

func (t *Texts) tok(p ir.Pos) token.Pos { return t.p.TokenPos(p.File, p.Offset) }

// Snippet implements ir.Texts.
func (t *Texts) Snippet(p ir.Pos) string { return t.p.Snippet(t.tok(p)) }

// Expr implements ir.Texts.
func (t *Texts) Expr(p ir.Pos) string { return t.p.ExprText(t.tok(p)) }

// Body implements ir.Texts.
func (t *Texts) Body(id ir.FuncID) string { return t.p.BodyTextOf(t.syntax[id]) }

// Cond implements ir.Texts.
func (t *Texts) Cond(p ir.Pos) string { return t.p.CondText(t.tok(p)) }

// CallArg implements ir.Texts.
func (t *Texts) CallArg(p ir.Pos, i int) (string, string) { return t.p.CallArgText(t.tok(p), i) }

// FuncSource implements ir.Texts.
func (t *Texts) FuncSource(id ir.FuncID, maxLines int) string {
	return t.p.FuncSourceOf(t.syntax[id], maxLines)
}
