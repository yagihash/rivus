// Command gen writes a synthetic Go module that reproduces, at a small size, the shapes that made
// the real projects (Gitea, gogs) big: a confluence of data through a cache of types, through
// os.Stderr, a widen that goes back and forth, a huge group of functions that call each other, and
// more. Each shape is switched on alone (-shapes), so that a profile has one cause only.
// The code carries its own answer key in the comments of bench/go-curated (@source / @sink /
// @sink(none) / @nosink). The same parameters always write the same bytes. See bench/README.md.
package main

import (
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Params is the shape of the generated module.
type Params struct {
	Seed    uint64
	Funcs   int             // ordinary functions (the ballast: a call graph without sinks)
	Pkgs    int             // packages of the ballast
	KeepPct int             // percent of the calls whose result is used (the others only call)
	SrcPct  int             // percent of the ballast with a source (it flows up, to no sink)
	EncPct  int             // percent of the ballast that hands its source to a cache of types / to os.Stderr
	ReadPct int             // percent of the ballast that reads the cache / a logger
	Loggers int             // packages with a logger made of os.Stderr
	Ring    int             // functions that call each other (shape scc)
	Wide    int             // functions of the ring of wide structs (shape widen)
	Chain   int             // globals in a chain (shape chain)
	Impls   int             // implementations of the interface (shape dyn)
	Callers int             // functions that call through the interface (shape dyn)
	Giant   int             // statements of the one huge function (shape giant)
	Copies  int             // copies of the "save and restore a global struct" shape (shape selfcopy)
	Shapes  map[string]bool // types stderr widen scc chain dyn glob giant selfcopy
}

// allShapes is the list of the shapes, in the order they are written.
var allShapes = []string{"types", "stderr", "widen", "scc", "chain", "dyn", "glob", "giant", "selfcopy"}

// Default is the size of the profiles (about 2,000 functions).
func Default() Params {
	return Params{Seed: 1, Funcs: 2000, Pkgs: 20, KeepPct: 15, SrcPct: 4, EncPct: 4, ReadPct: 25, Loggers: 8, Ring: 200, Wide: 16, Chain: 10, Impls: 150, Callers: 40, Giant: 3000, Copies: 40, Shapes: map[string]bool{}}
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

const fields = 32 // fields of model.Rec: more than maxOutElems (24) of widen

func pk(i int) string { return fmt.Sprintf("p%02d", i) }

// file is a Go file whose package clause and imports are written from its pkg and imports; a raw
// file has them in its body already.
type file struct {
	imports map[string]bool
	body    strings.Builder
	raw     bool
}

type gen struct {
	p     Params
	r     rng
	files map[string]*file
}

func (g *gen) file(path string) *file {
	f := g.files[path]
	if f == nil {
		f = &file{imports: map[string]bool{}}
		g.files[path] = f
	}
	return f
}

func (g *gen) rawFile(path, src string) {
	f := g.file(path)
	f.raw = true
	f.body.WriteString(src)
}

func (g *gen) on(s string) bool { return g.p.Shapes[s] }

func (g *gen) ballast() {
	n, np := g.p.Funcs, g.p.Pkgs
	pkgOf := func(i int) int { return i * np / n }
	for i := 0; i < n; i++ {
		pi := pkgOf(i)
		f := g.file(pk(pi) + "/f.go")
		f.imports["model"] = true
		var b strings.Builder
		fmt.Fprintf(&b, "func F%d(r *model.Rec, s string) string {\n\tv := s\n", i)
		if i > 0 {
			for k := 0; k < 3; k++ {
				w := 40
				if i < w {
					w = i
				}
				j := i - 1 - g.r.n(w)
				call := fmt.Sprintf("F%d(r, v)", j)
				if pkgOf(j) != pi {
					f.imports[pk(pkgOf(j))] = true
					call = fmt.Sprintf("%s.F%d(r, v)", pk(pkgOf(j)), j)
				}
				if g.r.pct(g.p.KeepPct) {
					fmt.Fprintf(&b, "\tv += %s\n", call) // the result is used: what the callee has goes up
				} else {
					fmt.Fprintf(&b, "\t_ = %s\n", call)
				}
			}
		}
		fmt.Fprintf(&b, "\tv += r.F%d\n", g.r.n(4))
		if g.r.pct(g.p.SrcPct) {
			f.imports["lib"] = true
			b.WriteString("\tx := lib.Source()\n\tv += x\n")
			if g.r.pct(g.p.EncPct * 100 / maxi(g.p.SrcPct, 1)) {
				if g.on("types") {
					f.imports["tcache"] = true
					if i%2 == 0 {
						b.WriteString("\ttcache.Encode(x)\n")
					} else {
						b.WriteString("\ttcache.EncodeV(x)\n")
					}
				}
				if g.on("stderr") {
					f.imports["logx"] = true
					b.WriteString("\tlogx.Warn(x)\n")
				}
			}
		}
		if g.r.pct(g.p.ReadPct) {
			if g.on("types") {
				f.imports["tcache"] = true
				b.WriteString("\tv += tcache.Describe(v)\n")
			}
			if g.on("stderr") {
				l := fmt.Sprintf("lg%02d", g.r.n(g.p.Loggers))
				f.imports[l] = true
				fmt.Fprintf(&b, "\tv += %s.Report(v)\n", l)
			}
		}
		b.WriteString("\treturn v\n}\n\n")
		f.body.WriteString(b.String())
	}
}

func (g *gen) shapeTypes() {
	g.rawFile("tcache/tcache.go", `// Package tcache is a cache of what is known about a type, as protobuf has (marshalInfoMap).
package tcache

import (
	"fmt"
	"reflect"
)

type Info struct {
	T reflect.Type
	N int
}

var infoMap = map[reflect.Type]*Info{}

// Encode: the type of the value is the key and the content of the cache.
func Encode(v interface{}) *Info {
	t := reflect.TypeOf(v)
	i := infoMap[t]
	if i == nil {
		i = &Info{T: t}
		infoMap[t] = i
	}
	return i
}

// EncodeV is the same through (reflect.Value).Type.
func EncodeV(v interface{}) *Info {
	t := reflect.ValueOf(v).Type()
	i := infoMap[t]
	if i == nil {
		i = &Info{T: t}
		infoMap[t] = i
	}
	return i
}

// Describe reads the cache: every function that uses it reads what every other function put in.
func Describe(v interface{}) string {
	return fmt.Sprint(infoMap[reflect.TypeOf(v)])
}
`)
	t := g.file("truth/types.go")
	t.imports["lib"], t.imports["tcache"], t.imports["reflect"], t.imports["fmt"] = true, true, true, true
	t.body.WriteString(`// TyReal: the data of a value goes on to the sink, and next to it the value is given to the cache of types:
// both flows stay (the second is the one through (reflect.Value).Interface, which carries the data).
func TyReal() {
	x := lib.Source() // @source(ty_real)
	tcache.Encode(x)
	lib.Sink(x) // @sink(ty_real)
	y := reflect.ValueOf(x).Interface()
	lib.Sink(fmt.Sprint(y)) // @sink(ty_real)
}

// TyQuiet reads the cache only: no source reaches it, but for the cache that joins every writer to every reader.
func TyQuiet(s string) {
	lib.Sink(tcache.Describe(s)) // @sink(none)
}

`)
}

func (g *gen) shapeStderr() {
	g.rawFile("logx/logx.go", `// Package logx writes to the standard error.
package logx

import (
	"fmt"
	"os"
)

var last string

// Warn writes the value to the standard error: nobody reads it back.
func Warn(s string) { fmt.Fprintf(os.Stderr, "warn: %s\n", s) }

// WarnReal writes the value to the standard error and keeps it in a variable that is not the standard error.
func WarnReal(s string) {
	fmt.Fprintf(os.Stderr, "warn: %s\n", s)
	last = s
}

// Last is what WarnReal kept.
func Last() string { return last }
`)
	for i := 0; i < g.p.Loggers; i++ {
		l := fmt.Sprintf("lg%02d", i)
		g.rawFile(l+"/lg.go", fmt.Sprintf(`// Package %s has a logger made of the standard error, as every library has.
package %s

import (
	"fmt"
	"log"
	"os"
)

var logger = log.New(os.Stderr, "%s: ", 0)

// Report logs the value and says what the logger is.
func Report(s string) string {
	logger.Printf("%%s", s)
	return fmt.Sprint(logger)
}
`, l, l, l))
	}
	t := g.file("truth/stderr.go")
	t.imports["lib"], t.imports["logx"], t.imports["lg00"] = true, true, true
	t.body.WriteString(`// SrReal: a value written to the standard error and kept in another variable is read back from that variable.
func SrReal() {
	x := lib.Source() // @source(sr_real)
	logx.WarnReal(x)
	lib.Sink(logx.Last()) // @sink(sr_real)
}

// SrQuiet reads a logger: what was written to the standard error is not in it.
func SrQuiet() {
	lib.Sink(lg00.Report("x")) // @sink(none)
}

`)
}

func (g *gen) shapeWide() {
	f := g.file("wide/wide.go")
	f.imports["model"] = true
	f.imports["strings"] = true
	m := g.p.Wide
	perms := [][3]string{{"a", "b", "c"}, {"b", "c", "a"}, {"c", "a", "b"}, {"b", "a", "c"}}
	fmt.Fprintf(&f.body, "var wg [%d]string\n\n", m)
	for k := 0; k < m; k++ {
		var fs []string
		for _, p := range []string{"a", "b", "c"} {
			for j, c := 0, 5+g.r.n(8); j < c; j++ {
				fs = append(fs, fmt.Sprintf("%s.F%d", p, g.r.n(fields)))
			}
		}
		fmt.Fprintf(&f.body, "func W%d(a, b, c *model.Rec, s string) string {\n\tout := strings.Join([]string{%s}, \",\")\n\tout += wg[%d]\n", k, strings.Join(fs, ", "), g.r.n(m))
		for _, to := range []int{(k + 1) % m, g.r.n(m)} {
			pm := perms[g.r.n(len(perms))]
			fmt.Fprintf(&f.body, "\tif len(s) > %d {\n\t\tout += W%d(%s, %s, %s, s[1:])\n\t}\n", g.r.n(5), to, pm[0], pm[1], pm[2])
		}
		fmt.Fprintf(&f.body, "\twg[%d] = out\n\treturn out\n}\n\n", k)
	}
	t := g.file("truth/wide.go")
	t.imports["lib"], t.imports["model"], t.imports["wide"] = true, true, true
	t.body.WriteString(`// WdReal: a field of the struct is read by the ring (the output of its functions is widened to the whole struct).
func WdReal() {
	r, q, u := &model.Rec{}, &model.Rec{}, &model.Rec{}
	r.F24 = lib.Source() // @source(wd_real)
	lib.Sink(wide.W0(r, q, u, "abcdef")) // @sink(wd_real)
}

`)
}

func (g *gen) shapeSCC() {
	f := g.file("ring/ring.go")
	f.imports["model"], f.imports["lib"] = true, true
	n := g.p.Ring
	fmt.Fprintf(&f.body, "var ringLog [%d]string\n\n", n)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&f.body, "func R%d(r *model.Rec, s string) string {\n\tv := s + r.F%d\n\tif len(v) > %d {\n\t\tv += R%d(r, v)\n\t}\n\tif len(s) > %d {\n\t\tv += R%d(r, s)\n\t}\n", i, i%4, i, (i+1)%n, i+1, (i*7+3)%n)
		if i%5 == 0 {
			fmt.Fprintf(&f.body, "\tringLog[%d] = lib.Source()\n", i) // every member of the ring writes what the others write (and nobody reads it)
		}
		if i == n/2 {
			f.body.WriteString("\tif len(v) == 5 {\n\t\tlib.Sink(v) // @sink(scc_real)\n\t}\n")
		}
		f.body.WriteString("\treturn v\n}\n\n")
	}
	t := g.file("truth/scc.go")
	t.imports["lib"], t.imports["model"], t.imports["ring"] = true, true, true
	t.body.WriteString(`// SccReal: a value goes into the big ring and reaches a sink in one of its functions.
func SccReal() {
	x := lib.Source() // @source(scc_real)
	ring.R0(&model.Rec{}, x)
}

`)
}

