package graphdb

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// The reading of the graph for the searches of "rivus graph" (callers, callees,
// path). A search asks for the neighbours of one function at a time, by the
// indexes (edges_from, edges_to, callsites_func), so that it never holds the
// graph in memory. The connection is one, so a query is read to its end before
// the next one is made.

// Func is a function row.
type Func struct {
	ID      int64
	FID     string
	Symbol  string
	File    string
	Line    int
	Col     int
	HasBody bool
	// Dep is a function of a dependency (not of the code that was asked about).
	Dep  bool
	Lang string
}

// Edge is a call site and a function it may reach.
type Edge struct {
	CallSite int64
	From, To int64
	Kind     string
	// NCand is the number of candidates of the call site.
	NCand int
	// File, Line and Col are the position of the call site.
	File      string
	Line, Col int
	// A reference (Kind is KindRef): the row of the refs table, how the function is
	// named as a value (RefArg, RefClosure, RefOther), and the callee of the call
	// that takes it as an argument. It has no call site (CallSite is 0), and the
	// position is where the reference is written.
	Ref     int64
	RefKind string
	Via     string
}

// Static reports whether the callee is named in the call.
func (e *Edge) Static() bool { return e.Kind == KindStatic }

// EdgeFilter says which edges a neighbour query returns. The calls that go
// through an interface or a function value have many candidates, so a filter
// is applied in the query: a hub is not read to be thrown away.
type EdgeFilter struct {
	// StaticOnly keeps only the calls whose callee is named.
	StaticOnly bool
	// MaxDynamic keeps a dynamic call only when it has at most this many
	// candidates (negative: no limit).
	MaxDynamic int
	// Refs adds the references (a function named as a value) as edges of the
	// kind KindRef, from the function that holds the reference to the one it names.
	Refs bool
}

// AllEdges is the filter that keeps every edge.
var AllEdges = EdgeFilter{MaxDynamic: -1}

func (f EdgeFilter) args() (staticOnly, maxDyn int) {
	if f.StaticOnly {
		staticOnly = 1
	}
	maxDyn = f.MaxDynamic
	if maxDyn < 0 {
		maxDyn = 1 << 30
	}
	return
}

const edgeCols = `e.callsite_id, e.from_func, e.to_func, e.kind, e.ncand, coalesce(fl.path,''), c.line, c.col`
const edgeJoin = `FROM edges e JOIN callsites c ON c.id = e.callsite_id LEFT JOIN files fl ON fl.id = c.file_id`
const edgeFilterSQL = `AND (e.kind = 'static' OR (? = 0 AND e.ncand <= ?)) ORDER BY fl.path, c.line, c.col, e.to_func, e.from_func`

func (d *DB) edges(q string, args ...any) ([]Edge, error) {
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.CallSite, &e.From, &e.To, &e.Kind, &e.NCand, &e.File, &e.Line, &e.Col); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// OutEdges are the calls a function makes (forward).
func (d *DB) OutEdges(from int64, f EdgeFilter) ([]Edge, error) {
	s, m := f.args()
	out, err := d.edges(`SELECT `+edgeCols+` `+edgeJoin+` WHERE e.from_func = ? `+edgeFilterSQL, from, s, m)
	if err != nil || !f.Refs {
		return out, err
	}
	refs, err := d.refEdges(`r.owner_func = ?`, from)
	return append(out, refs...), err
}

// InEdges are the calls that reach a function (backward).
func (d *DB) InEdges(to int64, f EdgeFilter) ([]Edge, error) {
	s, m := f.args()
	out, err := d.edges(`SELECT `+edgeCols+` `+edgeJoin+` WHERE e.to_func = ? `+edgeFilterSQL, to, s, m)
	if err != nil || !f.Refs {
		return out, err
	}
	refs, err := d.refEdges(`r.target_func = ?`, to)
	return append(out, refs...), err
}

