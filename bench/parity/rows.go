package main

import (
	"fmt"
	"strings"

	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// Shapes of a row (or of a function): what the default ("what goes in may come out", receiver and
// arguments to the results only, and nothing written to an argument or the receiver) gets wrong.
const (
	ShapeValue   = "i"    // returns a value; the default already carries the flow
	ShapeWrite   = "ii"   // writes to an argument or the receiver; the default drops the flow
	ShapeCall    = "iii"  // calls a callback (CB-LIST); the default does not carry it
	ShapeRule    = "iv"   // source / sink / sanitizer / boundary: the model is the point
	ShapePure    = "pure" // says that nothing flows (cuts the flow)
	ShapeUnknown = "?"    // an unmodeled function whose shape is not known
	ShapeNone    = "-"    // no result and nothing it writes to (not a carrier)
)

// Row is one rule of the built-in model.
type Row struct {
	Lang   string `json:"lang"`
	File   string `json:"file"`
	Role   string `json:"role"` // source, sink, sanitizer, propagator, config_read, global
	Kind   string `json:"kind,omitempty"`
	Symbol string `json:"symbol"`
	Match  string `json:"match,omitempty"`
	Shape  string `json:"shape"`
	// Call is true when the key is the name of a function that a call or a definition shows (the row can
	// be hit by an IR); false for a type, a field, a variable or a marker.
	Call bool `json:"call"`
	Pure bool `json:"pure,omitempty"`
}

// callbackNames are the last names of functions that call a function they are given. The model format
// cannot say it (CB-LIST), so a row is of this shape by its name only: a heuristic, and the table says so.
var callbackNames = map[string]bool{
	// go
	"sort.Slice": true, "sort.SliceStable": true, "sort.Search": true, "slices.SortFunc": true, "slices.IndexFunc": true,
	"strings.Map": true, "strings.FieldsFunc": true, "strings.IndexFunc": true, "strings.TrimFunc": true,
	"sync.Once.Do": true, "sync.Map.Range": true, "filepath.Walk": true, "filepath.WalkDir": true, "fs.WalkDir": true,
	"http.HandlerFunc": true, "http.HandleFunc": true, "time.AfterFunc": true,
	// dart and rust: the member name
	"then": true, "catchError": true, "whenComplete": true, "map": true, "forEach": true, "where": true, "fold": true,
	"reduce": true, "any": true, "every": true, "expand": true, "sort": true, "putIfAbsent": true, "update": true,
	"removeWhere": true, "retainWhere": true, "firstWhere": true, "lastWhere": true, "singleWhere": true,
	"listen": true, "transform": true, "asyncMap": true, "runZonedGuarded": true, "map_err": true, "and_then": true,
	"or_else": true, "unwrap_or_else": true, "map_or": true, "map_or_else": true, "for_each": true, "filter": true,
	"filter_map": true, "flat_map": true, "sort_by": true, "sort_by_key": true, "retain": true, "then_some": true,
	"ok_or_else": true, "get_or_insert_with": true, "or_insert_with": true, "inspect": true, "take_while": true,
	"skip_while": true, "find": true, "position": true, "max_by": true, "min_by": true, "partition": true,
	"spawn": true, "scope": true,
}

// IsCallbackName says whether a symbol is of a function known to call what it is given.
func IsCallbackName(lang, sym string) bool {
	switch lang {
	case "go":
		return callbackNames[goShort(sym)]
	case "dart":
		return callbackNames[memberOf(lang, sym)]
	default:
		return callbackNames[memberOf(lang, sym)]
	}
}

// goShort is "pkg.Func" or "pkg.Type.Method" with the last element of the import path.
func goShort(sym string) string {
	s := sym
	recv := ""
	if strings.HasPrefix(s, "(") {
		i := strings.Index(s, ")")
		if i < 0 {
			return s
		}
		recv = strings.TrimPrefix(s[1:i], "*")
		s = s[i+1:] // ".M"
		j := strings.LastIndex(recv, "/")
		recv = recv[j+1:]
		return recv + s
	}
	j := strings.LastIndex(s, "/")
	return s[j+1:]
}

// memberOf is the last name of a Dart or Rust symbol.
func memberOf(lang, sym string) string {
	switch lang {
	case "dart":
		if i := strings.LastIndex(sym, "#"); i >= 0 {
			sym = sym[i+1:]
		}
		if i := strings.LastIndex(sym, "."); i >= 0 {
			sym = sym[i+1:]
		}
		return sym
	default:
		if i := strings.LastIndex(sym, "::"); i >= 0 {
			sym = sym[i+2:]
		}
		return sym
	}
}

func langOfFile(name string) string {
	switch {
	case strings.HasPrefix(name, "go-"):
		return "go"
	case strings.HasPrefix(name, "dart-"):
		return "dart"
	case strings.HasPrefix(name, "rust-"):
		return "rust"
	}
	return "?"
}

func propShape(lang string, p model.Propagator) string {
	if p.Pure {
		return ShapePure
	}
	if p.Boundary != "" {
		return ShapeRule
	}
	for _, f := range p.Flows {
		if !strings.HasPrefix(f.To, "result") {
			return ShapeWrite
		}
	}
	if IsCallbackName(lang, p.Symbol) {
		return ShapeCall
	}
	return ShapeValue
}

// LoadRows reads every built-in rule and gives one Row per rule.
func LoadRows() ([]Row, error) {
	sets, err := builtin.Load()
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, rs := range sets {
		file := strings.TrimPrefix(rs.Origin, "builtin:")
		lang := langOfFile(file)
		add := func(role, kind, sym, match, shape string, call, pure bool) {
			out = append(out, Row{Lang: lang, File: file, Role: role, Kind: kind, Symbol: sym, Match: match, Shape: shape, Call: call, Pure: pure})
		}
		for _, s := range rs.Sources {
			switch {
			case s.Kind == model.KindCallResult:
				add("source", s.Kind, s.Symbol, s.Match, ShapeRule, true, false)
			case s.Kind == model.KindParam && s.Symbol != "":
				add("source", s.Kind, s.Symbol, s.Match, ShapeRule, true, false)
			default:
				key := s.Symbol
				if key == "" {
					key = firstNonEmpty(s.Type, s.Field, s.Marker, s.Extends)
				}
				add("source", s.Kind, key, s.Match, ShapeRule, false, false)
			}
		}
		for _, s := range rs.Sinks {
			add("sink", "", s.Symbol, s.Match, ShapeRule, true, false)
		}
		for _, s := range rs.Sanitizers {
			add("sanitizer", "", s.Symbol, s.Match, ShapeRule, true, false)
		}
		for _, p := range rs.Propagators {
			add("propagator", "", p.Symbol, p.Match, propShape(lang, p), true, p.Pure)
		}
		for _, c := range rs.ConfigReads {
			add("config_read", "", c.Symbol, "", ShapeRule, true, false)
		}
		for _, g := range rs.Globals {
			add("global", "", g.Symbol, "", ShapeRule, false, false)
		}
	}
	for _, r := range out {
		if r.Lang == "?" {
			return nil, fmt.Errorf("the language of %s is not known", r.File)
		}
	}
	return out, nil
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

// ModelIndex answers what the model says about a symbol.
type ModelIndex struct {
	Rows   []Row
	bySym  map[string][]int // lang+"\x00"+symbol
	Symbol map[string]bool
}

func NewModelIndex(rows []Row) *ModelIndex {
	m := &ModelIndex{Rows: rows, bySym: map[string][]int{}}
	for i, r := range rows {
		if r.Call {
			k := r.Lang + "\x00" + r.Symbol
			m.bySym[k] = append(m.bySym[k], i)
		}
	}
	return m
}

// ShapeOf is the shape of the rows that name the symbol (the most specific wins: a rule beats a pure beats a write beats a
// callback beats a plain value), or "" when there is no row.
func (m *ModelIndex) ShapeOf(lang, sym string) string {
	idx := m.bySym[lang+"\x00"+sym]
	if len(idx) == 0 {
		return ""
	}
	rank := map[string]int{ShapeRule: 5, ShapePure: 4, ShapeWrite: 3, ShapeCall: 2, ShapeValue: 1}
	best := ""
	for _, i := range idx {
		if rank[m.Rows[i].Shape] > rank[best] {
			best = m.Rows[i].Shape
		}
	}
	return best
}

// HasModel says whether a propagator, sanitizer, source or sink names the symbol (what the engine
// does not treat as "unmodeled").
func (m *ModelIndex) HasModel(lang, sym string) bool {
	return len(m.bySym[lang+"\x00"+sym]) > 0
}

// writerNames are the last names of functions that put their argument into the receiver or into another
// argument. Like callbackNames, a heuristic for the functions that have no row and no type to read (Dart, Rust,
// and the Go that is not the standard library).
var writerNames = map[string]bool{
	"Write": true, "WriteString": true, "WriteByte": true, "WriteRune": true, "Add": true, "Set": true, "Append": true,
	"Push": true, "Put": true, "Store": true, "Insert": true, "Scan": true, "Unmarshal": true, "Decode": true,
	"Bind": true, "ShouldBind": true, "ReadFrom": true, "Copy": true, "AddHeader": true,
	"add": true, "addAll": true, "insert": true, "insertAll": true, "write": true, "writeln": true, "writeAll": true,
	"writeCharCode": true, "addEntries": true, "setAll": true, "setRange": true, "fillRange": true, "replaceRange": true,
	"push": true, "push_str": true, "extend": true, "append": true, "write_all": true, "write_str": true, "write_fmt": true,
	"extend_from_slice": true, "resize": true, "entry": true, "set": true, "put": true, "store": true,
}

// IsWriterName says whether the last name of the symbol is one of a function that writes to what it is given.
func IsWriterName(lang, sym string) bool {
	switch lang {
	case "go":
		return writerNames[sym[strings.LastIndex(sym, ".")+1:]]
	default:
		return writerNames[memberOf(lang, sym)]
	}
}

// IsStd says whether the symbol is of the standard library of its language.
func IsStd(lang, sym string) bool {
	switch lang {
	case "go":
		return !strings.Contains(strings.SplitN(goPkgOf(sym), "/", 2)[0], ".")
	case "dart":
		return strings.HasPrefix(sym, "dart:")
	default:
		return strings.HasPrefix(sym, "std::") || strings.HasPrefix(sym, "core::") || strings.HasPrefix(sym, "alloc::")
	}
}
