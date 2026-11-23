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

// None now: actix-web, the one that was here, is built in (internal/model/builtin/rust-actix-web.yaml). A framework whose
// model is only an example (examples/rules) goes here, and the mechanism below tells the project about it.
var notBuiltinFrameworks = []notBuiltin{}

// noteNotBuiltin writes one line for each framework that the code uses and that no rule names (a
// framework with its model in the rules, given by -rules or the project, is not mentioned). The result is
// not changed.
func noteNotBuiltin(w io.Writer, src ir.Source, rules *model.Rules) {
	if src == nil || rules == nil {
		return
	}
	for _, fw := range notBuiltinFrameworks {
		if !rulesName(rules, fw.prefix) && usesPrefix(src, fw.prefix) {
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
func usesPrefix(src ir.Source, prefix string) bool {
	found := false
	ir.WalkProject(src, func(_ ir.Ref, f *ir.Func) bool {
		for _, pa := range f.Params {
			if strings.Contains(string(pa.Type), prefix) || strings.Contains(pa.Display, prefix) {
				found = true
				return false
			}
		}
		for bi := range f.Blocks {
			for ii := range f.Blocks[bi].Instrs {
				in := &f.Blocks[bi].Instrs[ii]
				if in.Op == ir.OpCall && in.Callee != nil && strings.HasPrefix(in.Callee.Symbol, prefix) {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}
