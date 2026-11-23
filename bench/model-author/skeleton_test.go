package main

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/model"
)

const synthForeign = `package opt

type Option func(int)
`

const synthPkg = `package syn

import (
	"context"
	"database/sql"
	"errors"
	"io"

	"foreign.example/opt"
)

type DB struct {
	*sql.DB
	Name   string
	hidden int
	Opt    opt.Option
}

type Queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	Run(query string) error
	hiddenMethod()
}

type Alias = DB
type Level int

const Max = 3
const Mode Level = 2

var ErrX = errors.New("x")

func Open(driver, dsn string, opts ...opt.Option) (*DB, error) { return nil, nil }
func Use(q Queryer, query string)                              {}
func Read(r io.Reader)                                         {}
func Foreign(o opt.Option)                                     {}

func (db *DB) Run(query string) error                                       { return nil }
func (db *DB) Query2(ctx context.Context, query string, args ...any) error  { return nil }
func (db *DB) Get(key string) (string, bool)                                { return "", false }
func (db *DB) Move(d Dest)                                                  {}
func (db *DB) Wrap(o opt.Option)                                            {}

type Dest struct{ Object string }

type Params []string

func (p Params) Get(k string) string { return "" }

func Unrelated(s string) string { return s }

func (db *DB) Unrelated2() {}
`

type synthLoaded struct {
	l     *loaded
	facts *Facts
}