// refEdges reads the references that a condition picks, as edges of the kind
// KindRef, in the order of their positions.
func (d *DB) refEdges(cond string, arg int64) ([]Edge, error) {
	rows, err := d.db.Query(`SELECT r.id, r.owner_func, r.target_func, r.kind, coalesce(r.via_callee,''), coalesce(fl.path,''), r.line, r.col
		FROM refs r LEFT JOIN files fl ON fl.id = r.file_id WHERE `+cond+` ORDER BY fl.path, r.line, r.col, r.target_func, r.owner_func, r.id`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Edge
	for rows.Next() {
		e := Edge{Kind: KindRef}
		if err := rows.Scan(&e.Ref, &e.From, &e.To, &e.RefKind, &e.Via, &e.File, &e.Line, &e.Col); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RefCount counts the references a function holds (out) or that name it (in).
func (d *DB) RefCount(id int64, in bool) (int, error) {
	col := "owner_func"
	if in {
		col = "target_func"
	}
	var n int
	err := d.db.QueryRow(`SELECT count(*) FROM refs WHERE `+col+` = ?`, id).Scan(&n)
	return n, err
}

// RefStats are what the file says of where a function is named as a value.
type RefStats struct {
	// Total is the number of references to it.
	Total int
	// Owners are the functions that hold most of them, the most first.
	Owners []RefOwner
}

// RefOwner is a function that holds references to another, and how many.
type RefOwner struct {
	ID    int64
	Count int
}

// RefsTo counts the references to a function, and names the functions that hold
// the most (up to top).
func (d *DB) RefsTo(id int64, top int) (RefStats, error) {
	var st RefStats
	if err := d.db.QueryRow(`SELECT count(*) FROM refs WHERE target_func = ?`, id).Scan(&st.Total); err != nil || st.Total == 0 {
		return st, err
	}
	rows, err := d.db.Query(`SELECT owner_func, count(*) c FROM refs WHERE target_func = ? GROUP BY owner_func ORDER BY c DESC, owner_func LIMIT ?`, id, top)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var o RefOwner
		if err := rows.Scan(&o.ID, &o.Count); err != nil {
			return st, err
		}
		st.Owners = append(st.Owners, o)
	}
	return st, rows.Err()
}

// EdgesOfCallSite are the functions one call may reach.
func (d *DB) EdgesOfCallSite(id int64) ([]Edge, error) {
	return d.edges(`SELECT `+edgeCols+` `+edgeJoin+` WHERE e.callsite_id = ? ORDER BY e.to_func`, id)
}

// HasOut reports whether a function calls anything.
func (d *DB) HasOut(from int64) (bool, error) {
	var n int
	err := d.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM edges WHERE from_func = ?)`, from).Scan(&n)
	return n == 1, err
}

// FuncFacts are what the file says of one function, as a reader that wants to
// know why nothing reaches it needs: who calls it, and what it is. Only what
// the structure layer has.
type FuncFacts struct {
	Exported bool
	HasBody  bool
	Dep      bool
	// Lang is the language of the function (go, dart, rust).
	Lang string
	// StaticCallSites are the call sites that name it; DynamicCallSites those
	// through an interface or a function value that list it as a candidate.
	StaticCallSites  int
	DynamicCallSites int
}

// FactsOf reads the facts of a function.
func (d *DB) FactsOf(id int64) (FuncFacts, error) {
	var ft FuncFacts
	var ex, body, dep int
	if err := d.db.QueryRow(`SELECT exported, has_body, dep, lang FROM funcs WHERE id = ?`, id).Scan(&ex, &body, &dep, &ft.Lang); err != nil {
		return ft, err
	}
	ft.Exported, ft.HasBody, ft.Dep = ex == 1, body == 1, dep == 1
	err := d.db.QueryRow(`SELECT coalesce(sum(kind = 'static'),0), coalesce(sum(kind IN ('interface','value')),0) FROM (SELECT DISTINCT callsite_id, kind FROM edges WHERE to_func = ?)`, id).Scan(&ft.StaticCallSites, &ft.DynamicCallSites)
	return ft, err
}

// HasIn reports whether anything calls a function.
func (d *DB) HasIn(to int64) (bool, error) {
	var n int
	err := d.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM edges WHERE to_func = ?)`, to).Scan(&n)
	return n == 1, err
}

// CallRow is a call site row, as the searches need it.
type CallRow struct {
	ID     int64
	Func   int64
	Kind   string
	NCand  int
	Callee string
	File   string
	Line   int
	Col    int
}

func (d *DB) callRows(q string, args ...any) ([]CallRow, error) {
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CallRow
	for rows.Next() {
		var c CallRow
		var callee sql.NullString
		if err := rows.Scan(&c.ID, &c.Func, &c.Kind, &c.NCand, &callee, &c.File, &c.Line, &c.Col); err != nil {
			return nil, err
		}
		c.Callee = callee.String
		out = append(out, c)
	}
	return out, rows.Err()
}

