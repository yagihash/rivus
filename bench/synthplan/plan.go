// Package synthplan is what the generators of bench/dart-synth and bench/rust-synth
// share: the parameters, the sizes, and the plan of the program (which function
// calls which, and how: directly, through a closure given to a shared helper, ...).
// The generators write the plan as the code of one language. The same parameters
// always give the same plan, and so the same bytes.
package synthplan

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Params is the shape of the generated program.
type Params struct {
	Seed       uint64
	Mods       int   // modules (files); the functions are placed by level, so the imports have no cycle
	Funcs      int   // functions besides the members of the SCCs
	Depth      int   // levels of the call graph (a function calls lower levels, mostly the one just below)
	SCCs       []int // sizes of the groups of functions that call each other (a ring and chords, one level each)
	ClosurePct int   // percent of the calls made through a closure given to a shared helper (the helper calls every closure, so everything that uses it falls into one SCC)
	RelayPct   int   // percent of the closure calls that go through a second helper (the relay)
	FieldPct   int   // percent of the closure calls where the closure is kept in the field of an object and called by a method
	CapturePct int   // percent of the closure calls whose closure captures a local variable of the caller, or writes one
	Fan        int   // calls of each function
	SinkPct    int   // percent of functions that pass a value to the sink
	SrcPct     int   // percent of functions that read the source
}

