package main

import (
	"regexp"
	"strconv"
	"strings"
)

var closureTail = regexp.MustCompile(`^(.*)\.(?:func)?(\d+)$`)

// FuncIndex finds the function of the graph (its fid, the form of ssa.Function.String) that a runtime function name
// (runtime.Frame.Function) stands for.
type FuncIndex struct {
	byKey map[string]string // the fid with every [...] written as [] -> the fid
}

// bracketKey writes every type argument list as [] (the runtime writes [...], the fid [T] or [string]).
func bracketKey(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '[':
			if depth == 0 {
				b.WriteString("[]")
			}
			depth++
		case r == ']':
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NewFuncIndex indexes the fids. A fid that has a body only for the pointer form of a method (or only the value form) is
// found by either form (a value method has both in the graph, the runtime names the value one).
func NewFuncIndex(fids []string) *FuncIndex {
	x := &FuncIndex{byKey: map[string]string{}}
	for _, f := range fids {
		x.byKey[bracketKey(f)] = f
	}
	return x
}

// Lookup maps the runtime name to a fid ("" when it is not a function of the graph).
//
//	pkg.F                   -> pkg.F
//	pkg.T.M                 -> (pkg.T).M          (a value receiver)
//	pkg.(*T).M              -> (*pkg.T).M
//	pkg.F.func1, .func1.2   -> pkg.F$1, pkg.F$1$2 (also pkg.F.func1.func1, which is what an inlined closure is called)
//	pkg.init.0              -> pkg.init#1
//	pkg.F[...], (*G[...])   -> the type argument lists are not compared
func (x *FuncIndex) Lookup(name string) string {
	name = bracketKey(name)
	var parts []string
	for {
		m := closureTail.FindStringSubmatch(name)
		if m == nil {
			break
		}
		name = m[1]
		parts = append([]string{m[2]}, parts...)
	}
	if strings.HasSuffix(name, ".init") && len(parts) > 0 {
		n, _ := strconv.Atoi(parts[0])
		name += "#" + strconv.Itoa(n+1)
		parts = parts[1:]
	}
	suffix := ""
	for _, p := range parts {
		suffix += "$" + p
	}
	for _, base := range baseForms(name) {
		if f, ok := x.byKey[base+suffix]; ok {
			return f
		}
	}
	return ""
}

// baseForms are the forms of the fid that a runtime name (without its closure numbers) can be.
func baseForms(name string) []string {
	slash := strings.LastIndexByte(name, '/')
	dot := strings.IndexByte(name[slash+1:], '.')
	if dot < 0 {
		return []string{name}
	}
	pkg := name[:slash+1+dot]
	rest := name[slash+1+dot+1:]
	if strings.HasPrefix(rest, "(*") { // pkg.(*T).M
		if i := strings.Index(rest, ")."); i > 0 {
			t, m := rest[2:i], rest[i+2:]
			return []string{"(*" + pkg + "." + t + ")." + m, "(" + pkg + "." + t + ")." + m}
		}
	}
	if i := strings.IndexByte(strings.ReplaceAll(rest, "[]", "__"), '.'); i > 0 { // pkg.T.M
		t, m := rest[:i], rest[i+1:]
		return []string{"(" + pkg + "." + t + ")." + m, "(*" + pkg + "." + t + ")." + m, name}
	}
	return []string{name}
}

// canon makes the pointer form and the value form of a method the same (the graph has the wrapper of a value method
// as a function of its own).
func canon(fid string) string {
	if strings.HasPrefix(fid, "(*") {
		return "(" + fid[2:]
	}
	return fid
}
