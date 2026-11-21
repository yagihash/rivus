package taint

import (
	"fmt"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
)

// A sink that names a part of its value (model.SinkPart: docs/DESIGN-field-sinks.md) is reached by
// the taint that is in that part (a sure finding), and by the taint that may be in it but is not told
// apart from the rest of the value (a finding with the uncertainty field_imprecision). The taint that is
// in a field beside it is not a flow to the sink.

// partKind is how a cell of a value relates to the field a sink names.
type partKind int

const (
	partNone  partKind = iota // a field beside the one named (or below a name that is not the one)
	partBelow                 // the field named, or below it
	partAbove                 // the value as a whole, or a part that holds the field named: the field is not told
)

// transparentStep is a step of an access path that is no field name: the pointer behind a field, or
// that an interface holds, and an element of a container. The names a model writes are matched against
// the path with these steps left out (docs/DESIGN-field-sinks.md 3).
func transparentStep(seg string) bool {
	return seg == "*" || strings.HasPrefix(seg, "[")
}

// matchParts matches the names of a field (the part of a sink) against the path of a cell, relative to
// the value the sink reads (rel: "/Task/*/MessageType"). The names are a subsequence of the path: the
// steps that are no names may stand between them, the names of the path may not be left out. When the
// path is only the first names (or none), the cell holds more than the field: rest is the names left.
func matchParts(names []string, rel string) (kind partKind, rest []string) {
	i := 0
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			continue
		}
		if i == len(names) {
			return partBelow, nil
		}
		switch {
		case seg == names[i]:
			i++
		case transparentStep(seg):
		default:
			return partNone, nil
		}
	}
	if i == len(names) {
		return partBelow, nil
	}
	return partAbove, names[i:]
}

// checkPart checks one part of a sink: the field named below the value of an argument or the receiver.
func (s *state) checkPart(cc *callCtx, site *sites.Site, p model.SinkPart) {
	var v *sg.Value
	switch {
	case p.Recv:
		v = cc.recv
	case p.Arg != nil && *p.Arg < len(cc.args):
		v = cc.args[*p.Arg]
	}
	if v == nil {
		return
	}
	names := model.FieldNames(p.Field)
	locs := s.locs(v)
	if bad := s.eng.unknownFieldNames(names); len(bad) > 0 {
		// A name that the program does not have: a misspelling, or a field that was renamed. Nothing
		// could be told apart by it: the sink is read whole, and the rule says so.
		note := s.newNote(cc.in, KindFieldUnresolved,
			fmt.Sprintf("the sink %s names the field %s, which is no field the program has (%s): the sink reads the value whole", site.Symbol, p.Field, strings.Join(bad, ", ")), site.Symbol)
		for _, f := range s.flatten(locs) {
			s.reach(site, f, cc.in, note, "")
		}
		return
	}
	opaque := s.touchedByOpaque(v)
	for _, l := range locs {
		for _, f := range s.collect(l) {
			kind, rest := partAbove, names
			if len(f.Cell.Path) >= len(l.Path) { // not a cell that holds the location whole
				kind, rest = matchParts(names, f.Rel)
			}
			switch kind {
			case partBelow:
				s.reach(site, f, cc.in, nil, "")
			case partAbove:
				if f.E.Kind == Param {
					// the caller says what is in the field
					s.reach(site, f, cc.in, nil, strings.Join(rest, "."))
				} else {
					s.reach(site, f, cc.in, s.fieldNote(cc.in.Pos, p.Field, s.whyUnsure(f.Cell.Path), f.Cell.Path), "")
				}
			}
			if opaque && !(kind == partAbove && f.E.Kind != Param) {
				// a function that is not read had the value: it may have moved the taint into the field
				s.reach(site, f, cc.in, s.fieldNote(cc.in.Pos, p.Field, "opaque_callee", f.Cell.Path), "")
			}
		}
	}
}

// whyUnsure says why the cell that holds a field the sink names does not tell it: the value is a
// container's element, the path was cut at K steps, or the value was tainted as a whole.
func (s *state) whyUnsure(cellPath string) string {
	switch {
	case hasElementSegment(cellPath):
		return "element"
	case strings.Count(cellPath, "/") >= s.eng.opts.K:
		return "path_limit"
	}
	return "whole_value"
}

// fieldNote is the uncertainty of a sink that read a field that is not told apart.
func (s *state) fieldNote(pos ir.Pos, field, why, cell string) *Uncertainty {
	return s.pathNote(KindFieldImprecision,
		fmt.Sprintf("why: %s; the sink reads the field %s of the value, and the taint is at %q, where the fields are not told apart", why, field, cell), pos)
}

// touchedByOpaque reports whether the value (or what it points to) was given to a function that the
// analysis has neither the body nor a model of: it may have changed any part of it.
func (s *state) touchedByOpaque(v *sg.Value) bool {
	if len(s.opaque) == 0 {
		return false
	}
	for _, l := range s.locs(v) {
		if s.opaque[l.Root] {
			return true
		}
	}
	for _, l := range s.pointeeOfInterface(v) {
		if s.opaque[l.Root] {
			return true
		}
	}
	return false
}

// noteOpaque remembers the values that a call that is neither read nor modelled was given and that it
// may change: what they point to, an interface that holds a pointer, a struct that holds references. It
// adds no taint: the only thing that reads it is a sink that names a part (touchedByOpaque).
func (s *state) noteOpaque(cc *callCtx) {
	for _, v := range cc.argv() {
		if v == nil {
			continue
		}
		switch v.Class {
		case ir.ClassPointer, ir.ClassContainer, ir.ClassInterface, ir.ClassStruct:
		default:
			continue
		}
		roots := append(append([]Loc(nil), s.locs(v)...), s.pointeeOfInterface(v)...)
		for _, l := range roots {
			if !s.opaque[l.Root] {
				if s.opaque == nil {
					s.opaque = map[Root]bool{}
				}
				s.opaque[l.Root] = true
				s.changed = true
			}
		}
	}
}

// unknownFieldNames are the names (of the field a sink names) that no instruction of the program uses as
// a field: a field that is neither read nor written anywhere cannot be where taint is. "[]" is no name.
func (e *Engine) unknownFieldNames(names []string) []string {
	e.fieldNamesOnce.Do(func() {
		e.fieldNames = map[string]bool{}
		for _, fn := range e.prog.Funcs {
			for _, b := range fn.Blocks {
				for _, in := range b.Instrs {
					if in.IR == nil {
						continue
					}
					if in.IR.Field != "" {
						e.fieldNames[in.IR.Field] = true
					}
					for _, st := range in.IR.Path {
						if st.Kind == ir.StepField {
							e.fieldNames[st.Field] = true
						}
					}
				}
			}
		}
	})
	var bad []string
	for _, n := range names {
		if n != "[]" && !e.fieldNames[n] {
			bad = append(bad, n)
		}
	}
	return bad
}
