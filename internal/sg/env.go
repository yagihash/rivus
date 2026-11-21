package sg

import "github.com/yagihash/rivus/internal/ir"

// linkEnvClosures finds the function of each closure that is made as the alloc of its environment
// (Rust: a closure that captured values is not a make_closure). The type of the environment says
// where the closure is written, and the function of the closure is at that place (ir.ClosureIndex:
// the one rule, shared with the narrowing of the candidates of Fn::call).
func (p *Program) linkEnvClosures() {
	var index *ir.ClosureIndex
	for _, fn := range p.Funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.IR == nil || in.IR.Env == "" {
					continue
				}
				p.HasEnv = true
				if index == nil {
					irs := make([]*ir.Func, 0, len(p.Funcs))
					for _, f := range p.Funcs {
						if isClosureID(f.ID) {
							irs = append(irs, f.IR)
						}
					}
					index = ir.NewClosureIndex(irs)
				}
				if f := index.Lookup(in.IR.Env); f != nil {
					if p.EnvClosures == nil {
						p.EnvClosures = map[*Instr]*Func{}
					}
					p.EnvClosures[in] = p.ByID[string(f.ID)]
				}
			}
		}
	}
}

func isClosureID(id string) bool { return ir.IsClosureID(id) }
