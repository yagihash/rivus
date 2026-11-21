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
	Seed      uint64
	Funcs     int             // ordinary functions (the ballast: a call graph without sinks)
	Pkgs      int             // packages of the ballast
	KeepPct   int             // percent of the calls whose result is used (the others only call)
	SrcPct    int             // percent of the ballast with a source (it flows up, to no sink)
	EncPct    int             // percent of the ballast that hands its source to a cache of types / to os.Stderr
	ReadPct   int             // percent of the ballast that reads the cache / a logger
	Loggers   int             // packages with a logger made of os.Stderr
	Ring      int             // functions that call each other (shape scc)
	Wide      int             // functions of the ring of wide structs (shape widen)
	Chain     int             // globals in a chain (shape chain)
	TStages   int             // caches of types in a chain, each one a global (shape typeschain)
	Impls     int             // implementations of the interface (shape dyn)
	Callers   int             // functions that call through the interface (shape dyn)
	Giant     int             // statements of the one huge function (shape giant)
	Copies    int             // copies of the "save and restore a global struct" shape (shape selfcopy)
	Ops       int             // operations that share one chain of middleware (shape middleware)
	Stages    int             // stages of the chain (shapes middleware, stagefield)
	OutFields int             // fields of the output type of each operation (shape stagefield)
	Shapes    map[string]bool // types stderr widen scc chain dyn glob giant selfcopy middleware stagefield
}

// allShapes is the list of the shapes, in the order they are written.
var allShapes = []string{"types", "stderr", "widen", "scc", "chain", "typeschain", "dyn", "glob", "giant", "selfcopy", "middleware"}

// namedShapes are the shapes that -shapes takes by name only: "all" does not turn them on (the input of
// bench/compare.sh's light tier stays the same for the base and the head).
var namedShapes = []string{"stagefield", "registry", "longlived"}

