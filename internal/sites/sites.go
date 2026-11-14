// Package sites enumerates the places where a rule matches: source sites,
// where untrusted data enters, and sink sites, where it must not arrive.
//
// Enumeration is level L0 (rule matching on resolved symbols) plus the L1
// literal mark. It needs neither a call graph nor any data flow analysis, so
// it is fast. Every site has a stable ID and a fingerprint (ADR-028).
package sites

import (
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/model"
)

// Roles of a site.
const (
	RoleSource = "source"
	RoleSink   = "sink"
)

// Site is one place where a rule matches.
type Site struct {
	ID          string            `json:"id"`
	Role        string            `json:"role"`
	Kind        string            `json:"kind"` // call_result, param_type, field, call
	Func        string            `json:"func"`
	Symbol      string            `json:"symbol"`
	Category    string            `json:"category,omitempty"`
	Labels      []string          `json:"labels,omitempty"` // source labels, or the labels a sink accepts
	Position    gossa.Position    `json:"position"`
	Snippet     string            `json:"snippet"`
	LiteralOnly *bool             `json:"literal_only,omitempty"` // L1, sinks only
	Fingerprint map[string]string `json:"fingerprint"`

	// Internal references for the analysis engine.
	Fn    *ssa.Function   `json:"-"`
	Instr ssa.Instruction `json:"-"`
	Param *ssa.Parameter  `json:"-"`
	Sink  []model.Sink    `json:"-"`
	Src   []model.Source  `json:"-"`
	pos   token.Pos
	order int
}

// Result is the enumeration of a program.
type Result struct {
	Sites []*Site
	// ByInstr maps a call or field instruction to the sites it carries.
	ByInstr map[ssa.Instruction][]*Site
	// ByParam maps a parameter to its source sites (param_type and param).
	ByParam map[*ssa.Parameter][]*Site
	// ByID finds a site by ID.
	ByID map[string]*Site
}

// Enumerate finds all source and sink sites in the functions of the program
// that belong to the loaded packages.
func Enumerate(p *gossa.Program, rules *model.Rules) *Result {
	res := &Result{
		ByInstr: map[ssa.Instruction][]*Site{},
		ByParam: map[*ssa.Parameter][]*Site{},
		ByID:    map[string]*Site{},
	}
	for _, fn := range p.Funcs {
		if !p.Own[fn] {
			continue
		}
		var raw []*Site
		order := 0
		add := func(s *Site) {
			s.Fn = fn
			s.Func = gossa.FuncID(fn)
			s.order = order
			order++
			raw = append(raw, s)
		}

		// Entry points declared by function: the framework hands taint to
		// these parameters.
		if sym := gossa.Symbol(fn); sym != "" {
			for _, src := range rules.ParamFuncs[sym] {
				off := 0
				if fn.Signature.Recv() != nil {
					off = 1
					if src.Recv && len(fn.Params) > 0 {
						add(&Site{Role: RoleSource, Kind: model.KindParam, Symbol: sym, Labels: src.Labels, Param: fn.Params[0], Src: []model.Source{src}, pos: fn.Params[0].Pos()})
					}
				}
				for _, i := range src.Params {
					if i >= 0 && i+off < len(fn.Params) {
						prm := fn.Params[i+off]
						add(&Site{Role: RoleSource, Kind: model.KindParam, Symbol: sym, Labels: src.Labels, Param: prm, Src: []model.Source{src}, pos: prm.Pos()})
					}
				}
			}
		}
		for _, prm := range fn.Params {
			t := types.TypeString(prm.Type(), nil)
			for _, src := range rules.ParamTypes {
				if src.Type == t {
					add(&Site{Role: RoleSource, Kind: model.KindParamType, Symbol: t, Labels: src.Labels, Param: prm, Src: []model.Source{src}, pos: prm.Pos()})
				}
			}
		}
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				switch in := in.(type) {
				case ssa.CallInstruction:
					sym := gossa.CalleeSymbol(in.Common())
					if sym == "" {
						continue
					}
					if srcs := rules.CallSources[sym]; len(srcs) > 0 {
						add(&Site{Role: RoleSource, Kind: model.KindCallResult, Symbol: sym, Labels: labelsOf(srcs), Instr: in, Src: srcs, pos: in.Pos()})
					}
					if sinks := rules.Sinks[sym]; len(sinks) > 0 {
						add(&Site{Role: RoleSink, Kind: "call", Symbol: sym, Category: sinks[0].Category, Labels: acceptsOf(sinks), Instr: in, Sink: sinks, pos: in.Pos()})
					}
				case *ssa.FieldAddr:
					if key := fieldKey(in.X.Type(), in.Field); key != "" {
						if srcs := rules.Fields[key]; len(srcs) > 0 {
							add(&Site{Role: RoleSource, Kind: model.KindField, Symbol: key, Labels: labelsOf(srcs), Instr: in, Src: srcs, pos: in.Pos()})
						}
					}
				case *ssa.Field:
					if key := fieldKey(in.X.Type(), in.Field); key != "" {
						if srcs := rules.Fields[key]; len(srcs) > 0 {
							add(&Site{Role: RoleSource, Kind: model.KindField, Symbol: key, Labels: labelsOf(srcs), Instr: in, Src: srcs, pos: in.Pos()})
						}
					}
				}
			}
		}
		if len(raw) == 0 {
			continue
		}
		res.finish(p, fn, raw)
	}
	sort.SliceStable(res.Sites, func(i, j int) bool {
		a, b := res.Sites[i], res.Sites[j]
		if a.Position.File != b.Position.File {
			return a.Position.File < b.Position.File
		}
		if a.Position.Offset != b.Position.Offset {
			return a.Position.Offset < b.Position.Offset
		}
		return a.ID < b.ID
	})
	return res
}