func (g *gen) shapeChain() {
	f := g.file("chain/chain.go")
	f.imports["lib"] = true
	n := g.p.Chain
	// the steps are written in the order that is against the order of analysis: the last first
	for i := n - 1; i >= 1; i-- {
		fmt.Fprintf(&f.body, "var G%d string\n\nfunc S%d() { G%d = G%d }\n\n", i, i, i, i-1)
	}
	fmt.Fprintf(&f.body, "var G0 string\n\nfunc S0() { G0 = lib.Source() } // @source(ch_real)\n\nfunc Out() {\n\tlib.Sink(G%d) // @sink(ch_real)\n}\n\n", n-1)
}

func (g *gen) shapeDyn() {
	f := g.file("dyn/dyn.go")
	f.imports["model"] = true
	f.body.WriteString("type Doer interface{ Do(r *model.Rec, s string) string }\n\n")
	for i := 0; i < g.p.Impls; i++ {
		ret := "s + \"x\""
		if i == 7 {
			ret = "s"
		}
		fmt.Fprintf(&f.body, "type I%d struct{ n int }\n\nfunc (x *I%d) Do(r *model.Rec, s string) string { return %s }\n\n", i, i, ret)
	}
	f.body.WriteString("type Const struct{}\n\nfunc (Const) Do(r *model.Rec, s string) string { return \"c\" }\n\n")
	for k := 0; k < g.p.Callers; k++ {
		nxt := ""
		if k+1 < g.p.Callers {
			nxt = fmt.Sprintf("\t\tv += D%d(d, r, v)\n", k+1)
		}
		fmt.Fprintf(&f.body, "func D%d(d Doer, r *model.Rec, s string) string {\n\tv := s\n\tif len(s) > %d {\n\t\tv = d.Do(r, v)\n\t}\n\tif len(r.F0) < %d {\n%s\t}\n\treturn v\n}\n\n", k, k, k+2, nxt)
	}
	t := g.file("truth/dyn.go")
	t.imports["lib"], t.imports["model"], t.imports["dyn"] = true, true, true
	t.body.WriteString(`// DynReal: a value goes through a call on an interface that has many implementations, one of which returns it.
func DynReal() {
	x := lib.Source() // @source(dyn_real)
	var d dyn.Doer = &dyn.I7{}
	lib.Sink(dyn.D0(d, &model.Rec{}, x)) // @sink(dyn_real)
}

// DynQuiet calls the implementation that returns a constant: the other implementations are not its candidates.
func DynQuiet() {
	x := lib.Source() // @source(dyn_quiet)
	var d dyn.Doer = dyn.Const{}
	lib.Sink(d.Do(&model.Rec{}, "k")) // @nosink(dyn_quiet)
	lib.Sink(x)                       // @sink(dyn_quiet)
}

`)
}

