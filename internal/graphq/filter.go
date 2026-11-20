package graphq

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yagihash/rivus/internal/graphdb"
)

// filterFuncs resolves the symbol a filter names to functions. A filter that
// names several functions (a short name that is not the end of one only) is an
// error that lists them: a filter that is not what the person meant must not
// pass for one.
func (e *Engine) filterFuncs(flag, spec string) ([]int64, string, error) {
	tg, err := e.Resolve(spec, false)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", flag, err)
	}
	if tg.isAmbiguous() {
		se := &SpecError{Spec: spec, Msg: fmt.Sprintf("%s names several (%s): name one", flag, tg.Desc)}
		for _, f := range tg.AmbiguousFuncs {
			se.Candidates = append(se.Candidates, f.ID)
		}
		for _, c := range tg.Ambiguous {
			se.Candidates = append(se.Candidates, pos(c.File, c.Line, c.Col))
		}
		return nil, "", fmt.Errorf("%s: %w", flag, se)
	}
	if tg.Call != nil {
		return nil, "", fmt.Errorf("%s: %q is a call, and the filter names a function", flag, spec)
	}
	f, err := e.fn(tg.Funcs[0])
	if err != nil {
		return nil, "", err
	}
	d := f.FID
	if len(tg.Funcs) > 1 {
		d += fmt.Sprintf(" (and %d more of the same symbol)", len(tg.Funcs)-1)
	}
	return tg.Funcs, d, nil
}

// implFilter is --only-impl: a call site through an interface that has a
// candidate which is a method of a type named is narrowed to those methods; a
// call site that has none is left alone (the person said what a Writer is, not
// what a Stringer is). It counts the call sites it met, and says them in the
// filter text of the answer.
type implFilter struct {
	e         *Engine
	types     []string
	c         *Common
	idx       int
	method    map[int64]bool
	site      map[int64]bool
	narrow    int
	left      int
	described string
}

func (f *implFilter) names(id int64) bool {
	if v, ok := f.method[id]; ok {
		return v
	}
	v := false
	if fn, err := f.e.fn(id); err == nil {
		for _, t := range f.types {
			if receiverNames(fn.Symbol, t) {
				v = true
				break
			}
		}
	}
	f.method[id] = v
	return v
}

// narrows: some candidate of the call site is a method of a type named.
func (f *implFilter) narrows(callsite int64) bool {
	if v, ok := f.site[callsite]; ok {
		return v
	}
	v := false
	if edges, err := f.e.db.EdgesOfCallSite(callsite); err == nil {
		for i := range edges {
			if f.names(edges[i].To) {
				v = true
				break
			}
		}
	}
	f.site[callsite] = v
	if v {
		f.narrow++
	} else {
		f.left++
	}
	f.c.Filters[f.idx] = f.text()
	return v
}

func (f *implFilter) text() string {
	return fmt.Sprintf("only-impl %s: a call through an interface that has a candidate of a type named so reaches only the methods of those types; a call through an interface with none is not narrowed (static calls and function values are not filtered); of the calls through an interface met so far, narrowed %s and left alone %s",
		strings.Join(f.types, ", "), plural(f.narrow, "call site"), plural(f.left, "call site"))
}

func plural(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}

func (f *implFilter) pass(ed *graphdb.Edge) bool {
	if ed.Kind != graphdb.KindInterface {
		return true
	}
	return !f.narrows(ed.CallSite) || f.names(ed.To)
}

// prepare turns the filters of the options into the predicate of the edges and
// the sets the search needs, and says them in the answer ("filters"). A filter
// is never silent.
func (e *Engine) prepare(o *Options, c *Common) error {
	if !o.filtered() {
		return nil
	}
	avoid := map[int64]bool{}
	for _, s := range o.Avoid {
		ids, d, err := e.filterFuncs("--avoid", s)
		if err != nil {
			return err
		}
		for _, id := range ids {
			avoid[id] = true
		}
		c.Filters = append(c.Filters, "avoid "+d+": no call to or from it")
	}
	avoidCall := map[int64]bool{}
	for _, s := range o.AvoidCall {
		tg, err := e.Resolve(s, true)
		if err != nil {
			return fmt.Errorf("--avoid-call: %w", err)
		}
		if tg.isAmbiguous() {
			se := &SpecError{Spec: s, Msg: fmt.Sprintf("--avoid-call names several (%s): give file:line:col", tg.Desc)}
			for _, c := range tg.Ambiguous {
				se.Candidates = append(se.Candidates, pos(c.File, c.Line, c.Col))
			}
			return fmt.Errorf("--avoid-call: %w", se)
		}
		if tg.Call == nil {
			return fmt.Errorf("--avoid-call: %q is a function, and the filter names a call (file:line:col)", s)
		}
		avoidCall[tg.Call.ID] = true
		c.Filters = append(c.Filters, fmt.Sprintf("avoid-call %s: no edge of this call site; the function that holds it, and its other calls, stay", s))
	}
	o.via = nil
	for _, s := range o.Via {
		ids, d, err := e.filterFuncs("--via", s)
		if err != nil {
			return err
		}
		o.via = append(o.via, ids)
		c.Filters = append(c.Filters, "via "+d+": only what goes through it")
	}
	var impl *implFilter
	if len(o.OnlyImpl) > 0 {
		for _, t := range o.OnlyImpl {
			ok, err := e.db.SymbolContains(t)
			if err != nil {
				return err
			}
			if !ok {
				c.Warnings = append(c.Warnings, fmt.Sprintf("--only-impl %s: no function has %q in its symbol; no call site is narrowed by it", t, t))
			}
		}
		impl = &implFilter{e: e, types: o.OnlyImpl, c: c, idx: len(c.Filters), method: map[int64]bool{}, site: map[int64]bool{}}
		c.Filters = append(c.Filters, impl.text())
	}
	base := o.Edge
	o.Edge = func(ed *graphdb.Edge) bool {
		if base != nil && !base(ed) {
			return false
		}
		if avoid[ed.From] || avoid[ed.To] || avoidCall[ed.CallSite] {
			return false
		}
		return impl == nil || impl.pass(ed)
	}
	return nil
}

