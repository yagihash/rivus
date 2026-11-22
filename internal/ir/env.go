package ir

import (
	"regexp"
	"strconv"
	"strings"
)

// envType is the type of the environment of a closure of Rust: where the closure is written.
var envType = regexp.MustCompile(`^\{closure@(.+):(\d+):(\d+): \d+:\d+\}$`)

// IsEnvType reports whether a type is the environment of a closure of Rust.
func IsEnvType(t string) bool { return strings.HasPrefix(t, "{closure@") && envType.MatchString(t) }

// ParentFuncIDs are the IDs that a function can be told by, from itself to the function around it: a closure and a local
// function are named after the function they are in with a trailing "$" and a number (Dart: `f$1`, `f$1$2`). A name that
// has a "$" of its own (a generated `_$FromJson`, `a$b`) is not cut there: only a trailing "$<digits>" is taken off, one at a time.
func ParentFuncIDs(id string) []string {
	out := []string{id}
	for {
		i := strings.LastIndex(id, "$")
		if i < 0 || i == len(id)-1 {
			return out
		}
		digits := id[i+1:]
		for _, c := range digits {
			if c < '0' || c > '9' {
				return out
			}
		}
		id = id[:i]
		out = append(out, id)
	}
}

// IsClosureID reports whether a function is a closure (Rust names it `path::{closure#n}`).
func IsClosureID(id string) bool { return strings.Contains(id, "{closure#") }

// ClosureIndex finds the function of a closure by the type of its environment. A closure that captured
// values is not a make_closure in Rust: it is an alloc of the type `{closure@file:line:col: ...}`, and the
// function of the closure is the one at that place. It is the one rule that says which function an alloc
// of an environment is the value of (the graph of the analysis and the narrowing of the candidates of a
// call of a closure use it).
type ClosureIndex struct {
	at map[closureAt][]*Func
}

type closureAt struct {
	file      string
	line, col int
}

// NewClosureIndex indexes the closures among the functions.
func NewClosureIndex(funcs []*Func) *ClosureIndex {
	ix := &ClosureIndex{at: map[closureAt][]*Func{}}
	for _, f := range funcs {
		if f == nil || !IsClosureID(string(f.ID)) {
			continue
		}
		file := f.Pos.File
		if file == "" {
			file = f.File
		}
		k := closureAt{file, f.Pos.Line, f.Pos.Col}
		ix.at[k] = append(ix.at[k], f)
	}
	return ix
}

// Lookup is the function of the closure whose environment has this type: exactly one function at the
// place (a macro that writes the closure in many places is not matched). nil if there is none or many.
func (ix *ClosureIndex) Lookup(env string) *Func {
	m := envType.FindStringSubmatch(env)
	if m == nil {
		return nil
	}
	line, _ := strconv.Atoi(m[2])
	col, _ := strconv.Atoi(m[3])
	if fs := ix.at[closureAt{m[1], line, col}]; len(fs) == 1 {
		return fs[0]
	}
	return nil
}
