package taint

import "github.com/yagihash/rivus/internal/ir"

// skippedBody tells whether one of the callee IDs (or the symbol of the call) is a function whose body the extractor could
// not read (ir.FnSkipped). A call of it is a call to a function with no body, and the note says that the body was there.
func (e *Engine) skippedBody(ids []string, sym string) *ir.FnSkipped {
	e.skippedOnce.Do(func() {
		if e.prog == nil || e.prog.IR == nil || len(e.prog.IR.FnSkipped) == 0 {
			return
		}
		e.skipped = make(map[string]*ir.FnSkipped, len(e.prog.IR.FnSkipped))
		for i := range e.prog.IR.FnSkipped {
			f := &e.prog.IR.FnSkipped[i]
			e.skipped[f.Func] = f
		}
	})
	if len(e.skipped) == 0 {
		return nil
	}
	for _, id := range ids {
		if f, ok := e.skipped[id]; ok {
			return f
		}
	}
	return e.skipped[sym]
}
