package ir

// StreamTexts is the Texts of a program whose extractor wrote the texts into
// the stream, as one in another process has to.
type StreamTexts struct {
	byKind map[string]map[posKey]string
	calls  map[posKey]CallText
	funcs  map[FuncID]FuncText
}

type posKey struct {
	file   string
	offset int
}

var _ Texts = (*StreamTexts)(nil)

// NewStreamTexts indexes the texts of a program.
func NewStreamTexts(p *Program) *StreamTexts {
	t := &StreamTexts{
		byKind: map[string]map[posKey]string{TextSnippet: {}, TextExpr: {}, TextCond: {}},
		calls:  map[posKey]CallText{},
		funcs:  map[FuncID]FuncText{},
	}
	for _, g := range p.Texts {
		m := t.byKind[g.Kind]
		if m == nil {
			continue
		}
		for _, o := range g.Offsets {
			m[posKey{g.File, o}] = g.Text
		}
	}
	for _, c := range p.CallTexts {
		t.calls[posKey{c.File, c.Offset}] = c
	}
	for _, f := range p.FuncTexts {
		t.funcs[f.Func] = f
	}
	return t
}

func (t *StreamTexts) Snippet(p Pos) string  { return t.byKind[TextSnippet][posKey{p.File, p.Offset}] }
func (t *StreamTexts) Expr(p Pos) string     { return t.byKind[TextExpr][posKey{p.File, p.Offset}] }
func (t *StreamTexts) Cond(p Pos) string     { return t.byKind[TextCond][posKey{p.File, p.Offset}] }
func (t *StreamTexts) Body(id FuncID) string { return t.funcs[id].Body }

func (t *StreamTexts) CallArg(p Pos, i int) (string, string) {
	c := t.calls[posKey{p.File, p.Offset}]
	if i >= 0 && i < len(c.Args) {
		return c.Args[i], c.Call
	}
	return "", c.Call
}

// FuncSource returns the source the extractor wrote, which it cut at
// SourceLines lines.
func (t *StreamTexts) FuncSource(id FuncID, maxLines int) string { return t.funcs[id].Source }
