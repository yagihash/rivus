// Package taint implements the taint analysis: a function-local solver whose
// results double as function summaries, and the bottom-up propagation of
// those summaries through the call graph (the functional approach, ADR-004).
//
// Phase 1 works directly on go/ssa. Phase 2 moves it onto the neutral IR.
package taint

import (
	"sort"
	"strconv"
	"strings"

	"github.com/yagihash/rivus/internal/model"
)

// ElemKind distinguishes concrete labels from symbolic parameter taint.
type ElemKind uint8

const (
	// Real is a concrete label that came from a source site.
	Real ElemKind = iota
	// Param is symbolic: "whatever the caller passes in parameter Param,
	// below Path". It turns the analysis of a function into its summary.
	Param
)

// Elem is one unit of taint.
//
// It is small and comparable: the strings it is made of (the label, the
// origin, the path, the sanitizer operations) are kept once in a table and the
// element holds their numbers. An analysis makes and copies and hashes
// millions of elements, and as a struct of strings each of them was 80 bytes.
// The numbers say nothing about order: elements are sorted by their strings.
type Elem struct {
	Kind   ElemKind
	Param  int32 // Param: parameter index (receiver first, then free variables)
	label  strID // Real: the label name
	origin strID // Real: ID of the source site that introduced it
	path   strID // Param: access path inside the parameter object
	mods   strID // Param: sanitizer operations applied since (see applyMods)
}

// Label is, for a Real element, the label name.
func (e Elem) Label() string { return e.label.str() }

// Origin is, for a Real element, the ID of the source site that introduced it.
func (e Elem) Origin() string { return e.origin.str() }

// Path is, for a Param element, the access path inside the parameter object.
func (e Elem) Path() string { return e.path.str() }

// Mods is, for a Param element, the sanitizer operations applied since.
func (e Elem) Mods() string { return e.mods.str() }

// WithPath is the element with another path.
func (e Elem) WithPath(p string) Elem { e.path = intern(p); return e }

// WithMods is the element with other sanitizer operations.
func (e Elem) WithMods(m string) Elem { e.mods = intern(m); return e }

// RealElem makes a concrete label that came from a source site.
func RealElem(label, origin string) Elem {
	return Elem{Kind: Real, label: intern(label), origin: intern(origin)}
}

// ParamElem makes the symbolic taint of a parameter, below path.
func ParamElem(idx int, path string) Elem {
	return Elem{Kind: Param, Param: int32(idx), path: intern(path)}
}

// Set is a set of taint elements. Each element carries the reason it first
// got into the set, which is nil for an element that was not produced by
// the analysis of the function (one read from the global state).
type Set map[Elem]*why

func (s Set) sorted() []Elem {
	out := make([]Elem, 0, len(s))
	for e := range s {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return lessElem(out[i], out[j]) })
	return out
}

func lessElem(a, b Elem) bool {
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.label != b.label {
		if x, y := a.Label(), b.Label(); x != y {
			return x < y
		}
	}
	if a.origin != b.origin {
		if x, y := a.Origin(), b.Origin(); x != y {
			return x < y
		}
	}
	if a.Param != b.Param {
		return a.Param < b.Param
	}
	if a.path != b.path {
		if x, y := a.Path(), b.Path(); x != y {
			return x < y
		}
	}
	return a.mods != b.mods && a.Mods() < b.Mods()
}

func (e Elem) String() string {
	if e.Kind == Real {
		return "real(" + e.Label() + "@" + e.Origin() + ")"
	}
	return "param" + strconv.Itoa(int(e.Param)) + e.Path() + "[" + e.Mods() + "]"
}

// --- sanitizer operations ---

// op is "drop these labels, and where something was dropped attach these".
type op struct {
	drops []string
	adds  []string
	// scoped: the sanitizer has neutralises (ADR-115). A value that went through another scoped sanitizer before has the
	// label that one gave, and this one adds its own to it (chained sanitizers: each neutralises its own categories).
	scoped bool
}