func loadSynth(t *testing.T) synthLoaded {
	t.Helper()
	fset := token.NewFileSet()
	check := func(path, src string, imp types.Importer) *types.Package {
		f, err := parser.ParseFile(fset, path+".go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		conf := types.Config{Importer: imp}
		p, err := conf.Check(path, fset, []*ast.File{f}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	std := importer.ForCompiler(fset, "source", nil)
	foreign := check("foreign.example/opt", synthForeign, std)
	pkg := check("m.example/syn", synthPkg, importerFunc(func(p string) (*types.Package, error) {
		if p == "foreign.example/opt" {
			return foreign, nil
		}
		return std.Import(p)
	}))
	zero := 0
	facts := &Facts{
		SinkRows: []model.Sink{
			{Symbol: "(*m.example/syn.DB).Run", Args: []int{0}, Category: "sql_injection"},
			{Symbol: "(*m.example/syn.DB).Query2", Args: []int{1}, Category: "sql_injection"},
			{Symbol: "(*m.example/syn.DB).Move", Parts: []model.SinkPart{{Arg: &zero, Field: "Object"}}, Category: "resource_name"},
		},
		FlowRows: []model.Propagator{
			{Symbol: "(*m.example/syn.DB).Get", Flows: []model.Flow{{From: "recv", To: "result0"}}},
			{Symbol: "(m.example/syn.Params).Get", Flows: []model.Flow{{From: "recv", To: "result"}}},
		},
	}
	return synthLoaded{
		l:     &loaded{Types: pkg, Module: "m.example/syn", Version: "v1.0.0", ModOf: map[string]string{"m.example/syn": "m.example/syn", "foreign.example/opt": "foreign.example"}},
		facts: facts,
	}
}

type importerFunc func(string) (*types.Package, error)

func (f importerFunc) Import(p string) (*types.Package, error) { return f(p) }

// what the cells of the synthetic package use
var synthUsed = map[string]bool{"Open": true, "Alias": true, "Mode": true, "ErrX": true, "Use": true, "Foreign": true, "Wrap": true, "Opt": true}

func genSynth(t *testing.T) (*skelResult, synthLoaded) {
	t.Helper()
	s := loadSynth(t)
	res, err := GenerateSkeleton(&skelInput{Root: s.l.Types, Module: s.l.Module, Version: s.l.Version, ModOf: s.l.ModOf, Facts: s.facts, UsedNames: synthUsed})
	if err != nil {
		t.Fatal(err)
	}
	return res, s
}

func TestSkeletonShape(t *testing.T) {
	res, _ := genSynth(t)
	if len(res.Files) != 1 {
		t.Fatalf("%d files", len(res.Files))
	}
	src := res.Files[0].Content
	if !strings.HasPrefix(src, GeneratedHeader+"\n") {
		t.Errorf("the header is not the first line:\n%s", src[:80])
	}
	for _, want := range []string{
		"type DB struct {\n\t*sql.DB\n\tName string\n\tv ", // the embedding of the real package, the exported field, the marker (a flow from the receiver)
		"type Alias = DB",                                                // an alias stays an alias
		"type Level int",                                                 // the type of a constant
		"const Mode Level = 2",                                           //
		"var ErrX = errors.New(\"ErrX\")",                                // an error value is a distinct value
		"func Open(driver string, dsn string) (*DB, error) {",            // the variadic parameter of another module is dropped
		"return &DB{DB: &sql.DB{}}, nil",                                 // a pointer to a struct is not nil, the embedded pointer is set
		"func (db *DB) Run(query string) error {\n\tmark.SinkSQL(query)", // the record of a sink row
		"for _, s := range args",                                         // (not for Query2: args is not the sink position: see below)
	} {
		if want == "for _, s := range args" {
			if strings.Contains(src, want) {
				t.Errorf("a position that the row does not name is recorded:\n%s", src)
			}
			continue
		}
		if !strings.Contains(src, want) {
			t.Errorf("no %q in\n%s", want, src)
		}
	}
	if !strings.Contains(src, "mark.SinkSQL(query)") || !strings.Contains(src, "mark.SinkCmd(d.Object)") {
		t.Errorf("the records of Query2 and of the part of Move are missing:\n%s", src)
	}
	if !strings.Contains(src, "return db.v, true") {
		t.Errorf("a getter does not return the marker:\n%s", src)
	}
	// the hidden method of the interface is not written, so the skeleton interface can be satisfied from outside
	if strings.Contains(src, "hiddenMethod") || strings.Contains(src, "hidden ") {
		t.Errorf("an unexported member is written:\n%s", src)
	}
	// a function with a parameter of another module is left out, and listed; so is the field
	if strings.Contains(src, "func Foreign") || strings.Contains(src, "func (db *DB) Wrap") {
		t.Errorf("a member that needs another module is written:\n%s", src)
	}
	om := strings.Join(res.Omitted, "\n")
	for _, w := range []string{"m.example/syn.Foreign", "m.example/syn.Open: the variadic parameter opts is dropped", "m.example/syn.DB.Opt"} {
		if !strings.Contains(om, w) {
			t.Errorf("not listed as left out: %s\n%s", w, om)
		}
	}
	if len(res.RowsMissing) != 0 {
		t.Errorf("rows missing: %v", res.RowsMissing)
	}
	// only what the rows, the cells and the interfaces need: not the rest of the API
	if strings.Contains(src, "Unrelated") || strings.Contains(src, "func Read") {
		t.Errorf("a member that no row and no cell points at is written:\n%s", src)
	}
	// a flow from a slice receiver cannot be carried: a warning (not a failure)
	w := strings.Join(res.Warnings, "\n")
	if !strings.Contains(w, "(m.example/syn.Params).Get: the flow from the receiver is not reproduced") {
		t.Errorf("no warning for the slice receiver: %v", res.Warnings)
	}
}

func TestSkeletonFailsOnARowThatHasNoPlace(t *testing.T) {
	s := loadSynth(t)
	s.facts.SinkRows = append(s.facts.SinkRows,
		model.Sink{Symbol: "m.example/syn.Gone", Args: []int{0}, Category: "sql_injection"},
		model.Sink{Symbol: "(*m.example/syn.DB).Wrap", Args: []int{0}, Category: "sql_injection"}) // a parameter of another module
	res, err := GenerateSkeleton(&skelInput{Root: s.l.Types, Module: s.l.Module, Version: s.l.Version, ModOf: s.l.ModOf, Facts: s.facts, UsedNames: synthUsed})
	if err != nil || strings.Join(res.RowsMissing, ",") != "(*m.example/syn.DB).Wrap,m.example/syn.Gone" {
		t.Fatalf("%v %v", err, res)
	}
	// the command stops with exit 30 and writes nothing
	repo := fakeRepo(t)
	var out bytes.Buffer
	err = runGenSkeleton(&out, &options{pkg: "m.example/syn", work: t.TempDir()}, repo, s.facts, s.l)
	if ee, ok := err.(*exitError); !ok || ee.code != ExitSkeleton {
		t.Fatalf("%v, want exit %d", err, ExitSkeleton)
	}
	if _, err := os.Stat(filepath.Join(repo, "bench", "zoo-go-pkg", "syn", "skel")); err == nil {
		t.Error("files were written for a failed run")
	}
	if !strings.Contains(out.String(), "row without a place in the skeleton: m.example/syn.Gone") {
		t.Errorf("the rows are not listed:\n%s", out.String())
	}
}

func TestSkeletonIsDeterministic(t *testing.T) {
	a, _ := genSynth(t)
	for i := 0; i < 3; i++ {
		b, _ := genSynth(t)
		if len(a.Files) != len(b.Files) || a.Files[0].Content != b.Files[0].Content {
			t.Fatal("the output differs between runs")
		}
	}
}

// repoRoot of the tests: bench/model-author -> the checkout.
func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// fakeRepo is a checkout that has nothing but bench/zoo-go (a link to the real one): the place where the generator writes.
func fakeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bench", "zoo-go-pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repoRoot(t), "bench", "zoo-go"), filepath.Join(dir, "bench", "zoo-go")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGenSkeletonWritesBuildsAndKeepsHandFiles(t *testing.T) {
	repo := fakeRepo(t)
	s := loadSynth(t)
	o := &options{pkg: "m.example/syn", work: t.TempDir()}
	var out bytes.Buffer
	if err := runGenSkeleton(&out, o, repo, s.facts, s.l); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	dir := filepath.Join(repo, "bench", "zoo-go-pkg", "syn")
	for _, f := range []string{"go.mod", "rules.yaml", "skel/go.mod", "skel/syn.go", "cells/sql_injection/sql_injection.go", "cells/resource_name/resource_name.go"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s: %v\n%s", f, err, out.String())
		}
	}
	mod, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
	for _, w := range []string{GeneratedHeader, "module zoopkg/syn", "m.example/syn v1.0.0", "replace zoo => ../../zoo-go", "replace m.example/syn => ./skel"} {
		if !strings.Contains(string(mod), w) {
			t.Errorf("go.mod has no %q:\n%s", w, mod)
		}
	}
	cell, _ := os.ReadFile(filepath.Join(dir, "cells", "sql_injection", "sql_injection.go"))
	for _, w := range []string{"@cell(F/pkg/syn/sql-injection)", "@vuln", "@fixed", "@decoy", "mark.CleanSQL(mark.Src(", "@sink(pkg-syn-sql-injection-vuln)", "@sink(none) @nosink(pkg-syn-sql-injection-decoy)", "new(syn.DB)"} {
		if !strings.Contains(string(cell), w) {
			t.Errorf("the cell template has no %q:\n%s", w, cell)
		}
	}
	if strings.Contains(string(cell), GeneratedHeader) {
		t.Error("a cell carries the header of a generated file: it would be taken for one that the command may overwrite")
	}
	// a second run: the generated files are the same, a hand file of skel/ and the cell are not touched, a stale generated file goes
	gen1, _ := os.ReadFile(filepath.Join(dir, "skel", "syn.go"))
	hand := filepath.Join(dir, "skel", "zoo_extra.go")
	os.WriteFile(hand, []byte("package syn\n\n// hand\n"), 0o644)
	stale := filepath.Join(dir, "skel", "old.go")
	os.WriteFile(stale, []byte(GeneratedHeader+"\npackage syn\n"), 0o644)
	cellPath := filepath.Join(dir, "cells", "sql_injection", "sql_injection.go")
	os.WriteFile(cellPath, []byte(string(cell)+"\n// edited by hand\n"), 0o644)
	if err := runGenSkeleton(&out, o, repo, s.facts, s.l); err != nil {
		t.Fatalf("second run: %v", err)
	}
	gen2, _ := os.ReadFile(filepath.Join(dir, "skel", "syn.go"))
	if !bytes.Equal(gen1, gen2) {
		t.Error("the generated skeleton changed on a second run")
	}
	if b, _ := os.ReadFile(hand); !strings.Contains(string(b), "// hand") {
		t.Error("a hand file of skel/ was touched")
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("a stale generated file stays")
	}
	if b, _ := os.ReadFile(cellPath); !strings.Contains(string(b), "// edited by hand") {
		t.Error("a cell was overwritten")
	}
}

func TestGenSkeletonExitCodes(t *testing.T) {
	repo := fakeRepo(t)
	s := loadSynth(t)
	o := &options{pkg: "m.example/syn", work: t.TempDir()}
	dir := filepath.Join(repo, "bench", "zoo-go-pkg", "syn")
	// a go.mod that somebody wrote and that says something else is not touched: 30
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module zoopkg/syn\n\ngo 1.27\n\nrequire other.example/x v1.0.0\n"), 0o644)
	err := runGenSkeleton(&bytes.Buffer{}, o, repo, s.facts, s.l)
	if ee, ok := err.(*exitError); !ok || ee.code != ExitSkeleton {
		t.Errorf("hand written go.mod: %v, want exit %d", err, ExitSkeleton)
	}
	// another package of the module has the skeleton: 30
	os.Remove(filepath.Join(dir, "go.mod"))
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(GeneratedHeader+"\n// Package: m.example/syn/other\nmodule zoopkg/syn\n"), 0o644)
	err = runGenSkeleton(&bytes.Buffer{}, o, repo, s.facts, s.l)
	if ee, ok := err.(*exitError); !ok || ee.code != ExitSkeleton || !strings.Contains(ee.Error(), "one package of a module") {
		t.Errorf("another package: %v, want exit %d", err, ExitSkeleton)
	}
	// the cells cannot be written: 40
	repo2 := fakeRepo(t)
	dir2 := filepath.Join(repo2, "bench", "zoo-go-pkg", "syn")
	os.MkdirAll(dir2, 0o755)
	os.WriteFile(filepath.Join(dir2, "cells"), []byte("a file"), 0o644)
	err = runGenSkeleton(&bytes.Buffer{}, o, repo2, s.facts, s.l)
	if ee, ok := err.(*exitError); !ok || ee.code != ExitCells {
		t.Errorf("cells that cannot be written: %v, want exit %d", err, ExitCells)
	}
}

