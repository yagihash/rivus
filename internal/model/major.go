package model

import (
	"fmt"
	"strings"
)

// MajorWildcard is the element of a Go module path that stands for the major version of the module: `/v*` matches `/v` and
// the digits of a major (`/v2`, `/v93`), one path element, and nothing else. A row written with it is for every major of the
// module (docs/DESIGN-package-models.md, 6.9; ADR-110).
const MajorWildcard = "/v*"

// HasMajorWildcard reports whether the text has the major element.
func HasMajorWildcard(s string) bool { return strings.Contains(s, MajorWildcard) }

// CheckMajorWildcard says what is wrong with the use of `/v*` in a symbol, a field, a type or a package: it is written at most
// once, after the path of a module, and it is a whole path element (after it comes `/`, `.` or `)`, or the text ends:
// `x/v*/pkg.F`, `x/v*.F`, `(*x/v*/pkg.T).M`).
func CheckMajorWildcard(s, what string) error {
	n := strings.Count(s, MajorWildcard)
	if n == 0 {
		return nil
	}
	if n > 1 {
		return fmt.Errorf("%s: %q has /v* more than once (a module has one major in its path)", what, s)
	}
	i := strings.Index(s, MajorWildcard)
	if i == 0 || s[i-1] == '/' || s[i-1] == '(' || s[i-1] == '*' {
		return fmt.Errorf("%s: %q: /v* stands for the major of a module, and comes after the path of the module", what, s)
	}
	if j := i + len(MajorWildcard); j < len(s) && s[j] != '/' && s[j] != '.' && s[j] != ')' {
		return fmt.Errorf("%s: %q: /v* is a whole path element (a /, a . or a ) comes after it), not a pattern for a part of one", what, s)
	}
	return nil
}

// isMajor reports whether s is the digits of a major of a Go module path: 2 or more, no leading zero (the paths of v0 and
// v1 have no major element).
func isMajor(s string) bool {
	if s == "" || s[0] == '0' || s == "1" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// MatchMajor reports whether the symbol (or field, type, package) is what the pattern names. A pattern without `/v*` names
// itself only. With it, the symbol has `/v` and the digits of a major where the pattern has the element.
func MatchMajor(pattern, sym string) bool {
	i := strings.Index(pattern, MajorWildcard)
	if i < 0 {
		return pattern == sym
	}
	pre, post := pattern[:i], pattern[i+len(MajorWildcard):]
	if len(sym) < len(pre)+2+len(post) || !strings.HasPrefix(sym, pre+"/v") || !strings.HasSuffix(sym, post) {
		return false
	}
	return isMajor(sym[len(pre)+2 : len(sym)-len(post)])
}

// ConcreteMajor writes the pattern for the major (the digits).
func ConcreteMajor(pattern, major string) string {
	return strings.Replace(pattern, MajorWildcard, "/v"+major, 1)
}

// majorsIn returns the majors that the import paths have for the pattern: the paths that start with the module path before
// `/v*` (what follows the last of ( ) * [ ] and a space), then `/v` and the digits, then the end or a `/`.
func majorsIn(pattern string, paths []string) []string {
	i := strings.Index(pattern, MajorWildcard)
	if i < 0 {
		return nil
	}
	mod := pattern[:i]
	if k := strings.LastIndexAny(mod, "()*[] "); k >= 0 {
		mod = mod[k+1:]
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		if !strings.HasPrefix(p, mod+"/v") {
			continue
		}
		rest := p[len(mod)+2:]
		if k := strings.IndexByte(rest, '/'); k >= 0 {
			rest = rest[:k]
		}
		if isMajor(rest) && !seen[rest] {
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out
}

// ExpandMajors makes the rows that name a module by `/v*` rows of each major of it that the program has (paths are the import
// paths of the packages of the program, the dependencies included). The index is by the exact symbol, so a row for the major
// of the program is a row of its own, as the rows that match: implementations make (Expand). A row that no major of the program
// fits stays as it is: nothing matches it. The rows of the sets are not changed.
func (r *Rules) ExpandMajors(paths []string) {
	if r.majorsDone {
		return
	}
	r.majorsDone = true
	pick := func(key string) []string {
		if !HasMajorWildcard(key) {
			return nil
		}
		return majorsIn(key, paths)
	}
	for key, v := range r.CallSources {
		for _, m := range pick(key) {
			c := ConcreteMajor(key, m)
			for _, s := range v {
				s.Symbol = c
				r.CallSources[c] = append(r.CallSources[c], s)
			}
		}
	}
	for key, v := range r.ParamFuncs {
		for _, m := range pick(key) {
			c := ConcreteMajor(key, m)
			for _, s := range v {
				s.Symbol = c
				r.ParamFuncs[c] = append(r.ParamFuncs[c], s)
			}
		}
	}
	for key, v := range r.CallbackParams {
		for _, m := range pick(key) {
			c := ConcreteMajor(key, m)
			for _, s := range v {
				s.Symbol = c
				r.CallbackParams[c] = append(r.CallbackParams[c], s)
			}
		}
	}
	for key, v := range r.Fields {
		for _, m := range pick(key) {
			c := ConcreteMajor(key, m)
			for _, s := range v {
				s.Field = c
				r.Fields[c] = append(r.Fields[c], s)
			}
		}
	}
	for _, s := range r.ParamTypes[:len(r.ParamTypes):len(r.ParamTypes)] {
		for _, m := range pick(s.Type) {
			cp := s
			cp.Type = ConcreteMajor(s.Type, m)
			r.ParamTypes = append(r.ParamTypes, cp)
		}
	}
	for key, v := range r.Sinks {
		for _, m := range pick(key) {
			c := ConcreteMajor(key, m)
			for _, s := range v {
				s.Symbol = c
				r.Sinks[c] = append(r.Sinks[c], s)
			}
		}
	}
	for key, v := range r.Sanitizers {
		for _, m := range pick(key) {
			c := ConcreteMajor(key, m)
			for _, s := range v {
				s.Symbol = c
				r.Sanitizers[c] = append(r.Sanitizers[c], s)
			}
		}
	}
	for key, v := range r.Propagators {
		for _, m := range pick(key) {
			c := ConcreteMajor(key, m)
			for _, p := range v {
				p.Symbol = c
				r.Propagators[c] = append(r.Propagators[c], p)
			}
		}
	}
	for _, set := range []map[string]bool{r.Calls, r.ReadImpls, r.SelfFirst, r.ConfigReads, r.GlobalsHoldNothing} {
		for key := range set {
			for _, m := range pick(key) {
				set[ConcreteMajor(key, m)] = true
			}
		}
	}
}