// finish assigns ordinals, fingerprints and positions to the sites of one
// function.
func (res *Result) finish(p *gossa.Program, fn *ssa.Function, raw []*Site) {
	// Source order, ties broken by instruction order, makes ordinals follow
	// what a reader sees.
	sort.SliceStable(raw, func(i, j int) bool {
		if raw[i].pos != raw[j].pos {
			return raw[i].pos < raw[j].pos
		}
		return raw[i].order < raw[j].order
	})
	body := fingerprint.Hash(p.BodyText(fn))
	ordinals := map[string]int{}
	identical := map[string]int{}
	for _, s := range raw {
		s.Position = p.Position(s.pos)
		s.Snippet = p.Snippet(s.pos)
		expr := p.ExprText(s.pos)
		exprHash := fingerprint.Hash(expr)

		okey := s.Role + "|" + s.Symbol
		ordinals[okey]++
		ord := ordinals[okey]
		ikey := okey + "|" + exprHash
		identical[ikey]++
		iord := identical[ikey]

		structural := fingerprint.Hash(s.Func, s.Role, s.Symbol, itoa(ord))
		s.Fingerprint = map[string]string{
			fingerprint.Structural: structural,
			fingerprint.Content:    fingerprint.Hash(s.Func, s.Role, s.Symbol, exprHash, itoa(iord)),
			fingerprint.Expr:       exprHash,
			fingerprint.Body:       body,
		}
		s.ID = fingerprint.IDPrefix + structural

		if s.Role == RoleSink && s.Instr != nil {
			lit := literalOnly(s)
			s.LiteralOnly = &lit
		}
		res.Sites = append(res.Sites, s)
		res.ByID[s.ID] = s
		if s.Instr != nil {
			res.ByInstr[s.Instr] = append(res.ByInstr[s.Instr], s)
		}
		if s.Param != nil {
			res.ByParam[s.Param] = append(res.ByParam[s.Param], s)
		}
	}
}

// literalOnly implements the L1 mark: every dangerous argument of the sink is
// a compile-time constant.
func literalOnly(s *Site) bool {
	call, ok := s.Instr.(ssa.CallInstruction)
	if !ok {
		return false
	}
	recv, args := gossa.CallArgs(call.Common())
	var checked int
	for _, i := range SinkArgs(s.Sink, len(args)) {
		checked++
		if !gossa.IsConstant(args[i]) {
			return false
		}
	}
	for _, sk := range s.Sink {
		if sk.Recv {
			checked++
			if recv == nil || !gossa.IsConstant(recv) {
				return false
			}
		}
	}
	return checked > 0
}

// SinkArgs returns the dangerous argument positions of a call with n
// arguments, according to the sink rules (all arguments when none is given).
func SinkArgs(sinks []model.Sink, n int) []int {
	seen := map[int]bool{}
	var out []int
	for _, sk := range sinks {
		if len(sk.Args) == 0 {
			for i := 0; i < n; i++ {
				if !seen[i] {
					seen[i] = true
					out = append(out, i)
				}
			}
			continue
		}
		for _, i := range sk.Args {
			if i >= 0 && i < n && !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	sort.Ints(out)
	return out
}

func fieldKey(t types.Type, idx int) string {
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok || idx < 0 || idx >= st.NumFields() {
		return ""
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name() + "." + st.Field(idx).Name()
}

func labelsOf(srcs []model.Source) []string {
	set := map[string]bool{}
	for _, s := range srcs {
		for _, l := range s.Labels {
			set[l] = true
		}
	}
	return sortedKeys(set)
}

// acceptsOf merges the labels the sink rules react to. A rule without a
// label list reacts to every label, so the merged answer is "any" (nil).
func acceptsOf(sinks []model.Sink) []string {
	set := map[string]bool{}
	for _, s := range sinks {
		if len(s.Accepts) == 0 {
			return nil
		}
		for _, l := range s.Accepts {
			set[l] = true
		}
	}
	return sortedKeys(set)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