func TestZooPkgDirName(t *testing.T) {
	for _, c := range []struct{ pkg, mod, want string }{
		{"github.com/jmoiron/sqlx", "github.com/jmoiron/sqlx", "sqlx"},
		{"cloud.google.com/go/storage", "cloud.google.com/go/storage", "storage"},
		{"github.com/google/go-github/v93/github", "github.com/google/go-github/v93", "go_github"},
		{"github.com/gin-gonic/gin", "github.com/gin-gonic/gin", "gin"},
		{"database/sql", "", "database_sql"},
		{"os", "", "os"},
	} {
		if got := goZooPkgDirName(c.pkg, c.mod); got != c.want {
			t.Errorf("%s: %s, want %s", c.pkg, got, c.want)
		}
	}
}

// The standard library has no skeleton: the module files and the cell template only, in bench/zoo-go-pkg/<name with _>.
func TestGenSkeletonStdlib(t *testing.T) {
	repo := fakeRepo(t)
	l, err := FetchAndLoad(t.TempDir(), "html", "")
	if err != nil {
		t.Fatal(err)
	}
	facts := &Facts{SinkRows: []model.Sink{{Symbol: "html.UnescapeString", Args: []int{0}, Category: "xss"}}}
	var out bytes.Buffer
	if err := runGenSkeleton(&out, &options{pkg: "html", work: t.TempDir()}, repo, facts, l); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	dir := filepath.Join(repo, "bench", "zoo-go-pkg", "html")
	if _, err := os.Stat(filepath.Join(dir, "skel")); err == nil {
		t.Error("the standard library has a skeleton")
	}
	mod, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
	if strings.Contains(string(mod), "replace html") || !strings.Contains(string(mod), "replace zoo") {
		t.Errorf("go.mod:\n%s", mod)
	}
	cell, err := os.ReadFile(filepath.Join(dir, "cells", "xss", "xss.go"))
	if err != nil || !strings.Contains(string(cell), "html.UnescapeString(q)") || !strings.Contains(string(cell), "mark.CleanHTML(") {
		t.Errorf("%v\n%s", err, cell)
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go")
	}
}