// Default is the size of the profiles (about 2,000 functions).
func Default() Params {
	return Params{Seed: 1, Funcs: 2000, Pkgs: 20, KeepPct: 15, SrcPct: 4, EncPct: 4, ReadPct: 25, Loggers: 8, Ring: 200, Wide: 16, Chain: 10, TStages: 14, Impls: 150, Callers: 40, Giant: 3000, Copies: 40, Ops: 40, Stages: 9, OutFields: 12, Shapes: map[string]bool{}}
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
		if g.on("middleware") && i%8 == 0 && g.p.Ops > 0 {
			// a function of the program calls an operation of the SDK with what it has (no random draw: the
			// ballast of the other shapes is the same)
			f.imports["ops"] = true
			fmt.Fprintf(&b, "\tv += ops.Op%d(r, v)\n", (i/8)%g.p.Ops)
		}
		if g.on("registry") && i%8 == 0 {
			// one function in eight decodes a value (a call of the gRPC stack): what the decoder carries goes up
			f.imports["reg"] = true
			b.WriteString("\tv += reg.Decode(v)\n")
		}
		if g.on("longlived") && i%8 == 0 {
			// one function in eight asks the stream for what it sent last: what flush wrote goes up the call graph
			f.imports["ll"] = true
			b.WriteString("\tv += ll.Last(v)\n")
		}
		if g.on("stagefield") && i%8 == 0 && g.p.Ops > 0 {
			f.imports["sfops"] = true
			fmt.Fprintf(&b, "\tv += sfops.Op%d(r, v)\n", (i/8)%g.p.Ops)
		}
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
				if g.on("typeschain") {
					f.imports["tchain"] = true
					b.WriteString("\ttchain.Put(x)\n")
				}
			}
		}
		if g.r.pct(g.p.ReadPct) {
			if g.on("types") {
				f.imports["tcache"] = true
				b.WriteString("\tv += tcache.Describe(v)\n")
			}
			if g.on("typeschain") {
				f.imports["tchain"] = true
				b.WriteString("\tv += tchain.Get(v)\n")
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

// shapeTypesChain is the cache of types of a library that registers what it knows in a second one, and that in a third
// (each one a package variable): the type of a value goes one cache further in a pass, so the data of the value reaches
// the readers of the last one after TStages passes (without the models of reflect.TypeOf).
func (g *gen) shapeTypesChain() {
	n := g.p.TStages
	var b strings.Builder
	b.WriteString("// Package tchain is a chain of caches of types.\npackage tchain\n\nimport (\n\t\"fmt\"\n\t\"reflect\"\n)\n\ntype Info struct{ T reflect.Type }\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "var m%d = map[reflect.Type]*Info{}\n", i)
	}
	b.WriteString("\n// Put gives the first cache the type of the value.\nfunc Put(v interface{}) {\n\tt := reflect.TypeOf(v)\n\tm0[t] = &Info{T: t}\n}\n\n")
	for i := n - 1; i >= 1; i-- {
		fmt.Fprintf(&b, "// Step%d registers what the cache %d knows in the cache %d.\nfunc Step%d() {\n\tfor k, v := range m%d {\n\t\tm%d[k] = v\n\t}\n}\n\n", i, i-1, i, i, i-1, i)
	}
	fmt.Fprintf(&b, "// Get reads the last cache.\nfunc Get(v interface{}) string {\n\treturn fmt.Sprint(m%d[reflect.TypeOf(v)])\n}\n", n-1)
	g.rawFile("tchain/tchain.go", b.String())
	t := g.file("truth/typeschain.go")
	t.imports["lib"], t.imports["tchain"] = true, true
	t.body.WriteString(`// TcReal: the data of a value goes on to the sink, and next to it the type of the value goes into the chain of caches.
func TcReal() {
	x := lib.Source() // @source(tc_real)
	tchain.Put(x)
	lib.Sink(x) // @sink(tc_real)
}

// TcQuiet reads the last cache only: no source reaches it, but for the chain that joins every writer to every reader.
func TcQuiet(s string) {
	lib.Sink(tchain.Get(s)) // @sink(none)
}

`)
}

func (g *gen) shapeChain() {
	f := g.file("chain/chain.go")
	f.imports["lib"] = true
	n := g.p.Chain
	miss := ""
	if n > 10 {
		miss = " @miss(silent)" // more steps than the passes the engine makes (MaxPasses 12): the flow is lost
	}
	// the steps are written in the order that is against the order of analysis: the last first
	for i := n - 1; i >= 1; i-- {
		fmt.Fprintf(&f.body, "var G%d string\n\nfunc S%d() { G%d = G%d }\n\n", i, i, i, i-1)
	}
	fmt.Fprintf(&f.body, "var G0 string\n\nfunc S0() { G0 = lib.Source() } // @source(ch_real)%s\n\nfunc Out() {\n\tlib.Sink(G%d) // @sink(ch_real)\n}\n\n", miss, n-1)
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

// shapeMiddleware is the shape of the AWS SDK for Go (smithy-go): a stack of steps, each a list of
// middleware that its HandleMiddleware chains with a decorated handler, and N operations that put
// the same M stages into a stack of their own and call it. Every operation reaches the chain's
// calls through one interface (Handler.Handle, Middleware.HandleMiddleware), so the candidates of
// those calls are all the stages and all the operations' last handlers.
func (g *gen) shapeMiddleware() {
	const steps = 3
	f := g.file("mw/mw.go")
	f.imports["model"] = true
	f.body.WriteString(`// Handler is what a step hands to the next one; Middleware is one stage.
type Handler interface {
	Handle(r *model.Rec, in any) (out any, md string)
}

type Middleware interface {
	HandleMiddleware(r *model.Rec, in any, next Handler) (out any, md string)
}

// node is a stage with the handler after it (smithy-go's decoratedHandler, linked in a list).
type node struct {
	Next Handler
	With Middleware
}

func (n *node) Handle(r *model.Rec, in any) (any, string) {
	return n.With.HandleMiddleware(r, in, n.Next)
}

// Step is an ordered list of stages: it is a stage too, of the stack. The last node of the
// list is given the handler after the step when the step runs (s.tail.Next = next).
type Step struct{ head, tail *node }

func (s *Step) Add(m Middleware) {
	n := &node{With: m}
	if s.head == nil {
		s.head = n
	} else {
		s.tail.Next = n
	}
	s.tail = n
}

func (s *Step) HandleMiddleware(r *model.Rec, in any, next Handler) (any, string) {
	if s.head == nil {
		return next.Handle(r, in)
	}
	s.tail.Next = next
	return s.head.Handle(r, in)
}

// Stack is the steps in order.
type Stack struct{ Steps [` + fmt.Sprint(steps) + `]*Step }

func NewStack() *Stack {
	s := &Stack{}
	for i := range s.Steps {
		s.Steps[i] = &Step{}
	}
	return s
}

func (s *Stack) HandleMiddleware(r *model.Rec, in any, last Handler) (any, string) {
	h := last
	for i := len(s.Steps) - 1; i >= 0; i-- {
		h = &node{Next: h, With: s.Steps[i]}
	}
	return h.Handle(r, in)
}

`)
	for j := 0; j < g.p.Stages; j++ {
		fmt.Fprintf(&f.body, "// S%d is a stage shared by every operation.\ntype S%d struct{}\n\nfunc (S%d) HandleMiddleware(r *model.Rec, in any, next Handler) (any, string) {\n\treq, _ := in.(*model.Rec)\n\tif req != nil {\n\t\treq.F%d = req.F%d + r.F%d\n\t\treq.F%d = req.F%d\n\t\treq.F%d += req.F%d\n\t}\n\treturn next.Handle(r, in)\n}\n\n", j, j, j, j%fields, (j+3)%fields, (j+5)%fields, (j+3)%fields, (j+7)%fields, (j+9)%fields, (j+7)%fields)
	}
	for j := 0; j < g.p.Stages; j++ {
		// the options of a client are functions that an operation calls on its stack (smithy-go: APIOptions)
		fmt.Fprintf(&f.body, "func Option%d() func(*Stack) error {\n\tm := S%d{}\n\treturn func(st *Stack) error {\n\t\tst.Steps[%d].Add(m)\n\t\treturn nil\n\t}\n}\n\n", j, j, j%steps)
	}
	o := g.file("ops/ops.go")
	o.imports["model"], o.imports["mw"] = true, true
	for i := 0; i < g.p.Ops; i++ {
		fmt.Fprintf(&o.body, "// last%d is the last handler of operation %d.\ntype last%d struct{}\n\nfunc (last%d) Handle(r *model.Rec, in any) (any, string) {\n\treq, _ := in.(*model.Rec)\n\tif req == nil {\n\t\treturn nil, \"\"\n\t}\n\treturn req.F%d + req.F%d, \"\"\n}\n\n", i, i, i, i, i%fields, (i+1)%fields)
		fmt.Fprintf(&o.body, "func Op%d(r *model.Rec, in string) string {\n\tst := mw.NewStack()\n\treq := &model.Rec{F0: in}\n", i)
		o.body.WriteString("\topts := []func(*mw.Stack) error{")
		for j := 0; j < g.p.Stages; j++ {
			fmt.Fprintf(&o.body, "mw.Option%d(), ", j)
		}
		o.body.WriteString("}\n\tfor _, fn := range opts {\n\t\t_ = fn(st)\n\t}\n")
		fmt.Fprintf(&o.body, "\tout, _ := st.HandleMiddleware(r, req, last%d{})\n\ts, _ := out.(string)\n\treturn s\n}\n\n", i)
	}
	t := g.file("truth/mw.go")
	t.imports["lib"], t.imports["model"], t.imports["ops"] = true, true, true
	t.body.WriteString(`// MwReal: a value goes into one operation and comes out of its chain of stages.
func MwReal() {
	x := lib.Source() // @source(mw_real)
	lib.Sink(ops.Op0(&model.Rec{}, x)) // @sink(mw_real)
}

// MwQuiet gives another operation a constant: it shares every call of the chain with the first.
func MwQuiet() {
	x := lib.Source() // @source(mw_quiet)
	lib.Sink(ops.Op1(&model.Rec{}, "k")) // @nosink(mw_quiet)
	lib.Sink(x)                          // @sink(mw_quiet)
}

`)
}

// shapeStagefield is the shape of memos' AWS SDK that the shape middleware does not reach: every operation
// has its OWN stage types (not nine shared ones) and its own output type with its own field names, and a
// stage writes the fields of its operation's output into the result that goes back up the chain. The steps
// of the stack build a struct with an interface FIELD (node.With) in a loop, and `n.With.HandleMiddleware`
// has every stage type of every operation as a candidate (ops x stages of them). The chain is one strongly
// connected component, and the union of the fields of all the outputs (ops x fields paths) goes into the
// summary of every member: elements of the order of (members) x ops x fields. Parameters: -ops, -stages,
// -fields.
func (g *gen) shapeStagefield() {
	const steps = 3
	f := g.file("sf/sf.go")
	f.imports["model"] = true
	f.body.WriteString(`// Req is what the stages read; Resp is what goes back up the chain (smithy-go: DeserializeOutput).
type Req struct{ Param string }

type Resp struct {
	Raw    string
	Result any
}

// Handler is what a step hands to the next one; Middleware is one stage.
type Handler interface {
	Handle(r *model.Rec, in *Req) (out *Resp, md string)
}

type Middleware interface {
	HandleMiddleware(r *model.Rec, in *Req, next Handler) (out *Resp, md string)
}

// node is a stage with the handler after it (smithy-go's decoratedHandler): the stage is an interface FIELD.
type node struct {
	Next Handler
	With Middleware
}

func (n *node) Handle(r *model.Rec, in *Req) (*Resp, string) {
	return n.With.HandleMiddleware(r, in, n.Next)
}

// Step is an ordered list of stages: it is a stage too, of the stack.
type Step struct{ head, tail *node }

func (s *Step) Add(m Middleware) {
	n := &node{With: m}
	if s.head == nil {
		s.head = n
	} else {
		s.tail.Next = n
	}
	s.tail = n
}

func (s *Step) HandleMiddleware(r *model.Rec, in *Req, next Handler) (*Resp, string) {
	if s.head == nil {
		return next.Handle(r, in)
	}
	s.tail.Next = next
	return s.head.Handle(r, in)
}

// Stack is the steps in order.
type Stack struct{ Steps [` + fmt.Sprint(steps) + `]*Step }

func NewStack() *Stack {
	s := &Stack{}
	for i := range s.Steps {
		s.Steps[i] = &Step{}
	}
	return s
}

func (s *Stack) Add(step int, m Middleware) { s.Steps[step].Add(m) }

func (s *Stack) HandleMiddleware(r *model.Rec, in *Req, last Handler) (*Resp, string) {
	h := last
	for i := len(s.Steps) - 1; i >= 0; i-- {
		h = &node{Next: h, With: s.Steps[i]}
	}
	return h.Handle(r, in)
}
`)
	o := g.file("sfops/ops.go")
	o.imports["model"], o.imports["sf"] = true, true
	m := g.p.Stages
	for i := 0; i < g.p.Ops; i++ {
		fmt.Fprintf(&o.body, "// Out%d is the output type of operation %d: its own field names.\ntype Out%d struct {\n", i, i, i)
		for k := 0; k < g.p.OutFields; k++ {
			fmt.Fprintf(&o.body, "\tOp%dF%d string\n", i, k)
		}
		o.body.WriteString("}\n\n")
		for j := 0; j < m; j++ {
			fmt.Fprintf(&o.body, "// Stage%d_%d is stage %d of operation %d; it writes some fields of Out%d.\ntype Stage%d_%d struct{}\n\n", i, j, j, i, i, i, j)
			fmt.Fprintf(&o.body, "func (Stage%d_%d) HandleMiddleware(r *model.Rec, in *sf.Req, next sf.Handler) (*sf.Resp, string) {\n\tresp, md := next.Handle(r, in)\n\tif resp == nil {\n\t\treturn nil, md\n\t}\n\to, _ := resp.Result.(*Out%d)\n\tif o == nil {\n\t\to = &Out%d{}\n\t\tresp.Result = o\n\t}\n", i, j, i, i)
			for k := j; k < g.p.OutFields; k += m {
				fmt.Fprintf(&o.body, "\to.Op%dF%d = resp.Raw + in.Param\n", i, k)
			}
			o.body.WriteString("\treturn resp, md\n}\n\n")
		}
		fmt.Fprintf(&o.body, "// last%d is the last handler of operation %d.\ntype last%d struct{}\n\nfunc (last%d) Handle(r *model.Rec, in *sf.Req) (*sf.Resp, string) {\n\treturn &sf.Resp{Raw: in.Param}, \"\"\n}\n\n", i, i, i, i)
		fmt.Fprintf(&o.body, "func Op%d(r *model.Rec, in string) string {\n\tst := sf.NewStack()\n", i)
		for j := 0; j < m; j++ {
			fmt.Fprintf(&o.body, "\tst.Add(%d, Stage%d_%d{})\n", j%steps, i, j)
		}
		fmt.Fprintf(&o.body, "\tresp, _ := st.HandleMiddleware(r, &sf.Req{Param: in}, last%d{})\n\tif resp == nil {\n\t\treturn \"\"\n\t}\n\ts := resp.Raw\n\tif o, ok := resp.Result.(*Out%d); ok {\n\t\ts += o.Op%dF0\n\t}\n\treturn s\n}\n\n", i, i, i)
	}
	t := g.file("truth/sf.go")
	t.imports["lib"], t.imports["model"], t.imports["sfops"] = true, true, true
	t.body.WriteString(`// SfReal: a value goes into one operation and comes out of its chain of stages.
func SfReal() {
	x := lib.Source() // @source(sf_real)
	lib.Sink(sfops.Op0(&model.Rec{}, x)) // @sink(sf_real)
}

// SfQuiet gives another operation a constant: it shares every call of the chain with the first.
func SfQuiet() {
	x := lib.Source() // @source(sf_quiet)
	lib.Sink(sfops.Op1(&model.Rec{}, "k")) // @nosink(sf_quiet)
	lib.Sink(x)                            // @sink(sf_quiet)
}

`)
}

// shapeRegistry is the shape of bench/gcp's gRPC stack (the false "other sources" of a true finding): N
// handlers take a request (a source by its type), and each one writes the body of its request into one
// registry (a package variable that is a map, as the connection table or the trace registry of a
// dependency) under a key of its own. Handler 0 also decodes its body and sends it to the sink; the decoder
// reads the registry under another key ("codec") and builds its result with it. The key is a constant and
// the elements of a map do not say which key they came from, so the value that the decoder returns carries
// the requests of all N handlers: the one real finding (handler 0 to the sink) lists the other N-1
// handlers' requests as other sources, and none of them was ever in the decoded value. Parameter: -ops (N).
// The finding lists at most 5 (maxOtherSources) and counts the rest (other_sources_count). The code has no answer key
// (@source marks): the flow of handler 0 and the number of other sources are in the expected file.
func (g *gen) shapeRegistry() {
	f := g.file("reg/reg.go")
	f.imports["lib"] = true
	f.body.WriteString(`// Req is a request: a source by its type (rules.yaml).
type Req struct{ Body string }

// M is the registry: every handler writes it, the decoder reads it.
var M = map[string]string{}

// Decode builds its result from its argument and the codec that the registry holds.
func Decode(s string) string { return s + M["codec"] }

`)
	for i := 0; i < g.p.Ops; i++ {
		if i == 0 {
			f.body.WriteString("// Handler0 registers its request and sends the decoded body to the sink.\nfunc Handler0(r *Req) {\n\tM[\"h0\"] = r.Body\n\tlib.Sink(Decode(r.Body))\n}\n\n")
			continue
		}
		fmt.Fprintf(&f.body, "// Handler%d registers its request.\nfunc Handler%d(r *Req) {\n\tM[\"h%d\"] = r.Body\n}\n\n", i, i, i)
	}
}

// shapeLonglived is the shape of bench/gcp's long-lived receivers (bench/explore/LOG.md, measurement 5): N handlers
// each put the body of their own request (a source by its type) into a receiver that lives on (a connection, a
// stream, the cache of a router) by a library call that fills the receiver as a whole (encoding/json.Unmarshal; the
// engine has no model for what it fills), and pass the whole receiver to a function (flush) that reads ANOTHER
// field (Meta) and writes it to a package variable (Sent). Meta carries the tag `json:"-"`, so the library never
// fills it. Reader reads the variable and sends it to the sink. Only handler 0 sets Meta itself, so the one real
// finding is handler 0 to the sink; the requests of the other N-1 handlers never reach Sent, and the finding lists
// them as other sources, since the engine does not tell the fields of the receiver apart after the library call.
// One function in eight of the ballast calls Last, which returns what flush wrote, so what Sent carries goes up the
// call graph (as the 1,082 functions of bench/gcp). Parameter: -ops (N). The code has no answer key (@source marks): the flow of handler 0 and the number of other
// sources are in the expected file. The acceptance of a flow that tells the fields apart (stage 5 of
// docs/DESIGN-shared-state.md: from: / to: on the path, the other field quiet) is other_sources_count 0 with the
// same finding. (-shapes longlived only: "all" does not turn it on.)
func (g *gen) shapeLonglived() {
	f := g.file("ll/ll.go")
	f.imports["lib"], f.imports["encoding/json"] = true, true
	f.body.WriteString(`// Req is a request: a source by its type (rules.yaml).
type Req struct{ Body string }

// Stream is a receiver that lives on: the handlers fill it as a whole (json.Unmarshal, which never fills Meta), and flush reads Meta.
type Stream struct {
	Pending string
	Meta    string ` + "`json:\"-\"`" + `
}

// Conn is the one long-lived stream; Sent is the package variable flush writes.
var Conn = &Stream{}

var Sent string

// flush gets the whole receiver and reads another field than the one the library fills.
func flush(s *Stream) { Sent = s.Meta }

// Reader reads the variable and sends it to the sink.
func Reader() { lib.Sink(Sent) }

// Last flushes the stream and gives back what was sent (the ballast calls it: what flush wrote goes up the call graph).
func Last(s string) string {
	flush(Conn)
	return s + Sent
}

`)
	for i := 0; i < g.p.Ops; i++ {
		if i == 0 {
			f.body.WriteString("// Handler0 puts its request into Meta itself.\nfunc Handler0(r *Req) {\n\tConn.Meta = r.Body\n\tflush(Conn)\n}\n\n")
			continue
		}
		fmt.Fprintf(&f.body, "// Handler%d fills the stream with its request by the library (Meta is never filled).\nfunc Handler%d(r *Req) {\n\tjson.Unmarshal([]byte(r.Body), Conn)\n\tflush(Conn)\n}\n\n", i, i)
	}
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
	for _, s := range append(append([]string{}, allShapes...), namedShapes...) {
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
		case "typeschain":
			g.shapeTypesChain()
		case "dyn":
			g.shapeDyn()
		case "glob":
			g.shapeGlob()
		case "giant":
			g.shapeGiant()
		case "selfcopy":
			g.shapeSelfcopy()
		case "middleware":
			g.shapeMiddleware()
		case "stagefield":
			g.shapeStagefield()
		case "registry":
			g.shapeRegistry()
		case "longlived":
			g.shapeLonglived()
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
	paramRule := ""
	if p.Shapes["registry"] {
		paramRule = "  - kind: param_type\n    type: \"*scale/reg.Req\"\n    labels: [user_input]\n"
	}
	if p.Shapes["longlived"] {
		paramRule += "  - kind: param_type\n    type: \"*scale/ll.Req\"\n    labels: [user_input]\n"
	}
	out["rules.yaml"] = "ruleset: rivus/bench-scale@1\n\nsources:\n  - symbol: scale/lib.Source\n    labels: [user_input]\n" + paramRule + "\nsinks:\n  - symbol: scale/lib.Sink\n    category: test\n"

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
	if p.Shapes["registry"] {
		mainImports["reg"] = true
		for i := 0; i < p.Ops; i++ {
			calls = append(calls, fmt.Sprintf("\treg.Handler%d(&reg.Req{})", i))
		}
	}
	if p.Shapes["longlived"] {
		mainImports["ll"] = true
		for i := 0; i < p.Ops; i++ {
			calls = append(calls, fmt.Sprintf("\tll.Handler%d(&ll.Req{})", i))
		}
		calls = append(calls, "\tll.Reader()")
	}
	if p.Shapes["selfcopy"] {
		mainImports["selfcopy"] = true
		for i := 0; i < p.Copies; i++ {
			calls = append(calls, fmt.Sprintf("\tselfcopy.SelfCopy%d()", i))
		}
	}
	for _, s := range []struct{ shape, fn string }{{"types", "TyReal()\n\ttruth.TyQuiet(\"\")"}, {"typeschain", "TcReal()\n\ttruth.TcQuiet(\"\")"}, {"stderr", "SrReal()\n\ttruth.SrQuiet()"}, {"widen", "WdReal()"}, {"scc", "SccReal()"}, {"dyn", "DynReal()\n\ttruth.DynQuiet()"}, {"middleware", "MwReal()\n\ttruth.MwQuiet()"}, {"stagefield", "SfReal()\n\ttruth.SfQuiet()"}} {
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
			case "strings", "fmt", "reflect", "log", "os", "encoding/json":
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
	flag.IntVar(&p.TStages, "tstages", p.TStages, "caches in the chain of caches of types")
	flag.IntVar(&p.Impls, "impls", p.Impls, "implementations of the interface (dyn)")
	flag.IntVar(&p.Callers, "callers", p.Callers, "callers through the interface (dyn)")
	flag.IntVar(&p.Giant, "giant", p.Giant, "statements of the huge function")
	flag.IntVar(&p.Ops, "ops", p.Ops, "operations that share the chain of middleware (middleware)")
	flag.IntVar(&p.Stages, "stages", p.Stages, "stages of the chain of middleware (middleware)")
	flag.IntVar(&p.OutFields, "fields", p.OutFields, "fields of the output type of each operation (stagefield)")
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
			for _, a := range append(append([]string{}, allShapes...), namedShapes...) {
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