func (g *gen) shapeGlob() {
	f := g.file("glob/glob.go")
	f.imports["lib"] = true
	f.body.WriteString(`// A handler writes a package variable and another entry point reads it (the write is explicit).
var Saved string

var State struct{ User, Other string }

func HandlerA() {
	Saved = lib.Source() // @source(gl_real)
	State.User = Saved
}

func HandlerB() {
	lib.Sink(Saved)      // @sink(gl_real)
	lib.Sink(State.User) // @sink(gl_real)
}

func HandlerC() {
	lib.Sink(State.Other) // @sink(none)
}

`)
}

func (g *gen) shapeGiant() {
	f := g.file("giant/giant.go")
	f.imports["model"], f.imports["lib"], f.imports["strings"] = true, true, true
	var b strings.Builder
	b.WriteString("// Giant is one function of very many statements (the work limit stops it, not a clock).\nfunc Giant(r *model.Rec, s string) string {\n\tv := s\n\tx := lib.Source() // @source(gi_real)\n\tw := r.F0\n")
	for i := 0; i < g.p.Giant; i++ {
		switch g.r.n(4) {
		case 0:
			fmt.Fprintf(&b, "\tv += r.F%d\n", g.r.n(4))
		case 1:
			b.WriteString("\tw = strings.ToUpper(v + w)\n")
		case 2:
			fmt.Fprintf(&b, "\tif len(v) > %d {\n\t\tv += w\n\t}\n", i)
		default:
			b.WriteString("\tv, w = w, v\n")
		}
	}
	b.WriteString("\tv += x\n\tlib.Sink(x) // @sink(gi_real)\n\treturn v + w\n}\n")
	f.body.WriteString(b.String())
}