const callCols = `c.id, c.func_id, c.kind, c.ncand, c.callee, coalesce(fl.path,''), c.line, c.col`
const callJoin = `FROM callsites c LEFT JOIN files fl ON fl.id = c.file_id`

// UnexpandedCalls are the dynamic calls of a function that have more than
// max candidates (a call with none included: nothing says where it goes).
func (d *DB) UnexpandedCalls(fn int64, max int) ([]CallRow, error) {
	return d.callRows(`SELECT `+callCols+` `+callJoin+` WHERE c.func_id = ? AND c.kind IN ('interface','value') AND (c.ncand > ? OR c.ncand = 0) ORDER BY c.line, c.col, c.id`, fn, max)
}

// BuiltinCalls are the calls of a builtin in a function.
func (d *DB) BuiltinCalls(fn int64) ([]CallRow, error) {
	return d.callRows(`SELECT `+callCols+` `+callJoin+` WHERE c.func_id = ? AND c.kind = 'builtin' ORDER BY c.line, c.col, c.id`, fn)
}

// BuiltinCount counts the calls of a builtin in a function.
func (d *DB) BuiltinCount(fn int64) (int, error) {
	var n int
	err := d.db.QueryRow(`SELECT count(*) FROM callsites WHERE func_id = ? AND kind = 'builtin'`, fn).Scan(&n)
	return n, err
}