func (o op) drops1(label string) bool {
	for _, d := range o.drops {
		if d == "*" || d == label {
			return true
		}
	}
	return false
}

func encodeOp(o op) string {
	e := strings.Join(o.drops, ",") + "+" + strings.Join(o.adds, ",")
	if o.scoped {
		e += "+s"
	}
	return e
}

func decodeOps(mods string) []op {
	if mods == "" {
		return nil
	}
	var out []op
	for _, part := range strings.Split(mods, ";") {
		d, rest, _ := strings.Cut(part, "+")
		a, flag, _ := strings.Cut(rest, "+")
		var o op
		o.scoped = flag == "s"
		if d != "" {
			o.drops = strings.Split(d, ",")
		}
		if a != "" {
			o.adds = strings.Split(a, ",")
		}
		out = append(out, o)
	}
	return out
}

// maxOps bounds the sanitizer operations remembered by a symbolic element.
// Beyond it the operations are forgotten, which can only keep more taint:
// the safe direction. Without a bound a sanitizer in a loop would create a
// new element in every iteration.
const maxOps = 4

func joinMods(a, b string) string {
	switch {
	case a == "":
		return capMods(b)
	case b == "":
		return a
	}
	for _, o := range strings.Split(b, ";") {
		if a == o || strings.HasSuffix(a, ";"+o) {
			continue // applying the same operation twice changes nothing
		}
		if strings.Count(a, ";") >= maxOps-1 {
			break
		}
		a = a + ";" + o
	}
	return a
}

func capMods(m string) string {
	parts := strings.Split(m, ";")
	if len(parts) > maxOps {
		parts = parts[:maxOps]
	}
	return strings.Join(parts, ";")
}

// sanitize applies a sanitizer to the elements: concrete labels are dropped
// (and replaced by the added ones), symbolic ones remember the operation so
// that it can be applied once the caller's actual labels are known.
func sanitize(in []Elem, o op, rules *model.Rules) []Elem {
	var out []Elem
	for _, e := range in {
		if e.Kind == Param {
			e = e.WithMods(joinMods(e.Mods(), encodeOp(o)))
			out = append(out, e)
			continue
		}
		if !o.drops1(e.Label()) {
			if o.scoped && rules.ChainsOnto(e.Label(), o.drops) {
				for _, a := range o.adds {
					out = append(out, RealElem(rules.ChainLabel(e.Label(), a), e.Origin()))
				}
				continue
			}
			out = append(out, e)
			continue
		}
		for _, a := range o.adds {
			out = append(out, RealElem(a, e.Origin()))
		}
	}
	return out
}

// applyMods applies the recorded operations of a symbolic element to a
// concrete element of the caller.
func applyMods(actual Elem, mods string, rules *model.Rules) []Elem {
	cur := []Elem{actual}
	for _, o := range decodeOps(mods) {
		cur = sanitize(cur, o, rules)
	}
	return cur
}

// --- access paths ---
//
// A path is "" (the whole object) or a sequence of "/segment" strings:
// "/Name" for a field, "/[]" for any element of a slice, array, map or
// channel, "[k]" for map keys, and "/*" for "what the pointer stored here
// points to".

func pathJoin(p, seg string) string { return p + "/" + seg }

func pathDepth(p string) int { return strings.Count(p, "/") }

func pathTrunc(p string, k int) (string, bool) {
	if pathDepth(p) <= k {
		return p, false
	}
	idx := 0
	for i := 0; i < k; i++ {
		j := strings.Index(p[idx+1:], "/")
		if j < 0 {
			return p, false
		}
		idx += j + 1
	}
	return p[:idx], true
}

// pathHasPrefix reports whether prefix is a (non-strict) path prefix of p.
func pathHasPrefix(p, prefix string) bool {
	return prefix == "" || p == prefix || strings.HasPrefix(p, prefix+"/")
}