// receiverType is the type a method symbol names, without its type arguments:
// "(*x/models/user.User).Name" is x/models/user.User, "<a::P as a::Tr>::run" is
// a::P (the trait is not the type), "a::P::run" is a::P, "X.M" is X.
func receiverType(symbol string) string {
	if strings.HasPrefix(symbol, "<") {
		depth, closeAt := 0, -1
		for i, r := range symbol {
			if r == '<' {
				depth++
			} else if r == '>' {
				depth--
				if depth == 0 {
					closeAt = i
					break
				}
			}
		}
		if closeAt < 0 {
			return ""
		}
		inner := symbol[1:closeAt]
		depth = 0
		for i := 0; i+4 <= len(inner); i++ {
			switch inner[i] {
			case '<':
				depth++
			case '>':
				depth--
			}
			if depth == 0 && inner[i:i+4] == " as " {
				inner = inner[:i]
				break
			}
		}
		return cleanType(inner)
	}
	end := len(symbol)
	for end > 0 {
		r, n := utf8.DecodeLastRuneInString(symbol[:end])
		if !isIdent(r) && r != '$' {
			break
		}
		end -= n
	}
	head := symbol[:end]
	head = strings.TrimSuffix(strings.TrimSuffix(head, "::"), ".")
	head = strings.TrimSuffix(strings.TrimPrefix(head, "("), ")")
	return cleanType(head)
}

// cleanType leaves out the type arguments (<...> and [...]) and the marks of a reference.
func cleanType(t string) string {
	var b strings.Builder
	depth := 0
	for _, r := range t {
		switch r {
		case '<', '[':
			depth++
			continue
		case '>', ']':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.TrimLeft(out, "&*")
	out = strings.TrimPrefix(strings.TrimPrefix(out, "mut "), "dyn ")
	return strings.TrimSpace(out)
}

// receiverNames reports whether the symbol of a method is a method of the type
// named: the name of the type (the last part of the type), or the type with
// its package or path, ending at a boundary. A trait, a package, or a type
// that is only an argument of the type is not the type.
func receiverNames(symbol, typ string) bool {
	if typ == "" {
		return false
	}
	t := receiverType(symbol)
	if t == "" {
		return false
	}
	if t == typ || t[strings.LastIndexAny(t, "./#:")+1:] == typ {
		return true
	}
	if strings.HasSuffix(t, typ) {
		switch t[len(t)-len(typ)-1] {
		case '/', '#', ':':
			return true
		}
	}
	return false
}

// filtered reports whether the answer was narrowed by a filter.
func (o *Options) filtered() bool { return o.nFilters() > 0 }

func (o *Options) nFilters() int {
	return len(o.Avoid) + len(o.Via) + len(o.OnlyImpl) + len(o.AvoidCall)
}

func entryInfo(en *graphdb.Entries) *EntryInfo {
	def := "the functions the extractors mark as entries: in Go main.main and the init of each package, in Dart main, in Rust the entry_fn of the crate"
	if en.Source == "exported" {
		def = "no function starts a program (no main): the exported functions of the code that was read (not of the dependencies); the same starts as the reachability of the taint analysis (sg.Program.Reachable)"
	}
	// the edges: the calls and the candidates of the calls through interfaces and function values, and (the fourth stage of the search, unless --no-refs) the references, which is what the taint analysis follows
	def += ". The path follows the calls in the graph file and the candidates of calls through interfaces and function values; when there is none, it follows the references too (a function named as a value: a handler registered with a router, a callback), which is the rule of the reachability of the taint analysis, so a function it takes as reached has a path here. With --no-refs the references are not followed, and a function that is only named as a value has no path"
	return &EntryInfo{Definition: def, Source: en.Source, Count: len(en.IDs), Langs: en.Langs, Examples: en.Examples}
}