// CallRowByID reads one call site.
func (d *DB) CallRowByID(id int64) (*CallRow, error) {
	rs, err := d.callRows(`SELECT `+callCols+` `+callJoin+` WHERE c.id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, fmt.Errorf("no call site %d", id)
	}
	return &rs[0], nil
}

// CallRowsAt reads the call sites at a position: with col 0, those of the line;
// else those that start at the column.
func (d *DB) CallRowsAt(file string, line, col int) ([]CallRow, error) {
	q := `SELECT ` + callCols + ` ` + callJoin + ` WHERE fl.path = ? AND c.line = ?`
	args := []any{file, line}
	if col > 0 {
		q += ` AND c.col = ?`
		args = append(args, col)
	}
	return d.callRows(q+` ORDER BY c.col, c.id`, args...)
}

// CallRowsBefore reads the call sites that start on the line at the last column
// that is not after col (the innermost call that has begun at that point).
func (d *DB) CallRowsBefore(file string, line, col int) ([]CallRow, error) {
	return d.callRows(`SELECT `+callCols+` `+callJoin+` WHERE fl.path = ? AND c.line = ? AND c.col = (
		SELECT max(c2.col) FROM callsites c2 JOIN files f2 ON f2.id = c2.file_id WHERE f2.path = ? AND c2.line = ? AND c2.col <= ? AND c2.kind != 'builtin')
		AND c.kind != 'builtin' ORDER BY c.id`, file, line, file, line, col)
}

// HubRef is a dynamic call site with many candidates.
type HubRef struct {
	ID    int64
	NCand int
}

// HubOut are the dynamic calls of a function with more than max candidates (all
// of them: the callers count them without a limit, and name the largest).
func (d *DB) HubOut(fn int64, max int) ([]HubRef, error) {
	return d.hubRefs(`SELECT id, ncand FROM callsites WHERE func_id = ? AND kind IN ('interface','value') AND ncand > ?`, fn, max)
}

// HubIn are the dynamic calls with more than max candidates that may reach a
// function (a call site once, however many of its candidates are the function).
func (d *DB) HubIn(fn int64, max int) ([]HubRef, error) {
	return d.hubRefs(`SELECT DISTINCT callsite_id, ncand FROM edges WHERE to_func = ? AND kind != 'static' AND ncand > ?`, fn, max)
}

func (d *DB) hubRefs(q string, args ...any) ([]HubRef, error) {
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HubRef
	for rows.Next() {
		var h HubRef
		if err := rows.Scan(&h.ID, &h.NCand); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// CandidateHistogram counts the dynamic call sites by the number of their
// candidates, in buckets: 0, 1, 2, 3, 4-5, 6-10, 11-20, 21-50, 51-100, 101+.
func (d *DB) CandidateHistogram(kind string) (map[string]int, error) {
	rows, err := d.db.Query(`SELECT CASE WHEN ncand <= 3 THEN CAST(ncand AS TEXT) WHEN ncand <= 5 THEN '4-5' WHEN ncand <= 10 THEN '6-10'
		WHEN ncand <= 20 THEN '11-20' WHEN ncand <= 50 THEN '21-50' WHEN ncand <= 100 THEN '51-100' ELSE '101+' END, count(*)
		FROM callsites WHERE kind = ? GROUP BY 1`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var b string
		var n int
		if err := rows.Scan(&b, &n); err != nil {
			return nil, err
		}
		out[b] = n
	}
	return out, rows.Err()
}

// FuncByID reads a function by its row.
func (d *DB) FuncByID(id int64) (*Func, error) {
	fs, err := d.funcs(`WHERE f.id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(fs) == 0 {
		return nil, fmt.Errorf("no function %d", id)
	}
	return &fs[0], nil
}

func (d *DB) funcs(where string, args ...any) ([]Func, error) {
	rows, err := d.db.Query(`SELECT f.id, f.fid, f.symbol, coalesce(fl.path,''), f.line, f.col, f.has_body, f.dep, f.lang FROM funcs f LEFT JOIN files fl ON fl.id = f.file_id `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Func
	for rows.Next() {
		var f Func
		var body, dep int
		if err := rows.Scan(&f.ID, &f.FID, &f.Symbol, &f.File, &f.Line, &f.Col, &body, &dep, &f.Lang); err != nil {
			return nil, err
		}
		f.HasBody = body == 1
		f.Dep = dep == 1
		out = append(out, f)
	}
	return out, rows.Err()
}

// FuncsNamed finds the functions whose ID or symbol is name. Several functions
// can have one symbol (a method and its wrapper).
func (d *DB) FuncsNamed(name string) ([]Func, error) {
	return d.funcs(`WHERE f.fid = ? OR f.symbol = ? ORDER BY f.has_body DESC, f.fid`, name, name)
}

// SymRow is what the search of a short name reads of every function.
type SymRow struct {
	ID      int64
	FID     string
	Symbol  string
	HasBody bool
	Dep     bool
}

// AllSymbols reads the names of every function (the search of a name that is
// the end of a symbol looks at all of them: there is no index for an end).
func (d *DB) AllSymbols() ([]SymRow, error) {
	rows, err := d.db.Query(`SELECT id, fid, symbol, has_body, dep FROM funcs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SymRow
	for rows.Next() {
		var r SymRow
		var b, dep int
		if err := rows.Scan(&r.ID, &r.FID, &r.Symbol, &b, &dep); err != nil {
			return nil, err
		}
		r.HasBody, r.Dep = b == 1, dep == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// FuncsByIDs reads the functions of some rows, in the order given.
func (d *DB) FuncsByIDs(ids []int64) ([]Func, error) {
	var out []Func
	for _, id := range ids {
		f, err := d.FuncByID(id)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, nil
}

// SymbolContains reports whether some function has text in its symbol (a
// cheap check that a type name is used at all).
func (d *DB) SymbolContains(text string) (bool, error) {
	var n int
	err := d.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM funcs WHERE instr(symbol, ?) > 0)`, text).Scan(&n)
	return n == 1, err
}

// Entries are the functions a program starts from: those the extractor marked
// as an entry (main, the initialisation of the packages), when one of them
// starts a program (is not the initializer of a variable); else the exported
// functions of the code that was read (a library has no start). It is the
// rule of the reachability the taint analysis states (sg.Program.Reachable).
type Entries struct {
	IDs []int64
	// Source is "entry" or "exported"; empty when nothing says where the program starts.
	Source string
	// Examples are some of their IDs.
	Examples []string
	Langs    []string
}

// EntryFuncs finds the functions a program starts from.
func (d *DB) EntryFuncs() (*Entries, error) {
	var starts int
	if err := d.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM funcs WHERE entry = 1 AND init = 0)`).Scan(&starts); err != nil {
		return nil, err
	}
	q, src := `SELECT id, fid, lang FROM funcs WHERE entry = 1 OR (exported = 1 AND dep = 0) ORDER BY fid`, "exported"
	if starts == 1 {
		q, src = `SELECT id, fid, lang FROM funcs WHERE entry = 1 ORDER BY fid`, "entry"
	}
	// with no start, the entries (an initializer) and the exported functions are the roots
	rows, err := d.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	en := &Entries{Source: src}
	langs := map[string]bool{}
	for rows.Next() {
		var id int64
		var fid, lang string
		if err := rows.Scan(&id, &fid, &lang); err != nil {
			return nil, err
		}
		en.IDs = append(en.IDs, id)
		if len(en.Examples) < 5 {
			en.Examples = append(en.Examples, fid)
		}
		langs[lang] = true
	}
	for l := range langs {
		en.Langs = append(en.Langs, l)
	}
	sort.Strings(en.Langs)
	if len(en.IDs) == 0 {
		en.Source = ""
	}
	return en, rows.Err()
}

// SimilarSymbols are symbols that end like name (the last word, any package),
// the ones with a body first.
func (d *DB) SimilarSymbols(name string, limit int) ([]string, error) {
	last := name
	if i := strings.LastIndexAny(name, "./:"); i >= 0 && i < len(name)-1 {
		last = name[i+1:]
	}
	rows, err := d.db.Query(`SELECT symbol FROM funcs WHERE lower(substr(symbol, -length(?))) = lower(?) GROUP BY symbol ORDER BY max(has_body) DESC, symbol LIMIT ?`, last, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// EnclosingFunc is the function that starts last at or before a line of a file
// (the end of a function is not stored, so this is where a line most likely is).
func (d *DB) EnclosingFunc(file string, line int) (*Func, error) {
	fs, err := d.funcs(`WHERE fl.path = ? AND f.line <= ? AND f.has_body = 1 ORDER BY f.line DESC, f.col DESC LIMIT 1`, file, line)
	if err != nil || len(fs) == 0 {
		return nil, err
	}
	return &fs[0], nil
}

// ResolveFile finds the name the file has in the graph from a name a person
// gives: the same name, an absolute path inside the analysed directory, or the
// end of a path (when only one file has it). Candidates are the files that
// have the same base name, when nothing matched.
func (d *DB) ResolveFile(name string) (path string, candidates []string, err error) {
	has := func(p string) bool {
		var n int
		d.db.QueryRow(`SELECT count(*) FROM files WHERE path = ?`, p).Scan(&n)
		return n > 0
	}
	try := []string{name, filepath.ToSlash(filepath.Clean(name))}
	if filepath.IsAbs(name) {
		for _, root := range []string{d.meta["dir"], evalLink(d.meta["dir"])} {
			if root == "" {
				continue
			}
			if rel, err := filepath.Rel(root, name); err == nil && !strings.HasPrefix(rel, "..") {
				try = append(try, filepath.ToSlash(rel))
			}
		}
		if real := evalLink(name); real != name {
			try = append(try, real)
			if root := evalLink(d.meta["dir"]); root != "" {
				if rel, err := filepath.Rel(root, real); err == nil && !strings.HasPrefix(rel, "..") {
					try = append(try, filepath.ToSlash(rel))
				}
			}
		}
	}
	for _, p := range try {
		if has(p) {
			return p, nil, nil
		}
	}
	// a path that is the end of the file's (src/a.go for pkg/src/a.go): one file only
	trimmed := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(name)), "./")
	if trimmed != "" && !filepath.IsAbs(name) {
		rows, err := d.db.Query(`SELECT path FROM files WHERE substr(path, -length(?)-1) = '/' || ? LIMIT 6`, trimmed, trimmed)
		if err != nil {
			return "", nil, err
		}
		var got []string
		for rows.Next() {
			var p string
			rows.Scan(&p)
			got = append(got, p)
		}
		rows.Close()
		if len(got) == 1 {
			return got[0], nil, nil
		}
		if len(got) > 1 {
			return "", got, nil
		}
	}
	base := filepath.Base(name)
	rows, err := d.db.Query(`SELECT path FROM files WHERE path = ? OR substr(path, -length(?)-1) = '/' || ? LIMIT 6`, base, base, base)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		rows.Scan(&p)
		candidates = append(candidates, p)
	}
	return "", candidates, rows.Err()
}

func evalLink(p string) string {
	if p == "" {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// SiteByID reads a site; ok is false when the file has none with the ID.
func (d *DB) SiteByID(id string) (s Site, ok bool, err error) {
	rows, err := d.db.Query(`SELECT s.id, s.role, s.kind, coalesce(f.fid,''), s.symbol, coalesce(s.category,''), coalesce(s.labels,''),
		coalesce(fl.path,''), s.line, s.col, s.offset, coalesce(s.callsite_id,0), coalesce(s.callee,'')
		FROM sites s LEFT JOIN funcs f ON f.id = s.func_id LEFT JOIN files fl ON fl.id = s.file_id WHERE s.id = ?`, id)
	if err != nil {
		return s, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return s, false, rows.Err()
	}
	err = rows.Scan(&s.ID, &s.Role, &s.Kind, &s.Func, &s.Symbol, &s.Category, &s.Labels, &s.File, &s.Line, &s.Col, &s.Offset, &s.CallSite, &s.Callee)
	return s, err == nil, err
}

// SiteIDsOfCall lists the IDs of the sites on a call site.
func (d *DB) SiteIDsOfCall(callsite int64) ([]string, error) {
	rows, err := d.db.Query(`SELECT id FROM sites WHERE callsite_id = ? ORDER BY id`, callsite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// FuncRowByFID finds a function row by its ID.
func (d *DB) FuncRowByFID(fid string) (*Func, error) {
	fs, err := d.funcs(`WHERE f.fid = ?`, fid)
	if err != nil || len(fs) == 0 {
		return nil, err
	}
	return &fs[0], nil
}

// Meta is a value of the meta table.
func (d *DB) Meta(key string) string { return d.meta[key] }

// MethodRow is an interface method as the call sites name it (callsites.callee
// of a call through an interface), with how many calls there are.
type MethodRow struct {
	Callee string
	// Sites is the number of call sites through an interface that name it,
	// Zero those of them with no candidate, Go those in a Go function.
	Sites, Zero, Go int
}

// InterfaceMethod reads the call sites through an interface that name callee.
// Sites is 0 when none does.
func (d *DB) InterfaceMethod(callee string) (MethodRow, error) {
	m := MethodRow{Callee: callee}
	err := d.db.QueryRow(`SELECT count(*), coalesce(sum(c.ncand = 0), 0), coalesce(sum(f.lang = 'go'), 0) FROM callsites c JOIN funcs f ON f.id = c.func_id WHERE c.kind = 'interface' AND c.callee = ?`, callee).Scan(&m.Sites, &m.Zero, &m.Go)
	return m, err
}

// InterfaceMethods reads every interface method that a call site names.
func (d *DB) InterfaceMethods() ([]MethodRow, error) {
	rows, err := d.db.Query(`SELECT c.callee, count(*), sum(c.ncand = 0), sum(f.lang = 'go') FROM callsites c JOIN funcs f ON f.id = c.func_id WHERE c.kind = 'interface' AND c.callee IS NOT NULL GROUP BY c.callee`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MethodRow
	for rows.Next() {
		var m MethodRow
		if err := rows.Scan(&m.Callee, &m.Sites, &m.Zero, &m.Go); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InterfaceEdges are the edges of the call sites through an interface that name
// callee (File, Line and Col are not read).
func (d *DB) InterfaceEdges(callee string) ([]Edge, error) {
	rows, err := d.db.Query(`SELECT e.callsite_id, e.from_func, e.to_func, e.kind, e.ncand FROM callsites c JOIN edges e ON e.callsite_id = c.id WHERE c.kind = 'interface' AND c.callee = ?`, callee)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.CallSite, &e.From, &e.To, &e.Kind, &e.NCand); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DynEdge is a dynamic edge, with the name its call site gives to the callee.
type DynEdge struct {
	Edge
	Callee string
}

// DynamicInEdges are the calls through an interface or a function value that
// may reach a function.
func (d *DB) DynamicInEdges(to int64) ([]DynEdge, error) {
	rows, err := d.db.Query(`SELECT `+edgeCols+`, coalesce(c.callee,'') `+edgeJoin+` WHERE e.to_func = ? AND e.kind IN ('interface','value') ORDER BY fl.path, c.line, c.col, e.from_func`, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DynEdge
	for rows.Next() {
		var e DynEdge
		if err := rows.Scan(&e.CallSite, &e.From, &e.To, &e.Kind, &e.NCand, &e.File, &e.Line, &e.Col, &e.Callee); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MaxReach is the most functions ReachSet reads.
const MaxReach = 200000

const inChunk = 400

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func idArgs(ids []int64) []any {
	a := make([]any, len(ids))
	for i, id := range ids {
		a[i] = id
	}
	return a
}

// ReachSet is the set of functions that the seeds reach over the edges of the file (forward: the callees; not
// forward: the callers), and over the references (a function named as a value: owner to target; backwards,
// target to owner) when refs is true. The seeds are in it. At most max functions are read: cut says it stopped.
func (d *DB) ReachSet(seeds []int64, forward, refs bool, max int) (ids []int64, cut bool, err error) {
	seen := map[int64]bool{}
	var frontier []int64
	for _, s := range seeds {
		if !seen[s] {
			seen[s] = true
			frontier = append(frontier, s)
			ids = append(ids, s)
		}
	}
	from, to, rfrom, rto := "from_func", "to_func", "owner_func", "target_func"
	if !forward {
		from, to, rfrom, rto = to, from, rto, rfrom
	}
	queries := []string{`SELECT DISTINCT ` + to + ` FROM edges WHERE ` + from + ` IN (%s)`}
	if refs {
		queries = append(queries, `SELECT DISTINCT `+rto+` FROM refs WHERE `+rfrom+` IN (%s)`)
	}
	for len(frontier) > 0 {
		var next []int64
		for i := 0; i < len(frontier); i += inChunk {
			chunk := frontier[i:min(i+inChunk, len(frontier))]
			for _, q := range queries {
				rows, err := d.db.Query(fmt.Sprintf(q, placeholders(len(chunk))), idArgs(chunk)...)
				if err != nil {
					return nil, false, err
				}
				for rows.Next() {
					var id int64
					if err := rows.Scan(&id); err != nil {
						rows.Close()
						return nil, false, err
					}
					if !seen[id] {
						seen[id] = true
						next = append(next, id)
						ids = append(ids, id)
					}
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return nil, false, err
				}
				rows.Close()
			}
			if len(ids) > max {
				return ids, true, nil
			}
		}
		frontier = next
	}
	return ids, false, nil
}

// UnresolvedCallsIn counts the calls, in the functions given, through an interface or a function value that have
// no candidate (nothing says where they go), and the calls on a dynamic value (the Dart extractor writes them as a
// call of dart:core#dynamic.invoke, which the graph holds as a builtin with no edge), and returns the first max of them.
func (d *DB) UnresolvedCallsIn(ids []int64, max int) (int, []CallRow, error) {
	total := 0
	var out []CallRow
	for i := 0; i < len(ids); i += inChunk {
		chunk := ids[i:min(i+inChunk, len(ids))]
		rows, err := d.callRows(`SELECT `+callCols+` `+callJoin+` WHERE c.func_id IN (`+placeholders(len(chunk))+`) AND ((c.kind IN ('interface','value') AND c.ncand = 0) OR (c.kind = 'builtin' AND c.callee LIKE '%#dynamic.invoke')) ORDER BY c.func_id, c.line, c.col, c.id`, idArgs(chunk)...)
		if err != nil {
			return 0, nil, err
		}
		total += len(rows)
		for _, r := range rows {
			if len(out) < max {
				out = append(out, r)
			}
		}
	}
	return total, out, nil
}

// FuncsIn reads the functions of the ids (the order is not the order given).
func (d *DB) FuncsIn(ids []int64) ([]Func, error) {
	var out []Func
	for i := 0; i < len(ids); i += inChunk {
		chunk := ids[i:min(i+inChunk, len(ids))]
		fs, err := d.funcs(`WHERE f.id IN (`+placeholders(len(chunk))+`)`, idArgs(chunk)...)
		if err != nil {
			return nil, err
		}
		out = append(out, fs...)
	}
	return out, nil
}

// NamedAsValue is which of the functions some function names as a value (a reference: an argument, a stored
// function, a closure made there).
func (d *DB) NamedAsValue(ids []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	for i := 0; i < len(ids); i += inChunk {
		chunk := ids[i:min(i+inChunk, len(ids))]
		rows, err := d.db.Query(`SELECT DISTINCT target_func FROM refs WHERE target_func IN (`+placeholders(len(chunk))+`)`, idArgs(chunk)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}
