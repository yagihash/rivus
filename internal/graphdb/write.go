package graphdb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // the driver: pure Go, no cgo

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
)

// FileRec is a file: the ones the load read have a hash of the content.
type FileRec struct {
	// Path is the name the positions use: relative to the analysed directory
	// when the file is inside it, else absolute.
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
	Size   int64  `json:"size,omitempty"`
	// Kind: source, dep, stdlib, module, lock, tree, ir, rules, position.
	Kind string `json:"kind"`
}

// Layer is a part of the file and its version.
type Layer struct {
	Name      string `json:"name"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
}

// Meta is what the file says about how and when it was made.
type Meta struct {
	RivusVersion string
	CreatedAt    time.Time
	SourceHash   string
	// Dir is the analysed directory (absolute).
	Dir       string
	Languages []string
	// Flags are the analysis flags as the command line gave them.
	Flags map[string]string
	// KeyConfig is everything but the files that the source hash is made of.
	KeyConfig map[string]string
	// Manifest is the hash of the list of the files of the tree; Roots are the
	// directories it was listed from.
	Manifest string
	Roots    []string
	Revision string
}

// Input is what Write stores.
type Input struct {
	Graph *sg.Program
	Sites []*sites.Site
	// BodyHash is the hash of the normalized body of a function, by its ID.
	BodyHash map[string]string
	Files    []FileRec
	Meta     Meta
}

// Counts are the numbers of rows of the tables.
type Counts struct {
	Funcs         int `json:"functions"`
	FuncsWithBody int `json:"functions_with_body"`
	CallSites     int `json:"call_sites"`
	Edges         int `json:"edges"`
	Refs          int `json:"refs"`
	Sites         int `json:"sites"`
	Files         int `json:"files"`
}

// LangOf tells the language of a function by its ID.
func LangOf(fid string) string {
	switch {
	case strings.HasPrefix(fid, "package:"), strings.HasPrefix(fid, "file:"), strings.HasPrefix(fid, "dart:"):
		return "dart"
	case strings.Contains(fid, "::"):
		return "rust"
	}
	return "go"
}

// Write makes the file at path (a file already there is replaced): the data is
// written to a file beside it, which is moved into place when it is complete,
// so that a reader never sees half of one.
func Write(path string, in *Input) (*Counts, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	tmp.Close()
	done := false
	defer func() {
		if !done {
			os.Remove(tmpName)
		}
	}()
	c, err := write(tmpName, in)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return nil, err
	}
	done = true
	return c, nil
}

type writer struct {
	tx      *sql.Tx
	fileIDs map[string]int64
	funcIDs map[string]int64
	nFiles  int64
	nFuncs  int64

	insFile, insFunc, insCall, insEdge, insSite, insRef *sql.Stmt
}

func write(path string, in *Input) (*Counts, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=journal_mode(OFF)&_pragma=synchronous(OFF)&_pragma=locking_mode(EXCLUSIVE)&_pragma=temp_store(MEMORY)")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createTables); err != nil {
		return nil, err
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	w := &writer{tx: tx, fileIDs: map[string]int64{}, funcIDs: map[string]int64{}}
	for _, p := range []struct {
		dst **sql.Stmt
		q   string
	}{
		{&w.insFile, `INSERT INTO files(id,path,sha256,size,kind) VALUES(?,?,?,?,?)`},
		{&w.insFunc, `INSERT INTO funcs(id,fid,symbol,file_id,line,col,offset,entry,init,dep,exported,has_body,lang,body_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`},
		{&w.insCall, `INSERT INTO callsites(func_id,file_id,line,col,offset,kind,callee,ncand,is_go) VALUES(?,?,?,?,?,?,?,?,?)`},
		{&w.insEdge, `INSERT INTO edges(callsite_id,from_func,to_func,kind,ncand) VALUES(?,?,?,?,?)`},
		{&w.insRef, `INSERT INTO refs(owner_func,file_id,line,col,offset,target_func,via_callee,kind) VALUES(?,?,?,?,?,?,?,?)`},
		{&w.insSite, `INSERT INTO sites(id,role,kind,func_id,symbol,category,labels,file_id,line,col,offset,callsite_id,callee) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`},
	} {
		if *p.dst, err = tx.Prepare(p.q); err != nil {
			return nil, err
		}
	}

	for _, f := range in.Files {
		if _, err := w.file(f.Path, f.SHA256, f.Size, f.Kind); err != nil {
			return nil, err
		}
	}
	// the functions that have a body, in a fixed order
	for _, fn := range in.Graph.Funcs {
		if _, err := w.funcRow(fn.ID, fn.Symbol, fn.Pos, true, fn, in.BodyHash[fn.ID]); err != nil {
			return nil, err
		}
	}
	// the call sites, and the edges
	callAt := map[callKey]int64{}
	var counts Counts
	for _, fn := range in.Graph.Funcs {
		fid := w.funcIDs[fn.ID]
		n, err := w.refs(fn, fid)
		if err != nil {
			return nil, err
		}
		counts.Refs += n
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				if ins.Op != ir.OpCall || ins.Callee == nil {
					continue
				}
				id, ne, err := w.callSite(fn, fid, ins)
				if err != nil {
					return nil, err
				}
				counts.CallSites++
				counts.Edges += ne
				pos := ins.Pos
				if pos.File == "" {
					pos.File = fn.Pos.File
				}
				callAt[callKey{fn.ID, pos.File, pos.Offset}] = id
			}
		}
	}
	for _, s := range in.Sites {
		if err := w.site(s, callAt); err != nil {
			return nil, err
		}
	}
	counts.Sites = len(in.Sites)

	if err := writeMeta(tx, in, &counts, w); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createIndexes); err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA optimize"); err != nil {
		return nil, err
	}
	counts.Funcs = int(w.nFuncs)
	counts.Files = int(w.nFiles)
	counts.FuncsWithBody = len(in.Graph.Funcs)
	return &counts, nil
}

// refs adds the references of a function: where it names a function as a value.
// It is what sg.Program.Reachable follows (an argument of an instruction that is
// a function), and a closure made with captured values, which Reachable does not
// follow (a closure that is not called there is not reached by it). The position
// is that of the instruction that holds the reference (the call, for an argument).
func (w *writer) refs(fn *sg.Func, owner int64) (int, error) {
	type ref struct {
		target ir.FuncID
		symbol string
		pos    ir.Pos
		via    string
		kind   string
	}
	var rows []ref
	closure := map[*sg.Instr]int{}
	seen := map[ref]bool{}
	add := func(r ref) int {
		if r.pos.File == "" {
			r.pos.File = fn.Pos.File
		}
		if seen[r] {
			return -1
		}
		seen[r] = true
		rows = append(rows, r)
		return len(rows) - 1
	}
	for _, b := range fn.Blocks {
		for _, ins := range b.Instrs {
			if ins.Op == ir.OpMakeClosure && ins.IR != nil && len(ins.Args) > 0 && ins.IR.Func != "" {
				if i := add(ref{target: ins.IR.Func, pos: ins.Pos, kind: RefClosure}); i >= 0 {
					closure[ins] = i
				}
			}
			// a closure of Rust that captured values: the alloc of its environment (Reachable follows it)
			if f := fn.Prog.EnvClosures[ins]; f != nil {
				if i := add(ref{target: ir.FuncID(f.ID), symbol: f.Symbol, pos: ins.Pos, kind: RefClosure}); i >= 0 {
					closure[ins] = i
				}
			}
		}
	}
	for _, b := range fn.Blocks {
		for _, ins := range b.Instrs {
			via := ""
			call := ins.Op == ir.OpCall && ins.Callee != nil
			if call {
				via = ins.Callee.Symbol
				if via == "" {
					via = string(ins.StaticID)
				}
			}
			for _, a := range ins.Args {
				if a == nil {
					continue
				}
				k := a.FuncRef() // a function item made a function pointer (Rust convert) too
				switch {
				case k != nil && k.FuncID != "":
					sym := ""
					if k.Fn != nil {
						sym = k.Fn.Symbol
					}
					r := ref{target: k.FuncID, symbol: sym, pos: ins.Pos, kind: RefOther}
					if call {
						r.via, r.kind = via, RefArg
					}
					add(r)
				case call && a.Kind == sg.KResult && a.Instr != nil:
					if i, ok := closure[a.Instr]; ok && rows[i].via == "" {
						rows[i].via = via
					}
				}
			}
		}
	}
	for _, r := range rows {
		file, err := w.file(r.pos.File, "", 0, "position")
		if err != nil {
			return 0, err
		}
		sym := r.symbol
		if sym == "" {
			sym = string(r.target)
		}
		to, err := w.funcRow(string(r.target), sym, ir.Pos{}, false, nil, "")
		if err != nil {
			return 0, err
		}
		var via any
		if r.via != "" {
			via = r.via
		}
		if _, err := w.insRef.Exec(owner, nullInt(file), r.pos.Line, r.pos.Col, r.pos.Offset, to, via, r.kind); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

type callKey struct {
	fid, file string
	offset    int
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (w *writer) file(path, sum string, size int64, kind string) (int64, error) {
	if path == "" {
		return 0, nil
	}
	if id, ok := w.fileIDs[path]; ok {
		return id, nil
	}
	w.nFiles++
	id := w.nFiles
	var s, z any
	if sum != "" {
		s, z = sum, size
	}
	if _, err := w.insFile.Exec(id, path, s, z, kind); err != nil {
		return 0, err
	}
	w.fileIDs[path] = id
	return id, nil
}

func nullInt(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// funcRow adds a function. A callee without a body is added the first time an
// edge needs it, with what the call says of it.
func (w *writer) funcRow(fid, symbol string, pos ir.Pos, body bool, fn *sg.Func, bodyHash string) (int64, error) {
	if id, ok := w.funcIDs[fid]; ok {
		return id, nil
	}
	file, err := w.file(pos.File, "", 0, "position")
	if err != nil {
		return 0, err
	}
	w.nFuncs++
	id := w.nFuncs
	var entry, init, dep, exported bool
	if fn != nil {
		entry, init, dep, exported = fn.Entry, fn.Init, fn.Dep, fn.Exported
	}
	var bh any
	if bodyHash != "" {
		bh = bodyHash
	}
	if _, err := w.insFunc.Exec(id, fid, symbol, nullInt(file), pos.Line, pos.Col, pos.Offset,
		b2i(entry), b2i(init), b2i(dep), b2i(exported), b2i(body), LangOf(fid), bh); err != nil {
		return 0, err
	}
	w.funcIDs[fid] = id
	return id, nil
}

// callSite adds a call and the edges from it, and returns its row and the number of edges.
func (w *writer) callSite(fn *sg.Func, from int64, in *sg.Instr) (int64, int, error) {
	pos := in.Pos
	if pos.File == "" {
		pos.File = fn.Pos.File
	}
	file, err := w.file(pos.File, "", 0, "position")
	if err != nil {
		return 0, 0, err
	}
	kind := KindStatic
	switch in.Callee.Kind {
	case ir.CalleeDynamic:
		kind = KindInterface
	case ir.CalleeValue:
		kind = KindValue
	}
	type target struct{ id, symbol string }
	var targets []target
	seen := map[string]bool{}
	add := func(id ir.FuncID, symbol string) {
		if id == "" || seen[string(id)] {
			return
		}
		seen[string(id)] = true
		targets = append(targets, target{string(id), symbol})
	}
	switch in.Callee.Kind {
	case ir.CalleeStatic:
		if in.StaticID == "" {
			kind = KindBuiltin
		}
		add(in.StaticID, in.Callee.Symbol)
	default:
		add(in.StaticID, "")
		syms := map[ir.FuncID]string{}
		for _, c := range in.Callee.ValueCands {
			syms[c.ID] = c.Symbol
		}
		for _, id := range in.Candidates {
			add(id, syms[id])
		}
	}
	// the name of the method or function the call names, when no function says it
	var callee any
	if (kind != KindStatic || len(targets) == 0) && in.Callee.Symbol != "" {
		callee = in.Callee.Symbol
	}
	res, err := w.insCall.Exec(from, nullInt(file), pos.Line, pos.Col, pos.Offset, kind, callee, len(targets), b2i(in.IR != nil && in.IR.Go))
	if err != nil {
		return 0, 0, err
	}
	cs, err := res.LastInsertId()
	if err != nil {
		return 0, 0, err
	}
	for _, t := range targets {
		sym := t.symbol
		if sym == "" {
			sym = t.id
		}
		to, err := w.funcRow(t.id, sym, ir.Pos{}, false, nil, "")
		if err != nil {
			return 0, 0, err
		}
		if _, err := w.insEdge.Exec(cs, from, to, kind, len(targets)); err != nil {
			return 0, 0, err
		}
	}
	return cs, len(targets), nil
}

func (w *writer) site(s *sites.Site, callAt map[callKey]int64) error {
	file, err := w.file(s.Position.File, "", 0, "position")
	if err != nil {
		return err
	}
	fid := w.funcIDs[s.Func]
	var cs any
	if id, ok := callAt[callKey{s.Func, s.Position.File, s.Position.Offset}]; ok {
		cs = id
	}
	var labels any
	if len(s.Labels) > 0 {
		labels = strings.Join(s.Labels, ",")
	}
	var callee any
	if s.Callee != "" {
		callee = s.Callee
	}
	var cat any
	if s.Category != "" {
		cat = s.Category
	}
	_, err = w.insSite.Exec(s.ID, s.Role, s.Kind, nullInt(fid), s.Symbol, cat, labels, nullInt(file), s.Position.Line, s.Position.Column, s.Position.Offset, cs, callee)
	return err
}

func writeMeta(tx *sql.Tx, in *Input, c *Counts, w *writer) error {
	m := in.Meta
	created := m.CreatedAt.UTC().Format(time.RFC3339)
	layers := []Layer{{Name: LayerStructure, Version: LayerVersions[LayerStructure], CreatedAt: created}}
	js := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	rows := [][2]string{
		{"schema_version", fmt.Sprint(SchemaVersion)},
		{"rivus_version", m.RivusVersion},
		{"created_at", created},
		{"source_hash", m.SourceHash},
		{"dir", m.Dir},
		{"languages", js(m.Languages)},
		{"flags", js(m.Flags)},
		{"key_config", js(m.KeyConfig)},
		{"manifest", m.Manifest},
		{"roots", js(m.Roots)},
		{"revision", m.Revision},
		{"layers", js(layers)},
	}
	for _, r := range rows {
		if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES(?,?)`, r[0], r[1]); err != nil {
			return err
		}
	}
	return nil
}
