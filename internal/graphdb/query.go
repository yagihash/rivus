package graphdb

import (
	"database/sql"
	"fmt"
	"path/filepath"
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
	Lang    string
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
	return d.edges(`SELECT `+edgeCols+` `+edgeJoin+` WHERE e.from_func = ? `+edgeFilterSQL, from, s, m)
}

// InEdges are the calls that reach a function (backward).
func (d *DB) InEdges(to int64, f EdgeFilter) ([]Edge, error) {
	s, m := f.args()
	return d.edges(`SELECT `+edgeCols+` `+edgeJoin+` WHERE e.to_func = ? `+edgeFilterSQL, to, s, m)
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

// HubOut are the dynamic calls of a function with more than max candidates:
// the total, and some of them (the largest first).
func (d *DB) HubOut(fn int64, max, limit int) (int, []CallRow, error) {
	var n int
	if err := d.db.QueryRow(`SELECT count(*) FROM callsites WHERE func_id = ? AND kind IN ('interface','value') AND ncand > ?`, fn, max).Scan(&n); err != nil || n == 0 {
		return 0, nil, err
	}
	rs, err := d.callRows(`SELECT `+callCols+` `+callJoin+` WHERE c.func_id = ? AND c.kind IN ('interface','value') AND c.ncand > ? ORDER BY c.ncand DESC, c.id LIMIT ?`, fn, max, limit)
	return n, rs, err
}

// HubIn are the dynamic calls with more than max candidates that may reach a
// function: the total, and some of them.
func (d *DB) HubIn(fn int64, max, limit int) (int, []CallRow, error) {
	var n int
	if err := d.db.QueryRow(`SELECT count(*) FROM edges WHERE to_func = ? AND kind != 'static' AND ncand > ?`, fn, max).Scan(&n); err != nil || n == 0 {
		return 0, nil, err
	}
	rs, err := d.callRows(`SELECT `+callCols+` FROM edges e JOIN callsites c ON c.id = e.callsite_id LEFT JOIN files fl ON fl.id = c.file_id
		WHERE e.to_func = ? AND e.kind != 'static' AND e.ncand > ? ORDER BY e.ncand DESC, c.id LIMIT ?`, fn, max, limit)
	return n, rs, err
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
	rows, err := d.db.Query(`SELECT f.id, f.fid, f.symbol, coalesce(fl.path,''), f.line, f.col, f.has_body, f.lang FROM funcs f LEFT JOIN files fl ON fl.id = f.file_id `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Func
	for rows.Next() {
		var f Func
		var body int
		if err := rows.Scan(&f.ID, &f.FID, &f.Symbol, &f.File, &f.Line, &f.Col, &body, &f.Lang); err != nil {
			return nil, err
		}
		f.HasBody = body == 1
		out = append(out, f)
	}
	return out, rows.Err()
}

// FuncsNamed finds the functions whose ID or symbol is name. Several functions
// can have one symbol (a method and its wrapper).
func (d *DB) FuncsNamed(name string) ([]Func, error) {
	return d.funcs(`WHERE f.fid = ? OR f.symbol = ? ORDER BY f.has_body DESC, f.fid`, name, name)
}

// FuncsEndingWith finds the functions whose symbol ends with "/"+name (a
// package path left out in front).
func (d *DB) FuncsEndingWith(name string, limit int) ([]Func, error) {
	return d.funcs(`WHERE substr(f.symbol, -length(?)-1) = '/' || ? OR substr(f.fid, -length(?)-1) = '/' || ? ORDER BY f.has_body DESC, f.fid LIMIT ?`, name, name, name, name, limit)
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
