package graphdb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

// DB is a file opened for reading.
type DB struct {
	db   *sql.DB
	Path string
	meta map[string]string
	// supplemented are the call sites that -resolve-func-values gave candidates (meta MetaFuncValues).
	supplemented map[int64]bool
}

// ErrSchema is the error of a file that is not of the schema this code reads.
type ErrSchema struct {
	Path        string
	Found, Want int
}

func (e *ErrSchema) Error() string {
	return fmt.Sprintf("%s: the graph file is of schema version %d, and this rivus reads version %d: it is not migrated, make it again with \"rivus index\"", e.Path, e.Found, e.Want)
}

// Open opens the file read only. The file is not trusted more than its
// content: it is opened with the writes off, and a file with anything but our
// tables (a view, a trigger) is refused.
func Open(path string) (*DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", path)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Add("_pragma", "query_only(1)")
	q.Add("_pragma", "trusted_schema(0)")
	q.Add("_pragma", "cell_size_check(1)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	d := &DB{db: db, Path: abs}
	if err := d.init(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) init() error {
	var ver int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&ver); err != nil {
		return fmt.Errorf("%s: not a graph file: %w", d.Path, err)
	}
	if ver != SchemaVersion {
		return &ErrSchema{Path: d.Path, Found: ver, Want: SchemaVersion}
	}
	var odd int
	if err := d.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type NOT IN ('table','index')`).Scan(&odd); err != nil {
		return fmt.Errorf("%s: not a graph file: %w", d.Path, err)
	}
	if odd > 0 {
		return fmt.Errorf("%s: not a graph file made by rivus (it holds views or triggers)", d.Path)
	}
	rows, err := d.db.Query(`SELECT key, value FROM meta`)
	if err != nil {
		return fmt.Errorf("%s: not a graph file: %w", d.Path, err)
	}
	defer rows.Close()
	d.meta = map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		d.meta[k] = v
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if d.meta["schema_version"] != strconv.Itoa(SchemaVersion) {
		return &ErrSchema{Path: d.Path, Want: SchemaVersion}
	}
	if raw := d.meta[MetaFuncValues]; raw != "" {
		var ids []int64
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			return fmt.Errorf("%s: the meta %q is not valid: %w", d.Path, MetaFuncValues, err)
		}
		d.supplemented = make(map[int64]bool, len(ids))
		for _, id := range ids {
			d.supplemented[id] = true
		}
	}
	return nil
}

// Close closes the file.
func (d *DB) Close() error { return d.db.Close() }

// Info is what the file says of itself.
type Info struct {
	Path          string            `json:"path"`
	SchemaVersion int               `json:"schema_version"`
	RivusVersion  string            `json:"rivus_version"`
	CreatedAt     string            `json:"created_at"`
	SourceHash    string            `json:"source_hash"`
	Dir           string            `json:"dir"`
	Revision      string            `json:"revision,omitempty"`
	Languages     []string          `json:"languages"`
	Layers        []Layer           `json:"layers"`
	Flags         map[string]string `json:"flags"`
	Counts        Counts            `json:"counts"`
	SizeBytes     int64             `json:"size_bytes"`
}

// Info reads the meta table and counts the rows.
func (d *DB) Info() (*Info, error) {
	fi, err := os.Stat(d.Path)
	if err != nil {
		return nil, err
	}
	in := &Info{Path: d.Path, SchemaVersion: SchemaVersion, RivusVersion: d.meta["rivus_version"], CreatedAt: d.meta["created_at"],
		SourceHash: d.meta["source_hash"], Dir: d.meta["dir"], Revision: d.meta["revision"], SizeBytes: fi.Size()}
	for k, dst := range map[string]any{"languages": &in.Languages, "layers": &in.Layers, "flags": &in.Flags} {
		if err := json.Unmarshal([]byte(d.meta[k]), dst); err != nil {
			return nil, fmt.Errorf("%s: the meta %q is not valid: %w", d.Path, k, err)
		}
	}
	in.Counts, err = d.Counts()
	return in, err
}

// Counts counts the rows of the tables. Files counts the files that were read.
func (d *DB) Counts() (Counts, error) {
	var c Counts
	for _, q := range []struct {
		dst *int
		sql string
	}{
		{&c.Funcs, `SELECT count(*) FROM funcs`},
		{&c.FuncsWithBody, `SELECT count(*) FROM funcs WHERE has_body = 1`},
		{&c.CallSites, `SELECT count(*) FROM callsites`},
		{&c.Edges, `SELECT count(*) FROM edges`},
		{&c.Refs, `SELECT count(*) FROM refs`},
		{&c.Sites, `SELECT count(*) FROM sites`},
		{&c.Files, `SELECT count(*) FROM files WHERE sha256 IS NOT NULL`},
	} {
		if err := d.db.QueryRow(q.sql).Scan(q.dst); err != nil {
			return c, err
		}
	}
	return c, nil
}

// KeyParts is what the source hash was made of, as the file recorded it.
type KeyParts struct {
	Config   map[string]string
	Files    []FileRec
	Manifest string
	Roots    []string
	Hash     string
	Dir      string
}

// KeyParts reads the parts of the source hash.
func (d *DB) KeyParts() (*KeyParts, error) {
	k := &KeyParts{Manifest: d.meta["manifest"], Hash: d.meta["source_hash"], Dir: d.meta["dir"]}
	if err := json.Unmarshal([]byte(d.meta["key_config"]), &k.Config); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(d.meta["roots"]), &k.Roots); err != nil {
		return nil, err
	}
	rows, err := d.db.Query(`SELECT path, sha256, size, kind FROM files WHERE sha256 IS NOT NULL ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f FileRec
		if err := rows.Scan(&f.Path, &f.SHA256, &f.Size, &f.Kind); err != nil {
			return nil, err
		}
		k.Files = append(k.Files, f)
	}
	return k, rows.Err()
}