// Sizes S, M and L: seconds, tens of seconds, minutes (to be seen in the README of each).
var Sizes = map[string]Params{
	"S": {Seed: 1, Mods: 5, Funcs: 100, Depth: 10, SCCs: []int{4, 3}, ClosurePct: 30, RelayPct: 30, FieldPct: 25, CapturePct: 40, Fan: 3, SinkPct: 6, SrcPct: 4},
	"M": {Seed: 1, Mods: 12, Funcs: 300, Depth: 20, SCCs: []int{10, 6, 5, 4}, ClosurePct: 30, RelayPct: 30, FieldPct: 25, CapturePct: 40, Fan: 3, SinkPct: 5, SrcPct: 3},
	"L": {Seed: 1, Mods: 20, Funcs: 600, Depth: 30, SCCs: []int{16, 10, 8, 6, 5, 4}, ClosurePct: 30, RelayPct: 30, FieldPct: 25, CapturePct: 40, Fan: 3, SinkPct: 4, SrcPct: 2},
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

// Kinds of a statement.
const (
	Call   = iota // v = f(v)
	Via           // v = helperDirect(v, closure that calls f)
	Relay         // the same through a helper that gives the closure to a second helper
	Hold          // the closure also reads a local variable of the caller (a capture)
	Field         // the closure is kept in a field of an object, and called by its method
	Write         // the closure writes a local variable of the caller, and the caller reads it
	Source        // v += source()
	Sink          // sink(v)
	Back          // a guarded call of the next member of the ring (never made at run time)
)

// Stmt is one statement of a function.
type Stmt struct {
	Kind   int
	Target int // the function called (not for Source and Sink)
}

// Func is a function of the plan: its name is "f" and the ID.
type Func struct {
	ID    int
	Level int
	Mod   int
	Ring  int // index of the SCC this is a member of, or -1
	Stmts []Stmt
}

// Plan is the program.
type Plan struct {
	P     Params
	Funcs []Func
}

// Build makes the plan of the parameters.
func Build(p Params) *Plan {
	r := &rng{s: p.Seed}
	pl := &Plan{P: p}
	byLevel := make([][]int, p.Depth)
	add := func(level, ring int) int {
		id := len(pl.Funcs)
		pl.Funcs = append(pl.Funcs, Func{ID: id, Level: level, Mod: level * p.Mods / p.Depth, Ring: ring})
		byLevel[level] = append(byLevel[level], id)
		return id
	}
	// every level has a function, so that a function of any level has a callee below it
	for l := 0; l < p.Depth; l++ {
		add(l, -1)
	}
	for i := p.Depth; i < p.Funcs; i++ {
		add(r.n(p.Depth), -1)
	}
	rings := make([][]int, len(p.SCCs))
	for i, k := range p.SCCs {
		level := 1 + (i*3+1)%(p.Depth-1)
		for j := 0; j < k; j++ {
			rings[i] = append(rings[i], add(level, i))
		}
	}
	// a callee below: mostly the level just below, sometimes any lower one
	below := func(level int) int {
		l := level - 1
		if !r.pct(70) && level > 1 {
			l = r.n(level)
		}
		for len(byLevel[l]) == 0 {
			l--
		}
		return byLevel[l][r.n(len(byLevel[l]))]
	}
	for i := range pl.Funcs {
		f := &pl.Funcs[i]
		if f.Level == 0 {
			if r.pct(p.SrcPct * 3) {
				f.Stmts = append(f.Stmts, Stmt{Kind: Source})
			}
			if r.pct(p.SinkPct * 3) {
				f.Stmts = append(f.Stmts, Stmt{Kind: Sink})
			}
			continue
		}
		if r.pct(p.SrcPct) {
			f.Stmts = append(f.Stmts, Stmt{Kind: Source})
		}
		fan := p.Fan
		if f.Ring >= 0 {
			fan-- // the ring is a call of its own
		}
		for c := 0; c < fan; c++ {
			kind := Call
			if r.pct(p.ClosurePct) {
				kind = Via
				switch {
				case r.pct(p.RelayPct):
					kind = Relay
				case r.pct(p.FieldPct):
					kind = Field
				case r.pct(p.CapturePct):
					if r.pct(50) {
						kind = Hold
					} else {
						kind = Write
					}
				}
			}
			f.Stmts = append(f.Stmts, Stmt{Kind: kind, Target: below(f.Level)})
		}
		if r.pct(p.SinkPct) {
			f.Stmts = append(f.Stmts, Stmt{Kind: Sink})
		}
	}
	// the rings: each member calls the next, and with four or more members also the one after it
	for _, ring := range rings {
		for j, id := range ring {
			f := &pl.Funcs[id]
			kind := Back
			f.Stmts = append(f.Stmts, Stmt{Kind: kind, Target: ring[(j+1)%len(ring)]})
			if len(ring) >= 4 {
				f.Stmts = append(f.Stmts, Stmt{Kind: Back, Target: ring[(j+2)%len(ring)]})
			}
		}
	}
	return pl
}

// Modules returns, for each module, the IDs of its functions in order.
func (pl *Plan) Modules() [][]int {
	out := make([][]int, pl.P.Mods)
	for _, f := range pl.Funcs {
		out[f.Mod] = append(out[f.Mod], f.ID)
	}
	return out
}

// Imports returns the modules that module m calls into (lower than m).
func (pl *Plan) Imports(m int) []int {
	seen := map[int]bool{}
	for _, f := range pl.Funcs {
		if f.Mod != m {
			continue
		}
		for _, s := range f.Stmts {
			if s.Kind == Source || s.Kind == Sink {
				continue
			}
			if tm := pl.Funcs[s.Target].Mod; tm != m {
				seen[tm] = true
			}
		}
	}
	var out []int
	for k := 0; k < m; k++ {
		if seen[k] {
			out = append(out, k)
		}
	}
	return out
}

// Main is the command of a generator: it reads the flags, builds the plan and writes
// the files that render makes of it.
func Main(name string, render func(*Plan) map[string]string) {
	size := flag.String("size", "S", "S, M or L")
	outDir := flag.String("out", "", "the directory to write (it is removed first if gen wrote it)")
	seed := flag.Uint64("seed", 0, "the seed (0: the one of the size)")
	mods := flag.Int("mods", -1, "modules")
	funcs := flag.Int("funcs", -1, "functions")
	depth := flag.Int("depth", -1, "levels")
	sccs := flag.String("sccs", "", "sizes of the SCCs, separated by commas (\"-\": none)")
	closure := flag.Int("closure", -1, "percent of calls through a closure given to a helper")
	relay := flag.Int("relay", -1, "percent of the closure calls through a second helper")
	field := flag.Int("field", -1, "percent of the closure calls through a closure kept in a field")
	capture := flag.Int("capture", -1, "percent of the closure calls that capture or write a local")
	fan := flag.Int("fan", -1, "calls of each function")
	sinks := flag.Int("sinks", -1, "percent of functions with a sink")
	sources := flag.Int("sources", -1, "percent of functions with a source")
	flag.Parse()
	p, ok := Sizes[*size]
	if !ok {
		fmt.Fprintln(os.Stderr, name+": the size is S, M or L")
		os.Exit(2)
	}
	if *seed != 0 {
		p.Seed = *seed
	}
	set := func(dst *int, v int) {
		if v >= 0 {
			*dst = v
		}
	}
	set(&p.Mods, *mods)
	set(&p.Funcs, *funcs)
	set(&p.Depth, *depth)
	set(&p.ClosurePct, *closure)
	set(&p.RelayPct, *relay)
	set(&p.FieldPct, *field)
	set(&p.CapturePct, *capture)
	set(&p.Fan, *fan)
	set(&p.SinkPct, *sinks)
	set(&p.SrcPct, *sources)
	if *sccs != "" {
		p.SCCs = nil
		if *sccs != "-" {
			for _, f := range strings.Split(*sccs, ",") {
				n, err := strconv.Atoi(f)
				if err != nil || n < 2 {
					fmt.Fprintln(os.Stderr, name+": -sccs wants sizes of 2 or more")
					os.Exit(2)
				}
				p.SCCs = append(p.SCCs, n)
			}
		}
	}
	if *outDir == "" || p.Depth < 3 || p.Mods < 1 || p.Mods > p.Depth || p.Funcs < p.Depth {
		fmt.Fprintln(os.Stderr, name+": -out is required; depth at least 3, mods at most depth, funcs at least depth")
		os.Exit(2)
	}
	if err := WriteFiles(*outDir, name, render(Build(p))); err != nil {
		fmt.Fprintln(os.Stderr, name+":", err)
		os.Exit(1)
	}
}

const marker = ".generated-by-gen"

// WriteFiles writes the files into dir; it removes dir first only if a generator wrote it.
func WriteFiles(dir, name string, files map[string]string) error {
	if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	} else if ents, err := os.ReadDir(dir); err == nil && len(ents) > 0 {
		return fmt.Errorf("%s exists and was not written by a generator", dir)
	}
	files[marker] = "written by " + name + "\n"
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