func (g *gen) shapeSelfcopy() {
	f := g.file("selfcopy/selfcopy.go")
	f.imports["lib"] = true
	f.body.WriteString("type inner struct{ m map[string]string }\n\ntype opts struct {\n\ta string\n\tp *inner\n}\n\n")
	for i := 0; i < g.p.Copies; i++ {
		fmt.Fprintf(&f.body, "var gOpts%d opts\n\n// a struct is copied out of a global and written back: what is in the pointee of one field must not go to the other.\nfunc SelfCopy%d() {\n\tgOpts%d.p = &inner{m: map[string]string{}}\n\tgOpts%d.p.m[\"k\"] = lib.Source() // @source(self%d)\n\tbefore := gOpts%d\n\tgOpts%d = before\n\tlib.Sink(gOpts%d.p.m[\"k\"]) // @sink(self%d)\n\tlib.Sink(gOpts%d.a) // @nosink(self%d)\n}\n\n", i, i, i, i, i, i, i, i, i, i, i)
	}
}

// Generate returns the files (path -> content) of the module of the parameters.
func Generate(p Params) (map[string]string, error) {
	g := &gen{p: p, r: rng{s: p.Seed}, files: map[string]*file{}}
	g.ballast()
	for _, s := range allShapes {
		if !p.Shapes[s] {
			continue
		}
		switch s {
		case "types":
			g.shapeTypes()
		case "stderr":
			g.shapeStderr()
		case "widen":
			g.shapeWide()
		case "scc":
			g.shapeSCC()
		case "chain":
			g.shapeChain()
		case "dyn":
			g.shapeDyn()
		case "glob":
			g.shapeGlob()
		case "giant":
			g.shapeGiant()
		case "selfcopy":
			g.shapeSelfcopy()
		}
	}
	out := map[string]string{}
	m := "// Package model has the struct every function takes.\npackage model\n\ntype Rec struct {\n"
	for i := 0; i < fields; i++ {
		m += fmt.Sprintf("\tF%d string\n", i)
	}
	out["model/model.go"] = m + "\tSub *Rec\n}\n"
	out["lib/lib.go"] = "// Package lib has the source and the sink the rules name.\npackage lib\n\nfunc Source() string { return \"x\" }\n\nfunc Sink(s string) {}\n"
	out["go.mod"] = "module scale\n\ngo 1.27\n"
	out["rules.yaml"] = "ruleset: rivus/bench-scale@1\n\nsources:\n  - symbol: scale/lib.Source\n    labels: [user_input]\n\nsinks:\n  - symbol: scale/lib.Sink\n    category: test\n"

	// main calls the shapes' entry points, so that nothing is unreachable
	var calls []string
	mainImports := map[string]bool{"model": true}
	mainImports[pk(p.Pkgs-1)] = true
	calls = append(calls, fmt.Sprintf("\t%s.F%d(r, \"\")", pk(p.Pkgs-1), p.Funcs-1))
	if p.Shapes["chain"] {
		mainImports["chain"] = true
		for i := 0; i < p.Chain; i++ {
			calls = append(calls, fmt.Sprintf("\tchain.S%d()", i))
		}
		calls = append(calls, "\tchain.Out()")
	}
	if p.Shapes["glob"] {
		mainImports["glob"] = true
		calls = append(calls, "\tglob.HandlerA()", "\tglob.HandlerB()", "\tglob.HandlerC()")
	}
	if p.Shapes["giant"] {
		mainImports["giant"] = true
		calls = append(calls, "\tgiant.Giant(r, \"\")")
	}
	if p.Shapes["selfcopy"] {
		mainImports["selfcopy"] = true
		for i := 0; i < p.Copies; i++ {
			calls = append(calls, fmt.Sprintf("\tselfcopy.SelfCopy%d()", i))
		}
	}
	for _, s := range []struct{ shape, fn string }{{"types", "TyReal()\n\ttruth.TyQuiet(\"\")"}, {"stderr", "SrReal()\n\ttruth.SrQuiet()"}, {"widen", "WdReal()"}, {"scc", "SccReal()"}, {"dyn", "DynReal()\n\ttruth.DynQuiet()"}} {
		if p.Shapes[s.shape] {
			mainImports["truth"] = true
			calls = append(calls, "\ttruth."+s.fn)
		}
	}
	mf := g.file("cmd/main.go")
	for k := range mainImports {
		mf.imports[k] = true
	}
	fmt.Fprintf(&mf.body, "func main() {\n\tr := &model.Rec{}\n%s\n}\n", strings.Join(calls, "\n"))

	for path, f := range g.files {
		if f.raw {
			out[path] = f.body.String()
			continue
		}
		pkg := strings.SplitN(path, "/", 2)[0]
		if pkg == "cmd" {
			pkg = "main"
		}
		var imps []string
		for i := range f.imports {
			switch i {
			case "strings", "fmt", "reflect", "log", "os":
				imps = append(imps, "\""+i+"\"")
			default:
				imps = append(imps, "\"scale/"+i+"\"")
			}
		}
		sort.Strings(imps)
		var b strings.Builder
		fmt.Fprintf(&b, "package %s\n\n", pkg)
		if len(imps) > 0 {
			b.WriteString("import (\n")
			for _, im := range imps {
				b.WriteString("\t" + im + "\n")
			}
			b.WriteString(")\n\n")
		}
		b.WriteString(f.body.String())
		out[path] = b.String()
	}
	for path, src := range out {
		if strings.HasSuffix(path, ".go") {
			b, err := format.Source([]byte(src))
			if err != nil {
				return nil, fmt.Errorf("%s: %v", path, err)
			}
			out[path] = string(b)
		}
	}
	return out, nil
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
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
	files[marker] = "written by bench/go-scale/gen\n"
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

func main() {
	p := Default()
	out := flag.String("out", "", "directory to write (made empty first if it was written by gen)")
	shapes := flag.String("shapes", "", "the shapes to switch on, comma separated: "+strings.Join(allShapes, ",")+", or all (none: only the ballast)")
	flag.Uint64Var(&p.Seed, "seed", p.Seed, "seed")
	flag.IntVar(&p.Funcs, "funcs", p.Funcs, "ordinary functions")
	flag.IntVar(&p.Pkgs, "pkgs", p.Pkgs, "packages of the ordinary functions")
	flag.IntVar(&p.KeepPct, "keep", p.KeepPct, "percent of the calls whose result is used")
	flag.IntVar(&p.SrcPct, "sources", p.SrcPct, "percent of the ordinary functions with a source")
	flag.IntVar(&p.EncPct, "enc", p.EncPct, "percent of the ordinary functions that give a source to the cache of types / os.Stderr")
	flag.IntVar(&p.ReadPct, "read", p.ReadPct, "percent of the ordinary functions that read the cache / a logger")
	flag.IntVar(&p.Loggers, "loggers", p.Loggers, "packages with a logger")
	flag.IntVar(&p.Ring, "ring", p.Ring, "functions in the ring (scc)")
	flag.IntVar(&p.Wide, "wide", p.Wide, "functions in the ring of wide structs (widen)")
	flag.IntVar(&p.Chain, "chain", p.Chain, "globals in the chain")
	flag.IntVar(&p.Impls, "impls", p.Impls, "implementations of the interface (dyn)")
	flag.IntVar(&p.Callers, "callers", p.Callers, "callers through the interface (dyn)")
	flag.IntVar(&p.Giant, "giant", p.Giant, "statements of the huge function")
	flag.IntVar(&p.Copies, "copies", p.Copies, "copies of the save-and-restore shape (selfcopy)")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "gen: -out is required")
		os.Exit(2)
	}
	switch *shapes {
	case "", "none":
	case "all":
		for _, s := range allShapes {
			p.Shapes[s] = true
		}
	default:
		for _, s := range strings.Split(*shapes, ",") {
			s = strings.TrimSpace(s)
			ok := false
			for _, a := range allShapes {
				ok = ok || a == s
			}
			if !ok {
				fmt.Fprintf(os.Stderr, "gen: unknown shape %q\n", s)
				os.Exit(2)
			}
			p.Shapes[s] = true
		}
	}
	if p.Funcs < 1 || p.Pkgs < 1 || p.Ring < 2 || p.Wide < 2 || p.Chain < 2 || p.Loggers < 1 {
		fmt.Fprintln(os.Stderr, "gen: funcs, pkgs and loggers at least 1; ring, wide and chain at least 2")
		os.Exit(2)
	}
	files, err := Generate(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	if err := write(*out, files); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}
