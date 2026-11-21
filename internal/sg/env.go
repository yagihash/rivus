package sg

import (
	"regexp"
	"strconv"
)

// envType is the type of the environment of a closure of Rust: where the closure is written.
var envType = regexp.MustCompile(`^\{closure@(.+):(\d+):(\d+): \d+:\d+\}$`)

// linkEnvClosures finds the function of each closure that is made as the alloc of its environment
// (Rust: a closure that captured values is not a make_closure). The type of the environment says
// where the closure is written, and the function of the closure is at that place.
func (p *Program) linkEnvClosures() {
	type at struct {
		file      string
		line, col int
	}
	var index map[at][]*Func
	for _, fn := range p.Funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.IR == nil || in.IR.Env == "" {
					continue
				}
				m := envType.FindStringSubmatch(in.IR.Env)
				if m == nil {
					continue
				}
				if index == nil {
					index = map[at][]*Func{}
					for _, f := range p.Funcs {
						if !isClosureID(f.ID) {
							continue
						}
						file := f.Pos.File
						if file == "" {
							file = f.IR.File
						}
						k := at{file, f.Pos.Line, f.Pos.Col}
						index[k] = append(index[k], f)
					}
				}
				line, _ := strconv.Atoi(m[2])
				col, _ := strconv.Atoi(m[3])
				// exactly one function is at the place (a macro that writes the closure in many places is not matched)
				if fs := index[at{m[1], line, col}]; len(fs) == 1 {
					if p.EnvClosures == nil {
						p.EnvClosures = map[*Instr]*Func{}
					}
					p.EnvClosures[in] = fs[0]
				}
			}
		}
	}
}

func isClosureID(id string) bool {
	for i := 0; i+9 <= len(id); i++ {
		if id[i:i+9] == "{closure#" {
			return true
		}
	}
	return false
}