// CallSite is a call as the file has it.
type CallSite struct {
	ID     int64  `json:"id"`
	Func   string `json:"func"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	Offset int    `json:"offset"`
	Kind   string `json:"kind"`
	Callee string `json:"callee,omitempty"`
	// Targets are the functions the call may reach.
	Targets []Target `json:"targets"`
}

// Target is a function an edge leads to.
type Target struct {
	ID      string `json:"id"`
	Symbol  string `json:"symbol"`
	HasBody bool   `json:"has_body"`
}

// CallSitesAt finds the calls at a position: file:line:col is one call (col
// is the position of the call, as the output of "rivus sites" has it), and
// with col 0, file:line gives every call on the line.
func (d *DB) CallSitesAt(file string, line, col int) ([]CallSite, error) {
	q := `SELECT c.id, f.fid, fl.path, c.line, c.col, c.offset, c.kind, c.callee
	      FROM callsites c JOIN files fl ON fl.id = c.file_id JOIN funcs f ON f.id = c.func_id
	      WHERE fl.path = ? AND c.line = ?`
	args := []any{file, line}
	if col > 0 {
		q += ` AND c.col = ?`
		args = append(args, col)
	}
	q += ` ORDER BY c.col, c.id`
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	var out []CallSite
	for rows.Next() {
		var c CallSite
		var callee sql.NullString
		if err := rows.Scan(&c.ID, &c.Func, &c.File, &c.Line, &c.Col, &c.Offset, &c.Kind, &callee); err != nil {
			rows.Close()
			return nil, err
		}
		c.Callee = callee.String
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range out {
		if out[i].Targets, err = d.targets(out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (d *DB) targets(callsite int64) ([]Target, error) {
	rows, err := d.db.Query(`SELECT f.fid, f.symbol, f.has_body FROM edges e JOIN funcs f ON f.id = e.to_func WHERE e.callsite_id = ? ORDER BY f.fid`, callsite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Target{}
	for rows.Next() {
		var t Target
		var body int
		if err := rows.Scan(&t.ID, &t.Symbol, &body); err != nil {
			return nil, err
		}
		t.HasBody = body == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

// FuncBySymbol finds the functions that have a symbol.
func (d *DB) FuncBySymbol(symbol string) ([]Target, error) {
	rows, err := d.db.Query(`SELECT fid, symbol, has_body FROM funcs WHERE symbol = ? ORDER BY fid`, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Target
	for rows.Next() {
		var t Target
		var body int
		if err := rows.Scan(&t.ID, &t.Symbol, &body); err != nil {
			return nil, err
		}
		t.HasBody = body == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

// Site is a site as the file has it.
type Site struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	Kind     string `json:"kind"`
	Func     string `json:"func,omitempty"`
	Symbol   string `json:"symbol"`
	Category string `json:"category,omitempty"`
	Labels   string `json:"labels,omitempty"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Offset   int    `json:"offset"`
	// CallSite is the row of the call site the site is on (0 when the site is not a call).
	CallSite int64  `json:"call_site,omitempty"`
	Callee   string `json:"callee,omitempty"`
}

// Sites lists the sites, in the order of their IDs.
func (d *DB) Sites() ([]Site, error) {
	rows, err := d.db.Query(`SELECT s.id, s.role, s.kind, coalesce(f.fid,''), s.symbol, coalesce(s.category,''), coalesce(s.labels,''),
		coalesce(fl.path,''), s.line, s.col, s.offset, coalesce(s.callsite_id,0), coalesce(s.callee,'')
		FROM sites s LEFT JOIN funcs f ON f.id = s.func_id LEFT JOIN files fl ON fl.id = s.file_id ORDER BY s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Site
	for rows.Next() {
		var s Site
		if err := rows.Scan(&s.ID, &s.Role, &s.Kind, &s.Func, &s.Symbol, &s.Category, &s.Labels, &s.File, &s.Line, &s.Col, &s.Offset, &s.CallSite, &s.Callee); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Position is a place in a file.
type Position struct {
	File      string
	Line, Col int
}

// QueryForTest returns the position in the first row of a query of three columns (file, line, col).
func (d *DB) QueryForTest(q string) (Position, error) {
	var p Position
	err := d.db.QueryRow(q).Scan(&p.File, &p.Line, &p.Col)
	return p, err
}
