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

// prepare turns the filters of the options into the predicate of the edges and
// the sets the search needs, and says them in the answer ("filters"). A filter
// is never silent.
func (e *Engine) prepare(o *Options, c *Common) error {
	if len(o.Avoid)+len(o.Via)+len(o.OnlyImpl) == 0 {
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
	o.via = nil
	for _, s := range o.Via {
		ids, d, err := e.filterFuncs("--via", s)
		if err != nil {
			return err
		}
		o.via = append(o.via, ids)
		c.Filters = append(c.Filters, "via "+d+": only what goes through it")
	}
	for _, t := range o.OnlyImpl {
		ok, err := e.db.SymbolContains(t)
		if err != nil {
			return err
		}
		if !ok {
			c.Warnings = append(c.Warnings, fmt.Sprintf("--only-impl %s: no function has %q in its symbol; every call through an interface is cut", t, t))
		}
		c.Filters = append(c.Filters, fmt.Sprintf("only-impl %s: a call through an interface reaches only the methods of types named %s (static calls and function values are not filtered)", t, t))
	}
	base, impls := o.Edge, o.OnlyImpl
	verdict := map[int64]bool{}
	o.Edge = func(ed *graphdb.Edge) bool {
		if base != nil && !base(ed) {
			return false
		}
		if avoid[ed.From] || avoid[ed.To] {
			return false
		}
		if len(impls) == 0 || ed.Kind != graphdb.KindInterface {
			return true
		}
		if v, ok := verdict[ed.To]; ok {
			return v
		}
		f, err := e.fn(ed.To)
		v := err != nil
		if err == nil {
			for _, t := range impls {
				if receiverNames(f.Symbol, t) {
					v = true
					break
				}
			}
		}
		verdict[ed.To] = v
		return v
	}
	return nil
}

// receiverNames reports whether the symbol of a method names the type (the
// part before the method's own name), at the boundaries of an identifier.
func receiverNames(symbol, typ string) bool {
	end := len(symbol)
	for end > 0 {
		r, n := utf8.DecodeLastRuneInString(symbol[:end])
		if !isIdent(r) {
			break
		}
		end -= n
	}
	head := symbol[:end]
	if typ == "" {
		return false
	}
	for from := 0; ; {
		i := strings.Index(head[from:], typ)
		if i < 0 {
			return false
		}
		i += from
		before := true
		if i > 0 {
			r, _ := utf8.DecodeLastRuneInString(head[:i])
			before = !isIdent(r)
		}
		after := true
		if j := i + len(typ); j < len(head) {
			r, _ := utf8.DecodeRuneInString(head[j:])
			after = !isIdent(r)
		}
		if before && after {
			return true
		}
		from = i + 1
	}
}

// filtered reports whether the answer was narrowed by a filter.
func (o *Options) filtered() bool { return len(o.Avoid)+len(o.Via)+len(o.OnlyImpl) > 0 }

func entryInfo(en *graphdb.Entries) *EntryInfo {
	def := "the functions the extractors mark as entries: in Go main.main and the init of each package, in Dart main, in Rust the entry_fn of the crate"
	if en.Source == "exported" {
		def = "no function starts a program (no main): the exported functions of the code that was read (not of the dependencies), as the reachability of the taint analysis takes them for a library"
	}
	return &EntryInfo{Definition: def, Source: en.Source, Count: len(en.IDs), Langs: en.Langs, Examples: en.Examples}
}
