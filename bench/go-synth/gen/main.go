// Command gen writes a synthetic Go module whose shape (call depth, size of
// the groups of functions that call each other, interface implementations,
// struct fields, function values) is set by parameters. The same parameters
// always write the same bytes. See bench/go-synth/README.md.
package main

import (
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Params is the shape of the generated module.
type Params struct {
	Seed     uint64
	Pkgs     int   // packages (the functions are placed by level, so the imports have no cycle)
	Funcs    int   // functions besides the members of the SCCs and the interface implementations
	Depth    int   // levels of the call graph: a function calls functions of lower levels
	SCCs     []int // sizes of the groups of functions that call each other (one level each)
	Ifaces   int   // interfaces (declared in package model; reachable from every function)
	Impls    int   // implementations of each interface (in the higher packages)
	BackPct  int   // percent of implementations that call back into a function that dispatches on the interface
	Fields   int   // fields of the struct that every function takes
	Fan      int   // calls of each function
	SinkPct  int   // percent of functions that pass a value to the sink
	SrcPct   int   // percent of functions that read the source
	SanPct   int   // percent of the sinks that sanitize first
	Handlers int   // function values registered in a table and called through it
	SvcPct   int   // percent of functions that call through an interface
	WithPct  int   // percent of calls made inside a closure given to a shared helper (model.With, as db.WithTx): the helper calls every closure, so it joins the functions that use it into one SCC
	DeepPct  int   // percent of functions that use a path of four fields (r.Sub.Sub.Sub.F), which is deeper than the engine follows
}

var sizes = map[string]Params{
	// S: seconds; M: tens of seconds; L: minutes. The shape is that of Gitea (see
	// README.md): long chains, SCCs, interface implementations that call back, deep
	// paths, closures given to a shared helper. The sizes are what the engine can
	// finish: every function of the engine's own cost grows with the fields and with
	// the size of the SCCs, so these are well below Gitea's.
	"S": {Seed: 1, Pkgs: 5, Funcs: 100, Depth: 10, SCCs: []int{4, 3}, Ifaces: 2, Impls: 4, BackPct: 50, Fields: 6, Fan: 3, SinkPct: 6, SrcPct: 4, SanPct: 25, Handlers: 10, SvcPct: 5, DeepPct: 10, WithPct: 2},
	"M": {Seed: 1, Pkgs: 12, Funcs: 300, Depth: 20, SCCs: []int{10, 6, 5, 4}, Ifaces: 3, Impls: 6, BackPct: 30, Fields: 4, Fan: 3, SinkPct: 5, SrcPct: 3, SanPct: 25, Handlers: 40, SvcPct: 4, DeepPct: 5, WithPct: 0},
	"L": {Seed: 1, Pkgs: 20, Funcs: 600, Depth: 30, SCCs: []int{16, 10, 8, 6, 5, 4}, Ifaces: 5, Impls: 8, BackPct: 30, Fields: 4, Fan: 3, SinkPct: 4, SrcPct: 2, SanPct: 25, Handlers: 80, SvcPct: 4, DeepPct: 5, WithPct: 0},
}

// rng is splitmix64: the generated code must not depend on the Go version.
type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
func (r *rng) n(n int) int {
	if n <= 1 {
		return 0
	}
	return int(r.next() % uint64(n))
}
func (r *rng) pct(p int) bool { return r.n(100) < p }

type fn struct {
	id    int
	level int
	pkg   int
	scc   int // index into SCCs, or -1
	peers []int
}

type pkgFile struct {
	body    strings.Builder
	imports map[string]bool
}

type gen struct {
	p     Params
	r     *rng
	fns   []*fn
	byLvl [][]int // function ids of each level
	pkgOf func(level int) int
	files map[string]*pkgFile // path -> file
	count map[int]int         // package -> functions written (to split files)
	// dispatchers[k] is the function that calls through interface k and that
	// no other function calls: an implementation that calls it back makes a cycle
	// (BackPct), and only then.
	dispatchers []*fn
}

func (g *gen) levelPkg(l int) int {
	p := l * g.p.Pkgs / g.p.Depth
	if p >= g.p.Pkgs {
		p = g.p.Pkgs - 1
	}
	return p
}

func pkgName(p int) string { return fmt.Sprintf("p%02d", p) }

func (g *gen) file(pkg, idx int) *pkgFile {
	path := fmt.Sprintf("%s/f%03d.go", pkgName(pkg), idx/40)
	f := g.files[path]
	if f == nil {
		f = &pkgFile{imports: map[string]bool{}}
		g.files[path] = f
	}
	return f
}

func (g *gen) call(f *pkgFile, from, to *fn, args string) string {
	if to.pkg == from.pkg {
		return fmt.Sprintf("F%d(%s)", to.id, args)
	}
	f.imports[to.pkg2()] = true
	return fmt.Sprintf("%s.F%d(%s)", pkgName(to.pkg), to.id, args)
}

func (f *fn) pkg2() string { return pkgName(f.pkg) }

// pick returns a function of a lower level (preferring the one just below).
func (g *gen) pick(level int) *fn {
	for tries := 0; tries < 8; tries++ {
		l := level - 1
		if g.r.pct(40) {
			l = g.r.n(level)
		}
		for ; l >= 0; l-- {
			if len(g.byLvl[l]) > 0 {
				ids := g.byLvl[l]
				return g.fns[ids[g.r.n(len(ids))]]
			}
		}
	}
	return nil
}

// implBand is the first level above the levels of the implementations (2 to implBand-1).
func (g *gen) implBand() int { return maxi(g.p.Depth/3, 3) }

func (g *gen) field() string { return "F" + strconv.Itoa(g.r.n(g.p.Fields)) }

// body writes the statements of one function. callees are the functions it must call.
func (g *gen) emit(me *fn, callees []*fn, extra []string) {
	f := g.file(me.pkg, g.count[me.pkg])
	g.count[me.pkg]++
	var b strings.Builder
	fmt.Fprintf(&b, "func F%d(r *model.Rec, s string) string {\n", me.id)
	b.WriteString("\tv := s\n")
	usedL, usedM := false, false
	for ci, c := range callees {
		kind := g.r.n(8)
		if g.r.pct(g.p.WithPct) {
			kind = 8
		}
		switch kind {
		case 0, 1:
			fmt.Fprintf(&b, "\tv = %s\n", g.call(f, me, c, "r, v"))
		case 2:
			fa := g.field()
			fmt.Fprintf(&b, "\tr.%s = %s\n\tv += r.%s\n", fa, g.call(f, me, c, "r, v"), g.field())
		case 3:
			usedL = true
			fmt.Fprintf(&b, "\tl = append(l, %s)\n", g.call(f, me, c, "r, v"))
		case 4:
			usedM = true
			fmt.Fprintf(&b, "\tm[%q] = %s\n\tv += m[%q]\n", "k"+strconv.Itoa(g.r.n(6)), g.call(f, me, c, "r, v"), "k"+strconv.Itoa(g.r.n(6)))
		case 5:
			fmt.Fprintf(&b, "\tf%d := func(x string) string { return %s }\n\tv = f%d(v)\n", ci, g.call(f, me, c, "r, x+v"), ci)
		case 6:
			fmt.Fprintf(&b, "\tif r.Sub != nil {\n\t\tr.Sub.%s = %s\n\t}\n\tv += r.Sub.%s\n", g.field(), g.call(f, me, c, "r.Sub, v"), g.field())
		case 8:
			// a closure given to a function that calls it, as db.WithTx(ctx, func(...) error { ... }) does
			f.imports["model"] = true
			fmt.Fprintf(&b, "\tv = model.With(r, func(r *model.Rec) string { return %s })\n", g.call(f, me, c, "r, v"))
		case 7:
			fmt.Fprintf(&b, "\tif len(v) > %d {\n\t\tv = %s\n\t}\n", g.r.n(40), g.call(f, me, c, "r, r."+g.field()))
		}
	}
	if g.r.pct(g.p.DeepPct) {
		d := "r.Sub.Sub.Sub." + g.field()
		fmt.Fprintf(&b, "\tif r.Sub != nil && r.Sub.Sub != nil && r.Sub.Sub.Sub != nil {\n\t\t%s = v\n\t\tv += %s\n\t}\n", d, "r.Sub.Sub.Sub."+g.field())
	}
	for _, e := range extra {
		b.WriteString(e)
	}
	if g.p.Ifaces > 0 && me.level >= g.implBand() && g.r.pct(g.p.SvcPct) {
		k := g.r.n(g.p.Ifaces)

		fmt.Fprintf(&b, "\tv = r.Svc%d.Do%d(r, v)\n", k, k)
	}
	if g.r.pct(g.p.SrcPct) {
		f.imports["lib"] = true
		b.WriteString("\tv += lib.Source()\n")
	}
	if g.r.pct(g.p.SinkPct) {
		f.imports["lib"] = true
		if g.r.pct(g.p.SanPct) {
			b.WriteString("\tv = lib.Sanitize(v)\n")
		}
		if g.r.pct(50) {
			fmt.Fprintf(&b, "\tlib.Sink(r.%s)\n", g.field())
		} else {
			b.WriteString("\tlib.Sink(v)\n")
		}
	}
	if usedL {
		f.imports["strings"] = true
		b.WriteString("\tv += strings.Join(l, \",\")\n")
	}
	b.WriteString("\treturn v\n}\n\n")
	// declarations of l and m go first
	src := b.String()
	var decl string
	if usedL {
		decl += "\tvar l []string\n"
	}
	if usedM {
		decl += "\tm := map[string]string{}\n"
	}
	src = strings.Replace(src, "\tv := s\n", "\tv := s\n"+decl, 1)
	f.body.WriteString(src)
}

func (g *gen) run() map[string]string {
	p := g.p
	r := &rng{s: p.Seed*0x2545f4914f6cdd1d + 7}
	g.r = r
	g.files = map[string]*pkgFile{}
	g.count = map[int]int{}
	g.byLvl = make([][]int, p.Depth)
	g.dispatchers = make([]*fn, maxi(p.Ifaces, 1))
	id := 0
	add := func(level, scc int) *fn {
		f := &fn{id: id, level: level, pkg: g.levelPkg(level), scc: scc}
		id++
		g.fns = append(g.fns, f)
		g.byLvl[level] = append(g.byLvl[level], f.id)
		return f
	}
	// SCCs: each in one level (hence one package), spread over the levels
	var sccMembers [][]*fn
	for i, n := range p.SCCs {
		lvl := 1 + (i*(p.Depth-2)/maxi(len(p.SCCs), 1)+r.n(3))%(p.Depth-1)
		var m []*fn
		for k := 0; k < n; k++ {
			m = append(m, add(lvl, i))
		}
		sccMembers = append(sccMembers, m)
	}
	// plain functions: more at the low levels
	for k := 0; k < p.Funcs; k++ {
		u := float64(r.n(1000)) / 1000
		lvl := int(u * u * float64(p.Depth))
		add(lvl, -1)
	}
	// every level has a function to call
	for l := 0; l < p.Depth; l++ {
		if len(g.byLvl[l]) == 0 {
			add(l, -1)
		}
	}
	// emit in id order: callees are chosen from lower levels only
	for _, f := range g.fns {
		var callees []*fn
		var extra []string
		if f.scc >= 0 {
			m := sccMembers[f.scc]
			idx := 0
			for i, x := range m {
				if x == f {
					idx = i
				}
			}
			next := m[(idx+1)%len(m)]
			callees = append(callees, next)
			if r.pct(30) {
				callees = append(callees, m[r.n(len(m))])
			}
			if f.level > 0 && r.pct(70) {
				if c := g.pick(f.level); c != nil {
					callees = append(callees, c)
				}
			}
		} else if f.level > 0 {
			for k := 0; k < p.Fan; k++ {
				if r.pct(25) && k > 0 {
					continue
				}
				if c := g.pick(f.level); c != nil {
					callees = append(callees, c)
				}
			}
		}
		g.emit(f, callees, extra)
	}
	// the dispatchers: level 1, not called by the others
	for k := 0; k < p.Ifaces; k++ {
		d := &fn{id: id, level: 1, pkg: g.levelPkg(1), scc: -1}
		id++
		g.fns = append(g.fns, d)
		g.dispatchers[k] = d
		g.emit(d, nil, []string{fmt.Sprintf("\tv = r.Svc%d.Do%d(r, v)\n", k, k)})
	}
	// interface implementations: in the lower packages, above the dispatchers; they
	// call lower functions and, some of them, the dispatcher (a cycle across packages)
	var wire strings.Builder
	for k := 0; k < p.Ifaces; k++ {
		for j := 0; j < p.Impls; j++ {
			lvl := 2 + r.n(g.implBand()-2)
			pk := g.levelPkg(lvl)
			f := g.file(pk, g.count[pk])
			g.count[pk]++
			f.imports["model"] = true
			ty := fmt.Sprintf("Impl%d_%d", k, j)
			var b strings.Builder
			fmt.Fprintf(&b, "type %s struct{ n int }\n\nfunc (x *%s) Do%d(r *model.Rec, s string) string {\n\tv := s\n", ty, ty, k)
			for c := 0; c < p.Fan; c++ {
				if callee := g.pick(lvl); callee != nil {
					fmt.Fprintf(&b, "\tv = %s\n", g.call(f, &fn{pkg: pk}, callee, "r, v"))
				}
			}
			if r.pct(p.BackPct) {
				callee := g.dispatchers[k]
				fmt.Fprintf(&b, "\tr.Svc%d = x\n\tv = %s\n", k, g.call(f, &fn{pkg: pk}, callee, "r, v"))
			}
			b.WriteString("\treturn v\n}\n\n")
			f.body.WriteString(b.String())
			fmt.Fprintf(&wire, "\tr.Svc%d = &%s.%s{}\n\t_ = r.Svc%d.Do%d(r, in)\n", k, pkgName(pk), ty, k, k)
		}
	}
	// the app: handlers in a table, middleware, and the entry
	out := map[string]string{}
	var app strings.Builder
	app.WriteString("package main\n\nimport (\n\t\"synth/lib\"\n\t\"synth/model\"\n")
	used := map[int]bool{}
	var tbl strings.Builder
	top := len(g.byLvl) - 1
	var cands []*fn
	for l := top; l >= 0 && len(cands) < p.Handlers*2; l-- {
		for _, id := range g.byLvl[l] {
			cands = append(cands, g.fns[id])
		}
	}
	for i := 0; i < p.Handlers && i < len(cands); i++ {
		c := cands[r.n(len(cands))]
		used[c.pkg] = true
		fmt.Fprintf(&tbl, "\t\t%q: %s.F%d,\n", "h"+strconv.Itoa(i), pkgName(c.pkg), c.id)
	}
	for _, line := range strings.Split(wire.String(), "\n") {
		if i := strings.Index(line, "&"); i >= 0 {
			name := line[i+1:]
			if d := strings.Index(name, "."); d > 0 {
				pk, _ := strconv.Atoi(name[1:d])
				used[pk] = true
			}
		}
	}
	var pk []int
	for k := range used {
		pk = append(pk, k)
	}
	sort.Ints(pk)
	for _, k := range pk {
		fmt.Fprintf(&app, "\t%q\n", "synth/"+pkgName(k))
	}
	app.WriteString(")\n\n")
	app.WriteString("type handler func(*model.Rec, string) string\n\ntype middleware func(handler) handler\n\n")
	app.WriteString("func logging(next handler) handler {\n\treturn func(r *model.Rec, s string) string { return next(r, s+\"!\") }\n}\n\n")
	app.WriteString("func auth(next handler) handler {\n\treturn func(r *model.Rec, s string) string {\n\t\tif r.Sub == nil {\n\t\t\treturn \"\"\n\t\t}\n\t\treturn next(r, s)\n\t}\n}\n\n")
	app.WriteString("func chain(h handler, ms ...middleware) handler {\n\tfor i := len(ms) - 1; i >= 0; i-- {\n\t\th = ms[i](h)\n\t}\n\treturn h\n}\n\n")
	app.WriteString("var routes = map[string]handler{\n" + tbl.String() + "}\n\n")
	app.WriteString("func main() {\n\tr := &model.Rec{Sub: &model.Rec{}}\n\tin := lib.Source()\n" + wire.String())
	app.WriteString("\tfor name, h := range routes {\n\t\t_ = name\n\t\t_ = chain(h, logging, auth)(r, in)\n\t}\n}\n")
	out["app/main.go"] = app.String()

	// model: the struct with the fields and the interfaces
	var m strings.Builder
	m.WriteString("// Package model holds the struct every generated function takes.\npackage model\n\n")
	for k := 0; k < p.Ifaces; k++ {
		fmt.Fprintf(&m, "// Svc%d is an interface with %d implementations.\ntype Svc%d interface{ Do%d(r *Rec, s string) string }\n\n", k, p.Impls, k, k)
	}
	m.WriteString("type Rec struct {\n")
	for i := 0; i < p.Fields; i++ {
		fmt.Fprintf(&m, "\tF%d string\n", i)
	}
	m.WriteString("\tSub *Rec\n")
	for k := 0; k < p.Ifaces; k++ {
		fmt.Fprintf(&m, "\tSvc%d Svc%d\n", k, k)
	}
	m.WriteString("}\n\n")
	m.WriteString("\n// With calls f with r (the shape of a transaction helper).\nfunc With(r *Rec, f func(*Rec) string) string { return f(r) }\n")
	out["model/model.go"] = m.String()
	out["lib/lib.go"] = "// Package lib has the source, the sink and the sanitizer the rules name.\npackage lib\n\nfunc Source() string { return \"x\" }\n\nfunc Sink(s string) {}\n\nfunc Sanitize(s string) string { return s }\n"
	out["go.mod"] = "module synth\n\ngo 1.27\n"
	out["rules.yaml"] = "ruleset: rivus/bench-synth@1\n\nsources:\n  - symbol: synth/lib.Source\n    labels: [user_input]\n\nsinks:\n  - symbol: synth/lib.Sink\n    category: test\n\nsanitizers:\n  - symbol: synth/lib.Sanitize\n    drops: [user_input]\n"
	var paths []string
	for path := range g.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		f := g.files[path]
		pk := strings.SplitN(path, "/", 2)[0]
		var b strings.Builder
		fmt.Fprintf(&b, "package %s\n\nimport (\n", pk)
		var imps []string
		for i := range f.imports {
			switch {
			case i == "lib", i == "model":
				imps = append(imps, "\"synth/"+i+"\"")
			case i == "strings":
				imps = append(imps, "\"strings\"")
			case strings.HasSuffix(i, "_self"):
			default:
				imps = append(imps, "\"synth/"+i+"\"")
			}
		}
		imps = append(imps, "\"synth/model\"")
		sort.Strings(imps)
		for i, s := range imps {
			if i > 0 && imps[i-1] == s {
				continue
			}
			b.WriteString("\t" + s + "\n")
		}
		b.WriteString(")\n\n" + f.body.String())
		out[path] = b.String()
	}
	return out
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	size := flag.String("size", "S", "S, M or L (the other flags override it)")
	outDir := flag.String("out", "", "directory to write (made empty first if it was written by gen)")
	var p Params
	flag.Uint64Var(&p.Seed, "seed", 0, "seed")
	flag.IntVar(&p.Pkgs, "pkgs", 0, "packages")
	flag.IntVar(&p.Funcs, "funcs", 0, "functions")
	flag.IntVar(&p.Depth, "depth", 0, "levels of the call graph")
	sccs := flag.String("sccs", "", "sizes of the SCCs, comma separated")
	flag.IntVar(&p.Ifaces, "ifaces", -1, "interfaces")
	flag.IntVar(&p.Impls, "impls", -1, "implementations of each interface")
	flag.IntVar(&p.BackPct, "back", -1, "percent of implementations that call back")
	flag.IntVar(&p.Fields, "fields", 0, "fields of the struct")
	flag.IntVar(&p.Fan, "fan", 0, "calls of each function")
	flag.IntVar(&p.SinkPct, "sinks", -1, "percent of functions with a sink")
	flag.IntVar(&p.SrcPct, "sources", -1, "percent of functions with a source")
	flag.IntVar(&p.SanPct, "sanitize", -1, "percent of sinks sanitized first")
	flag.IntVar(&p.Handlers, "handlers", -1, "function values in the table")
	flag.IntVar(&p.WithPct, "with", -1, "percent of calls made inside a closure given to a shared helper")
	flag.IntVar(&p.DeepPct, "deep", -1, "percent of functions with a path of four fields")
	flag.IntVar(&p.SvcPct, "svc", -1, "percent of functions that call through an interface")
	flag.Parse()
	base, ok := sizes[*size]
	if !ok {
		fmt.Fprintln(os.Stderr, "gen: size must be S, M or L")
		os.Exit(2)
	}
	setU := func(d *uint64, v uint64) {
		if v != 0 {
			*d = v
		}
	}
	setI := func(d *int, v int, unset int) {
		if v != unset {
			*d = v
		}
	}
	setU(&base.Seed, p.Seed)
	setI(&base.Pkgs, p.Pkgs, 0)
	setI(&base.Funcs, p.Funcs, 0)
	setI(&base.Depth, p.Depth, 0)
	if *sccs != "" {
		base.SCCs = nil
		for _, s := range strings.Split(*sccs, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(s))
			if err == nil && n > 0 {
				base.SCCs = append(base.SCCs, n)
			}
		}
	}
	setI(&base.Ifaces, p.Ifaces, -1)
	setI(&base.Impls, p.Impls, -1)
	setI(&base.BackPct, p.BackPct, -1)
	setI(&base.Fields, p.Fields, 0)
	setI(&base.Fan, p.Fan, 0)
	setI(&base.SinkPct, p.SinkPct, -1)
	setI(&base.SrcPct, p.SrcPct, -1)
	setI(&base.SanPct, p.SanPct, -1)
	setI(&base.Handlers, p.Handlers, -1)
	setI(&base.SvcPct, p.SvcPct, -1)
	setI(&base.DeepPct, p.DeepPct, -1)
	setI(&base.WithPct, p.WithPct, -1)
	if *outDir == "" {
		fmt.Fprintln(os.Stderr, "gen: -out is required")
		os.Exit(2)
	}
	if base.Depth < 3 || base.Pkgs < 1 || base.Fields < 1 {
		fmt.Fprintln(os.Stderr, "gen: depth must be at least 3, pkgs and fields at least 1")
		os.Exit(2)
	}
	files, err := Generate(base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	if err := write(*outDir, files); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

// Generate returns the files (path -> content) of the module of the parameters.
func Generate(p Params) (map[string]string, error) {
	g := &gen{p: p}
	files := g.run()
	for path, src := range files {
		if strings.HasSuffix(path, ".go") {
			b, err := format.Source([]byte(src))
			if err != nil {
				return nil, fmt.Errorf("%s: %v", path, err)
			}
			files[path] = string(b)
		}
	}
	return files, nil
}

const marker = ".generated-by-gen"

func write(dir string, files map[string]string) error {
	if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	} else if ents, err := os.ReadDir(dir); err == nil && len(ents) > 0 {
		return fmt.Errorf("%s exists and was not written by gen", dir)
	}
	files[marker] = "written by bench/go-synth/gen\n"
	for path, src := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
			return err
		}
	}
	return nil
}
