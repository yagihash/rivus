// Package graphdb stores the call graph of a program in one SQLite file, and
// reads it back (docs/DESIGN-graph.md). The file is a throw-away cache: a
// derived copy that can always be made again, so its format is never migrated
// (a file of another schema version is refused, and the message says to index
// again).
//
// The tables are in layers. The structure layer (files, funcs, callsites,
// edges, sites) is made without running the taint analysis. A taint layer
// (findings, uncertainties) is added later; meta records which layers the
// file has, so that a reader does not take their absence for "no findings".
package graphdb

// SchemaVersion is the version of the tables below (PRAGMA user_version). Any
// change of the tables raises it.
//
// Version 2 added the refs table (where a function is named as a value).
const SchemaVersion = 2

// SchemaName is the name of the JSON that the graph commands print.
// MetaFuncValues is the key of the meta row that lists the call sites (rows of callsites, a JSON array) whose
// candidates the analysis of the function values gave (-resolve-func-values). A file made without the flag has no such
// row. It is in meta and not a column of callsites: the tables are the same, so the version of the schema is too.
const MetaFuncValues = "func_values_supplemented"

const SchemaName = "rivus.graph/1"

// LayerStructure is the layer of functions, call sites, edges and sites.
const LayerStructure = "structure"

// LayerVersions are the versions of the layers this code writes. A layer whose
// version differs from what the reader knows is not trusted.
var LayerVersions = map[string]int{LayerStructure: 2}

// Kinds of a call site, and of the edges from it: where the callee is known from.
const (
	KindStatic    = "static"    // the function is named in the call
	KindInterface = "interface" // a call through an interface or a virtual method: the candidates are implementations
	KindValue     = "value"     // a call of a function value
	KindBuiltin   = "builtin"   // a builtin of the language: no function
)

// Kinds of a reference (a function named as a value, not called there), and of
// the edge that stands for it in a search.
const (
	// KindRef is the kind of an edge that is a reference, in place of a call.
	KindRef = "ref"
	// RefArg: the function is an argument of a call (m.Get("/x", repo.Activity)).
	RefArg = "arg"
	// RefClosure: a closure made there (a function literal that captures values,
	// a method value).
	RefClosure = "closure"
	// RefOther: stored, returned, or put in a structure.
	RefOther = "other"
)

const createTables = `
CREATE TABLE meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
) WITHOUT ROWID;

-- the files the load read (with the hash of the content), and the files that positions name
CREATE TABLE files (
	id     INTEGER PRIMARY KEY,
	path   TEXT NOT NULL UNIQUE,
	sha256 TEXT,
	size   INTEGER,
	kind   TEXT NOT NULL
);

-- every function of the program, and the callees that have no body (has_body = 0)
CREATE TABLE funcs (
	id        INTEGER PRIMARY KEY,
	fid       TEXT NOT NULL UNIQUE,
	symbol    TEXT NOT NULL,
	file_id   INTEGER REFERENCES files(id),
	line      INTEGER NOT NULL,
	col       INTEGER NOT NULL,
	offset    INTEGER NOT NULL,
	entry     INTEGER NOT NULL,
	init      INTEGER NOT NULL,
	dep       INTEGER NOT NULL,
	exported  INTEGER NOT NULL,
	has_body  INTEGER NOT NULL,
	lang      TEXT NOT NULL,
	body_hash TEXT
);

-- a call: where it is, and how the callee is known
CREATE TABLE callsites (
	id      INTEGER PRIMARY KEY,
	func_id INTEGER NOT NULL REFERENCES funcs(id),
	file_id INTEGER REFERENCES files(id),
	line    INTEGER NOT NULL,
	col     INTEGER NOT NULL,
	offset  INTEGER NOT NULL,
	kind    TEXT NOT NULL,
	callee  TEXT,
	ncand   INTEGER NOT NULL,
	is_go   INTEGER NOT NULL
);

-- a call site and a function it may reach; ncand is the number of candidates of the call site
CREATE TABLE edges (
	callsite_id INTEGER NOT NULL REFERENCES callsites(id),
	from_func   INTEGER NOT NULL REFERENCES funcs(id),
	to_func     INTEGER NOT NULL REFERENCES funcs(id),
	kind        TEXT NOT NULL,
	ncand       INTEGER NOT NULL
);

-- a function named as a value (an argument, a stored function, a closure made there): the owner holds
-- the reference, at the position it is written; via_callee is the function the call that takes it as an
-- argument names (empty for an assignment or a return)
CREATE TABLE refs (
	id          INTEGER PRIMARY KEY,
	owner_func  INTEGER NOT NULL REFERENCES funcs(id),
	file_id     INTEGER REFERENCES files(id),
	line        INTEGER NOT NULL,
	col         INTEGER NOT NULL,
	offset      INTEGER NOT NULL,
	target_func INTEGER NOT NULL REFERENCES funcs(id),
	via_callee  TEXT,
	kind        TEXT NOT NULL
);

-- the places where a source or a sink rule matches (what "rivus sites" lists)
CREATE TABLE sites (
	id          TEXT PRIMARY KEY,
	role        TEXT NOT NULL,
	kind        TEXT NOT NULL,
	func_id     INTEGER REFERENCES funcs(id),
	symbol      TEXT NOT NULL,
	category    TEXT,
	labels      TEXT,
	file_id     INTEGER REFERENCES files(id),
	line        INTEGER NOT NULL,
	col         INTEGER NOT NULL,
	offset      INTEGER NOT NULL,
	callsite_id INTEGER REFERENCES callsites(id),
	callee      TEXT
) WITHOUT ROWID;
`

const createIndexes = `
CREATE INDEX funcs_symbol    ON funcs(symbol);
CREATE INDEX funcs_pos       ON funcs(file_id, line);
CREATE INDEX callsites_pos   ON callsites(file_id, line, col);
CREATE INDEX callsites_func  ON callsites(func_id);
CREATE INDEX edges_callsite  ON edges(callsite_id);
CREATE INDEX edges_from      ON edges(from_func);
CREATE INDEX edges_to        ON edges(to_func);
CREATE INDEX refs_target     ON refs(target_func);
CREATE INDEX refs_owner      ON refs(owner_func);
CREATE INDEX sites_pos       ON sites(file_id, line);
CREATE INDEX sites_func      ON sites(func_id);
CREATE INDEX sites_symbol    ON sites(symbol);
CREATE INDEX sites_callsite  ON sites(callsite_id);
`
