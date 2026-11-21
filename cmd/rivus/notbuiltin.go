package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
)

// notBuiltin is a framework that rivus does not model on its own (ADR-063): the built-in rules cover the
// standard library, the libraries that have no replacement and the SQL drivers; a specific framework is
// for the project to add, and examples/rules has a model to start from.
type notBuiltin struct {
	name   string // what the project calls it
	prefix string // what its symbols and types start with in the IR
	file   string // the example rule file
	what   string // what the model tells rivus
}

var notBuiltinFrameworks = []notBuiltin{
	{"actix-web", "actix_web::", "examples/rules/rust-actix-web.yaml", "the entry points of the handlers"},
}

// noteNotBuiltin writes one line for each framework that the code uses and that no rule names (a
// framework with its model in the rules, given by -rules or the project, is not mentioned). The result is
// not changed.
func noteNotBuiltin(w io.Writer, prog *ir.Program, rules *model.Rules) {
	if prog == nil || rules == nil {
		return
	}
	for _, fw := range notBuiltinFrameworks {
		if !rulesName(rules, fw.prefix) && usesPrefix(prog, fw.prefix) {
			fmt.Fprintf(w, "rivus: this project uses %s, and its model is not built in: %s are not known. -rules %s gives them (README: ルールとモデル, ADR-063)\n", fw.name, fw.what, fw.file)
		}
	}
}

func rulesName(r *model.Rules, prefix string) bool {
	for _, s := range r.ParamTypes {
		if strings.Contains(s.Type, prefix) {
			return true
		}
	}
	for k := range r.ParamFuncs {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range r.CallSources {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range r.Sinks {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range r.Sanitizers {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range r.Propagators {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

// usesPrefix: a function of the project (not of a dependency) calls a symbol that starts with prefix, or
// takes a parameter of a type that does.
func usesPrefix(p *ir.Program, prefix string) bool {
	for _, f := range p.Funcs {
		if f.Dep {
			continue
		}
		for _, pa := range f.Params {
			if strings.Contains(string(pa.Type), prefix) || strings.Contains(pa.Display, prefix) {
				return true
			}
		}
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				if in.Op == ir.OpCall && in.Callee != nil && strings.HasPrefix(in.Callee.Symbol, prefix) {
					return true
				}
			}
		}
	}
	return false
}
