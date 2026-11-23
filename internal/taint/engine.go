package taint

import (
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yagihash/rivus/internal/sg"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sites"
)

// Options tunes the analysis.
type Options struct {
	// K is the maximum access path depth (ADR-014).
	K int
	// DepK is the access path depth of the functions of the dependencies (an experiment, -dep-k): the paths are cut
	// at DepK steps there, and at K in the functions of the project. Zero is the same as K. A cut path stands for
	// everything below it, so a smaller DepK only adds flows (docs/DESIGN-deps-cost-status.md).
	DepK int
	// MaxPasses bounds the number of times the whole program is revisited
	// while summaries change.
	MaxPasses int
	// Timeout bounds the whole run. When it expires, the functions not yet
	// analysed are left out and the result says so (depth_limit). Zero means
	// no limit.
	Timeout time.Duration
	// FuncTimeout bounds the time of the analysis of one function, whatever
	// the budget; a function that exceeds it is treated like one that
	// exceeded the budget (depth_limit). Zero means the default.
	FuncTimeout time.Duration
	// Budget bounds the number of elements added while analysing one
	// function. A function that exceeds it is cut short and recorded as
	// depth_limit; its summary may then miss flows. Zero means the default.
	Budget int
	// SlowFunc, if set, is called for functions whose analysis took longer
	// than SlowThreshold. It is a development aid. d is the wall time; cpu is the CPU time of the
	// goroutine's own thread over the same span (0 when the system has no per-thread clock): wall time
	// grows with the collector and with the other workers, so a wall time well over the CPU time is not
	// the function's work. While SlowFunc is set, compute locks its goroutine to its OS thread to read it.
	// Workers is the number of functions analysed at the same time: the
	// strongly connected components of one level of the call graph (the ones
	// that do not call each other) run together. Zero is the number of CPUs.
	// The result does not depend on it.
	Workers int
	// NumericTaint follows taint through values of a number or bool type.
	// By default it stops there: the sinks of the injection categories take
	// text, and a number made from a request has no text to inject. Turn it
	// on for rules whose sinks are quantities (a size, a count, a delay).
	NumericTaint bool
	// PassStats adds to the stats what changed in the summaries in each pass, and the functions without a
	// model that tainted values reached the most (ADR-061). TraceElement is a site ID: where its
	// elements first leave their function is printed to stderr. Neither changes the findings.
	PassStats    bool
	TraceElement string
	// PassStatsTop is how many packages packages_top lists (0: all of them), and PassStatsSort is what they are
	// ordered by: "reanalyses" (the default), "time", "functions" or "elems"; equal values go by name.
	PassStatsTop  int
	PassStatsSort string
	// FuncValues is the diagnostic of the function values (funcvalues.go): the places that make a function value are
	// synthetic sources, the calls of a function value are synthetic sinks (the sites must be those of
	// FuncValueSites), and Result.FuncCandidates says what each call may call. The findings are not reported.
	FuncValues    bool
	SlowFunc      func(fn string, d, cpu time.Duration, outputs, roots int)
	SlowThreshold time.Duration
}

// DefaultOptions are the defaults.
func DefaultOptions() Options {
	return Options{K: 3, MaxPasses: 12, Budget: 400000, FuncTimeout: 30 * time.Second, PassStatsTop: 20}
}

// Engine runs the whole-program analysis.
type Engine struct {
	suppressions []*Suppression // the places that models removed taint, for the findings that are built after them
	// the diagnostics (ADR-061)
	pass        int
	passChanges []PassChange
	seenEver    map[*sg.Func]map[uint64]struct{}
	unmodeled   unmodeledCount
	traced      map[*sg.Func]bool
	pkgs        map[string]*pkgAcc
	largestSCC  []*sg.Func // the functions of the largest component (-pass-stats only)
	// dispatch caches dispatchGuards by call: what it makes depends on the call
	// alone, and it is asked at every analysis of the function.
	dispatch sync.Map // *sg.Instr -> map[*sg.Func][]Guard
	// valueGuards is the same for the calls of function values (valueGuards).
	valueGuards sync.Map // *sg.Instr -> map[string][]Guard
	// fvDropped is the calls (symbol, then the call) where a model dropped a function value (Options.FuncValues).
	fvDropped sync.Map // string -> *sync.Map of *sg.Instr

	// implsIndex are the implementations of the trait methods that the program has, by the last name of the
	// trait and the method (callrows.go, methodImpls)
	implsOnce  sync.Once
	implsIndex map[string][]*sg.Func
	// projectUnits are the packages that have a function of the code of the project (bodyreplaced.go)
	projectOnce   sync.Once
	projectUnits  map[string]bool
	methodsOnce   sync.Once
	methodsByType map[string][]*sg.Func
	// identityStores are the closures made (the make_closure instructions) whose write of a global was left out because it
	// writes back the value that was read from it (stats.identity_global_stores).
	identityStores sync.Map // *sg.Instr -> struct{}
	prog           *sg.Program
	// makers says where each closure is made (the function and the place): made once, when first asked.
	makersOnce sync.Once
	makers     map[string]closureSite
	// fieldNames are the field names that the instructions of the program use (the sinks that name a
	// field check theirs against it: fieldsink.go).
	fieldNamesOnce sync.Once
	fieldNames     map[string]bool
	// reach is what the program can reach from its entry points (nil: unknown).
	reach map[*sg.Func]bool
	// reachCB is reach with the methods that a framework calls as entry points too (ADR-073).
	reachCB map[*sg.Func]bool
	texts   ir.Texts
	sites   *sites.Result
	rules   *model.Rules
	opts    Options
	// aliasArgs: the calls whose result is the same storage as an argument
	// (a model flow with alias), by symbol.
	aliasArgs map[string]int

	// mu guards what the analyses of the functions of one level write together:
	// the summaries, the states, the marks of the functions to analyse again,
	// the time spent and the notes. The global state is not written during a
	// level: what a function writes to it waits in pendingGlobals and
	// pendingReaders, and is merged when the level is done, so that what a
	// function reads of it does not depend on who ran first.
	mu             sync.RWMutex
	pendingGlobals []globalWrite
	pendingReaders []globalReader
	dirty          map[*sg.Func]bool
	summaries      map[*sg.Func]*Summary
	states         map[*sg.Func]*state
	globals        *globalStore

	// notConverged lists the functions whose local analysis did not reach a
	// fixed point, each once (however many times it was analysed again).
	notConverged    []string
	notConvergedSet map[string]bool
	// degraded lists the functions whose analysis hit a limit, with which (the
	// deg* bits). They are analysed once and then left alone.
	degraded map[*sg.Func]uint8
	// spent is the time each function has used so far.
	spent map[*sg.Func]time.Duration
	// guardLimit lists the functions with a flow that had more guards than
	// maxGuards: the flow is kept without guards (ADR-006).
	guardLimit map[*sg.Func]bool
	// coarsened lists the functions in which an access path was cut at the depth
	// k, or the cell of a parameter was merged into the whole of it: the cells are
	// coarser than the code (more than the truth, never less).
	coarsened map[*sg.Func]string
	// writers lists, for each package variable, the functions that store to it.
	// deadline is when the whole run has to stop (zero: no limit).
	deadline time.Time
	writers  map[*sg.Value][]string
	// loadErrors are the problems the loading of the code had.
	loadErrors  []ir.LoadError
	skippedOnce sync.Once                // skippedBody indexes ir.FnSkipped once
	skipped     map[string]*ir.FnSkipped // the functions whose body the extractor could not read, by ID
	loadOnce    sync.Once                // indexLoadErrors runs once (buildFinding may run in several goroutines)
	recovered   map[string]string        // recoveredFiles
	errFuncs    map[string]string        // errorFunctions, made once
	// callSites lists, for each function, the calls that may reach it.
	callSites map[*sg.Func][]*sg.Instr
	// heldNothing: for each package variable that holds nothing, the functions that wrote to it (ADR-056).
	heldMu      sync.Mutex
	heldNothing map[string]map[*sg.Func]bool
}

// writersOf lists the functions that directly store to the package variable.
func (e *Engine) writersOf(g *sg.Value) []string {
	if e.writers == nil {
		e.writers = map[*sg.Value][]string{}
		seen := map[*sg.Value]map[string]bool{}
		for _, fn := range e.prog.Funcs {
			for _, b := range fn.Blocks {
				for _, in := range b.Instrs {
					var gl *sg.Value
					switch in.Op {
					case ir.OpGlobalStore:
						gl = e.prog.Global(in.IR.Global)
					case ir.OpStore, ir.OpFieldStore, ir.OpIndexStore:
						gl = globalRoot(in.Args[0])
					default:
						continue
					}
					if gl == nil {
						continue
					}
					id := e.funcID(fn)
					if seen[gl] == nil {
						seen[gl] = map[string]bool{}
					}
					if !seen[gl][id] {
						seen[gl][id] = true
						e.writers[gl] = append(e.writers[gl], id)
					}
				}
			}
		}
		for _, l := range e.writers {
			sort.Strings(l)
		}
	}
	return e.writers[g]
}

// globalRoot follows field and element addresses back to a package variable.
func globalRoot(v *sg.Value) *sg.Value {
	for v != nil {
		if v.Kind == sg.KGlobal {
			return v
		}
		if v.Instr != nil && v.Instr.Op == ir.OpAddrOf && len(v.Instr.Args) == 1 {
			v = v.Instr.Args[0]
			continue
		}
		return nil
	}
	return nil
}

// New creates an engine. graph may be nil: calls through interfaces and
// function values are then reported as unresolved.
func New(p *sg.Program, texts ir.Texts, st *sites.Result, rules *model.Rules, opts Options) *Engine {
	if opts.K <= 0 {
		opts.K = DefaultOptions().K
	}
	if opts.DepK < 0 {
		opts.DepK = 0
	}
	if opts.MaxPasses <= 0 {
		opts.MaxPasses = DefaultOptions().MaxPasses
	}
	if opts.Budget <= 0 {
		opts.Budget = DefaultOptions().Budget
	}
	if opts.FuncTimeout <= 0 {
		opts.FuncTimeout = DefaultOptions().FuncTimeout
	}
	// the loaders have checked the rules (Rules.Validate, after the expansion too): what is wrong there
	// costs its own symbol, not the alias models of the others
	aliasArgs, _ := rules.AliasArgs()
	return &Engine{
		prog: p, texts: texts, sites: st, rules: rules, opts: opts,
		summaries:  map[*sg.Func]*Summary{},
		states:     map[*sg.Func]*state{},
		degraded:   map[*sg.Func]uint8{},
		spent:      map[*sg.Func]time.Duration{},
		guardLimit: map[*sg.Func]bool{},
		coarsened:  map[*sg.Func]string{},
		globals:    newGlobalStore(),
		aliasArgs:  aliasArgs,
	}
}

func (e *Engine) funcID(fn *sg.Func) string { return fn.ID }

// SetLoadErrors tells the engine what the loading of the code could not see:
// they become missing_source uncertainties.
func (e *Engine) SetLoadErrors(errs []ir.LoadError) { e.loadErrors = errs }

// errorFunctions are the functions that the load says hold a compile error (a load error with Func), with the first
// message and the number of them.
func (e *Engine) errorFunctions() map[string]string {
	e.loadOnce.Do(e.indexLoadErrors)
	return e.errFuncs
}

func (e *Engine) indexLoadErrors() {
	e.recovered = map[string]string{}
	for _, le := range e.loadErrors {
		if le.Unread && strings.HasPrefix(le.Message, ir.SyntaxErrorPrefix) {
			e.recovered[le.Package] = le.Message
		}
	}
	e.errFuncs = map[string]string{}
	count := map[string]int{}
	for _, le := range e.loadErrors {
		if le.Func == "" {
			continue
		}
		count[le.Func]++
		if _, ok := e.errFuncs[le.Func]; !ok {
			e.errFuncs[le.Func] = le.Message
		}
	}
	for fn, n := range count {
		if n > 1 {
			e.errFuncs[fn] += " (" + itoa(n) + " errors in the function)"
		}
	}
}

// recoveredFiles are the files that the load says were read on a tree that is not what their text says (a syntax
// error: the load error is unread, its package is the file, and its message starts with "syntax_error:"), with the
// message.
func (e *Engine) recoveredFiles() map[string]string {
	e.loadOnce.Do(e.indexLoadErrors)
	return e.recovered
}

// numericStopRecords turns what the analyses of the functions saw into one record for each
// source site. A stop that happens inside a callee, on the symbolic data of its parameter, is
// not seen: it is seen only when the callee is given the data itself.
func (e *Engine) numericStopRecords() []*Uncertainty {
	type agg struct {
		symbol, label string
		sites         map[string]bool
		funcs         []string
		first         ir.Pos
		fn            string
	}
	by := map[string]*agg{}
	fns := make([]*sg.Func, 0, len(e.states))
	for fn := range e.states {
		fns = append(fns, fn)
	}
	sort.Slice(fns, func(i, j int) bool {
		if fns[i].Dep != fns[j].Dep {
			return !fns[i].Dep // the code of the project first
		}
		return fns[i].ID < fns[j].ID
	})
	for _, fn := range fns {
		stops := e.states[fn].numericStops
		origins := make([]string, 0, len(stops))
		for o := range stops {
			origins = append(origins, o)
		}
		sort.Strings(origins)
		for _, origin := range origins {
			st := stops[origin]
			symbol := ""
			if site := e.sites.ByID[origin]; site != nil {
				symbol = site.Symbol
			}
			// One record for each kind of source (a request type is a source in every function that takes
			// one: a record for each of them says the same thing a hundred times)
			key := symbol + "|" + st.label
			a := by[key]
			if a == nil {
				a = &agg{symbol: symbol, label: st.label, sites: map[string]bool{}, first: st.pos, fn: e.funcID(fn)}
				by[key] = a
			}
			a.sites[origin] = true
			if len(a.funcs) < 10 {
				a.funcs = append(a.funcs, e.funcID(fn))
			}
		}
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []*Uncertainty
	for _, k := range keys {
		a := by[k]
		u := newUncertainty(KindNumericStop, "numeric_stop: the data of this source (label "+a.label+", "+itoa(len(a.sites))+" source sites) became a number or a bool, "+
			"which are not followed (-numeric-taint follows them); a sink that takes a size or a count is not reached by it. A stop inside a function "+
			"that is given the data only through its parameters is not recorded here", a.symbol)
		u.Func = a.fn
		u.pos = a.first
		u.Candidates = a.funcs
		e.fill(u)
		out = append(out, u)
	}
	return out
}

func (e *Engine) noteCoarsened(fn *sg.Func, truncated, widened bool) {
	why := ""
	switch {
	case truncated && widened:
		why = "access paths cut at depth k, and the cells of a parameter merged"
	case truncated:
		why = "access paths cut at depth k"
	default:
		why = "the cells of a parameter merged into the whole of it"
	}
	e.mu.Lock()
	e.coarsened[fn] = why
	e.mu.Unlock()
}

func (e *Engine) noteGuardLimit(fn *sg.Func) {
	e.mu.Lock()
	e.guardLimit[fn] = true
	e.mu.Unlock()
}

// summaryOf is the summary of a function that has been analysed, or nil.
func (e *Engine) summaryOf(fn *sg.Func) *Summary {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.summaries[fn]
}

// globalWrite and globalReader wait for the end of the level (see mu).
type globalWrite struct {
	g    *sg.Value
	path string
	e    Elem
	// steps are the steps the writer took to put e there (nil when the store has it)
	steps *rope
	// fn is the function that wrote it (its ID): the order of the writes of a level
	// is made from it, not from the order the goroutines finished in.
	fn string
}

type globalReader struct {
	g  *sg.Value
	fn *sg.Func
}

// sanitizes reports whether the summary has a flow that passes a sanitizer
// or carries a label of a source.
func sanitizes(sum *Summary) bool {
	for _, o := range sum.Outputs {
		for _, oe := range o.Elems {
			if oe.E.Kind == Real || oe.E.Mods() != "" {
				return true
			}
		}
	}
	for _, sr := range sum.Sinks {
		if sr.Elem.Mods() != "" {
			return true
		}
	}
	return false
}

func (e *Engine) noteNotConverged(fn *sg.Func) {
	id := e.funcID(fn)
	e.mu.Lock()
	if !e.notConvergedSet[id] {
		if e.notConvergedSet == nil {
			e.notConvergedSet = map[string]bool{}
		}
		e.notConvergedSet[id] = true
		e.notConverged = append(e.notConverged, id)
	}
	e.mu.Unlock()
}

// Stats describes a run.
type Stats struct {
	Functions     int   `json:"functions"`
	Passes        int   `json:"passes"`
	Converged     bool  `json:"converged"`
	Reanalyses    int   `json:"reanalyses"`
	DurationMilli int64 `json:"duration_ms"`
	// DepSpentMilli and OwnSpentMilli are the time spent analysing the functions of the dependencies and of the code itself (the sum over all workers).
	// The shape of the call graph: the levels of components, the size of the
	// largest component (functions that call each other, analysed one after the
	// other) and the most components in one level.
	Levels        int      `json:"levels,omitempty"`
	LargestSCC    int      `json:"largest_scc,omitempty"`
	WidestLevel   int      `json:"widest_level,omitempty"`
	DepSpentMilli int64    `json:"dep_spent_ms,omitempty"`
	OwnSpentMilli int64    `json:"own_spent_ms,omitempty"`
	NotConverged  []string `json:"not_converged,omitempty"`
	TimedOut      bool     `json:"timed_out,omitempty"`
	// TestsSkipped is what was not read because the build does not ship it (ADR-062); absent when nothing was left out.
	TestsSkipped *ir.TestsSkipped `json:"tests_skipped,omitempty"`
	// DroppedGlobals are the package variables that a rule says hold nothing (ADR-056), and the number of functions
	// that wrote to them.
	DroppedGlobals []DroppedGlobal `json:"dropped_globals,omitempty"`
	// IdentityGlobalStores is the number of closures given away whose write of a global was left out: it writes back
	// the value read from the same global, whole, which adds nothing (ADR-059).
	IdentityGlobalStores int `json:"identity_global_stores,omitempty"`
	// PassChanges and UnmodeledTop are the diagnostics of -pass-stats (ADR-061): what changed in the summaries in each
	// pass, and the functions without a model that tainted values reached the most.
	PassChanges  []PassChange      `json:"pass_changes,omitempty"`
	UnmodeledTop []UnmodeledSymbol `json:"unmodeled_top,omitempty"`
	// PackagesTop and DispatchCandidates are the same kind (ADR-059): where the work went, by package, and how many
	// functions the dynamic calls may reach.
	PackagesTop []PackageStat `json:"packages_top,omitempty"`
	// LargestSCCPackages is the packages of the functions of the largest component, the 10 with the most (-pass-stats).
	// It is made from the call graph before the analysis starts, so a run that timed out has it too.
	LargestSCCPackages []SCCPackage `json:"largest_scc_packages,omitempty"`
	// SummarySizes is how the elements of the summaries are spread over the functions, and what they are made of (-pass-stats).
	SummarySizes       *SummarySizes       `json:"summary_sizes,omitempty"`
	DispatchCandidates *DispatchCandidates `json:"dispatch_candidates,omitempty"`
	// DispatchNotNarrowed counts the calls through Fn::call (Rust) that have several candidates and kept every
	// one, by why: the callers of a function may not all be read, the receiver comes from something that is not
	// followed, or a limit was reached. Absent when there was none (a run of another language).
	DispatchNotNarrowed *NotNarrowed `json:"dispatch_not_narrowed,omitempty"`
	// Unstable lists (up to ten) functions whose summary still changed in
	// the last pass, when the run did not converge (empty when it converged); UnstableCount is
	// how many there were.
	Unstable      []string `json:"unstable,omitempty"`
	UnstableCount int      `json:"unstable_count,omitempty"`
	// BudgetExhausted, FuncTimeouts and RunTimeouts count the functions whose analysis was
	// stopped by the work limit (Budget: the same on any machine), by the time limit of one
	// function (-func-timeout: a clock) and by the time limit of the run (-timeout: a clock);
	// a function is counted once, under the clock when it was stopped by one.
	BudgetExhausted int `json:"budget_exhausted,omitempty"`
	FuncTimeouts    int `json:"func_timeouts,omitempty"`
	RunTimeouts     int `json:"run_timeouts,omitempty"`
	// FuncValues is what -resolve-func-values did (docs/DESIGN-callouts.md, 5.6); absent when the flag is off.
	FuncValues *FuncValuesStats `json:"func_values,omitempty"`
	// LoadDependent is true when the result depends on the load of the machine: a function was
	// stopped by a clock, or the run was.
	LoadDependent bool `json:"load_dependent,omitempty"`

	// Sizes of the summaries, to see where the time goes.
	SummaryElems   int    `json:"summary_elems"`
	SummarySteps   int    `json:"summary_steps"`
	LargestSummary string `json:"largest_summary,omitempty"`
	LargestElems   int    `json:"largest_summary_elems,omitempty"`
}

// Result is the outcome of a run.
type Result struct {
	Findings      []*Finding     `json:"findings"`
	Uncertainties []*Uncertainty `json:"uncertainties"`
	// Suppressions are the places where a model decided by an LLM removed
	// taint.
	Suppressions []*Suppression `json:"suppressions,omitempty"`
	Stats        Stats          `json:"stats"`
	// FuncCandidates is the result of the diagnostic Options.FuncValues (not in the output).
	FuncCandidates []FuncCandidates `json:"-"`
	// FuncDropped is, of the same run, the calls with a model that were given a function value that the model does not
	// say anything of (by symbol; not in the output).
	FuncDropped []FuncDropped `json:"-"`
}

// Run analyses every function with a body, callees before callers, until the
// summaries stop changing.
func (e *Engine) Run() *Result {
	start := time.Now()
	e.reach = e.prog.Reachable()
	e.reachCB = e.prog.ReachableWithCallbacks()
	fns := e.prog.Funcs
	callees := func(ci *sg.Instr) []*sg.Func {
		if ci.Op == ir.OpMakeClosure {
			// a closure that is not only called where it is made has its summary applied at the
			// place it is made (C-7): the function that makes it depends on it
			if ci.Escapes && ci.IR != nil {
				if f := e.prog.ByID[string(ci.IR.Func)]; f != nil {
					return []*sg.Func{f}
				}
			}
			return nil
		}
		// the functions that the callback rows of a model make concrete here (a library function that calls
		// what it was given): this function needs their summaries
		cb := e.callbackFuncs(ci)
		if len(cb) == 0 {
			if ci.StaticID != "" {
				if ci.Static != nil {
					return []*sg.Func{ci.Static}
				}
				return nil
			}
			return ci.CandidateFns
		}
		if ci.StaticID != "" {
			if ci.Static != nil {
				return append([]*sg.Func{ci.Static}, cb...)
			}
			return cb
		}
		return append(append([]*sg.Func(nil), ci.CandidateFns...), cb...)
	}
	sccs, callers := order(fns, callees)
	levels := levelsOf(sccs, callees)
	st0 := Stats{Levels: len(levels)}
	for _, lv := range levels {
		if len(lv) > st0.WidestLevel {
			st0.WidestLevel = len(lv)
		}
		for _, scc := range lv {
			if len(scc) > st0.LargestSCC {
				st0.LargestSCC = len(scc)
				if e.opts.PassStats {
					e.largestSCC = scc
				}
			}
		}
	}
	e.writersOf(nil) // made now: the analyses read it together

	e.dirty = map[*sg.Func]bool{}
	for _, fn := range fns {
		e.dirty[fn] = true
	}
	st := st0
	st.Functions = len(fns)
	var deadline time.Time
	if e.opts.Timeout > 0 {
		deadline = start.Add(e.opts.Timeout)
		e.deadline = deadline
	}
	expired := func() bool { return !deadline.IsZero() && time.Now().After(deadline) }
	workers := e.opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	var timedOut atomic.Bool
	var reanalyses atomic.Int64
	var lastUnstable []string
	var lastChanged int
	for pass := 0; pass < e.opts.MaxPasses; pass++ {
		st.Passes = pass + 1
		e.pass = pass + 1
		var anyDone atomic.Bool
		var changedNow []string
		var changedMu sync.Mutex
		for _, level := range levels {
			// the components of a level do not call each other: they run
			// together, and what they write to the global state is merged
			// after them
			runLevel(level, workers, func(scc []*sg.Func) {
				if len(scc) >= parallelSCC {
					// A large component: its functions are analysed together, each
					// against the summaries the last round left, and published
					// in the order of the component after the round.
					// This is so for any number of workers (with one, the round runs one
					// function after the other): the output does not depend on -workers (#96).
					for iter := 0; iter < 8; iter++ {
						var todo []*sg.Func
						for _, fn := range scc {
							if expired() {
								timedOut.Store(true)
								break
							}
							if e.takeDirty(fn) {
								todo = append(todo, fn)
							}
						}
						if len(todo) == 0 {
							break
						}
						anyDone.Store(true)
						reanalyses.Add(int64(len(todo)))
						results := make([]*fnResult, len(todo))
						// the clock is looked at before each function: after the limit the rest of the round is not
						// computed (its result stays nil and is not published)
						runParallel(len(todo), workers, func(i int) {
							if expired() {
								timedOut.Store(true)
								return
							}
							results[i] = e.compute(todo[i])
						})
						progress := false
						for i, fn := range todo {
							if results[i] == nil {
								continue
							}
							if e.commit(fn, results[i], callers) {
								progress = true
								changedMu.Lock()
								changedNow = append(changedNow, e.funcID(fn))
								changedMu.Unlock()
							}
						}
						if !progress {
							break
						}
					}
					return
				}
				for iter := 0; iter < 8; iter++ {
					progress := false
					for _, fn := range scc {
						if expired() {
							timedOut.Store(true)
							break
						}
						if !e.takeDirty(fn) {
							continue
						}
						anyDone.Store(true)
						reanalyses.Add(1)
						if e.analyzeFunc(fn, callers) {
							progress = true
							changedMu.Lock()
							changedNow = append(changedNow, e.funcID(fn))
							changedMu.Unlock()
						}
					}
					if !progress || len(scc) == 1 && !selfCalls(scc[0], callees) {
						break
					}
				}
			})
			e.commitGlobals()
		}
		sort.Strings(changedNow)
		lastChanged = len(changedNow)
		if len(changedNow) > 10 {
			changedNow = changedNow[:10]
		}
		lastUnstable = changedNow
		if timedOut.Load() {
			st.TimedOut = true
			break
		}
		if !anyDone.Load() {
			st.Converged = true
			break
		}
		if !e.anyDirty(e.dirty) {
			st.Converged = true
			break
		}
	}
	st.Reanalyses = int(reanalyses.Load())
	st.DroppedGlobals = e.droppedGlobals()
	e.identityStores.Range(func(_, _ any) bool { st.IdentityGlobalStores++; return true })
	// the functions that changed in the last pass say what is not settled, and only a run that did not
	// converge has any (a run that converged may have changed them in its last pass: that is settling)
	if !st.Converged {
		st.Unstable, st.UnstableCount = lastUnstable, lastChanged
	}
	sort.Strings(e.notConverged)
	st.NotConverged = e.notConverged
	st.PassChanges = e.passChanges
	for _, sum := range e.summaries {
		n, steps := 0, 0
		for _, o := range sum.Outputs {
			n += len(o.Elems)
			for _, oe := range o.Elems {
				steps += oe.Steps.len()
			}
		}
		st.SummaryElems += n
		st.SummarySteps += steps
		// The summaries are in a map: ties go to the smaller name, so that
		// the same code always gives the same answer.
		if n > st.LargestElems || (n == st.LargestElems && n > 0 && sum.Func < st.LargestSummary) {
			st.LargestElems, st.LargestSummary = n, sum.Func
		}
	}
	var depSpent, ownSpent time.Duration
	for fn, d := range e.spent {
		if fn.Dep {
			depSpent += d
		} else {
			ownSpent += d
		}
	}
	st.DepSpentMilli, st.OwnSpentMilli = depSpent.Milliseconds(), ownSpent.Milliseconds()
	for _, d := range e.degraded {
		switch {
		case d&degFuncTimeout != 0:
			st.FuncTimeouts++
		case d&degRunTimeout != 0:
			st.RunTimeouts++
		default:
			st.BudgetExhausted++
		}
	}
	// What stopped on the clock depends on the load of the machine: the same run on a quieter
	// one may find more (ADR-054).
	st.LoadDependent = st.FuncTimeouts > 0 || st.RunTimeouts > 0 || st.TimedOut
	st.DurationMilli = time.Since(start).Milliseconds()
	return e.collect(st)
}

// parallelSCC is the size from which the functions of a strongly connected
// component are analysed together.
const parallelSCC = 8

// takeDirty says whether the function is to be analysed (it is marked, and
// not given up), and clears the mark.
func (e *Engine) takeDirty(fn *sg.Func) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.dirty[fn]
	e.dirty[fn] = false
	return d && e.degraded[fn] == 0
}

// runLevel runs f on the components, at most workers at a time, and waits.
func runLevel(level [][]*sg.Func, workers int, f func(scc []*sg.Func)) {
	runParallel(len(level), workers, func(i int) { f(level[i]) })
}

// runParallel runs f(0) ... f(n-1), at most workers at a time, and waits.
func runParallel(n, workers int, f func(i int)) {
	if workers <= 1 || n == 1 {
		for i := 0; i < n; i++ {
			f(i)
		}
		return
	}
	if workers > n {
		workers = n
	}
	var wg sync.WaitGroup
	var next atomic.Int64
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= n {
					return
				}
				f(i)
			}
		}()
	}
	wg.Wait()
}

// levelsOf groups the components (callees first) into levels: a component is
// one level above the highest component it calls, so the components of a level
// do not depend on each other, and a level depends only on the ones before it.
func levelsOf(sccs [][]*sg.Func, callees func(*sg.Instr) []*sg.Func) [][][]*sg.Func {
	of := map[*sg.Func]int{}
	for i, scc := range sccs {
		for _, fn := range scc {
			of[fn] = i
		}
	}
	level := make([]int, len(sccs))
	top := 0
	for i, scc := range sccs {
		for _, fn := range scc {
			for _, b := range fn.Blocks {
				for _, in := range b.Instrs {
					if in.Op != ir.OpCall && in.Op != ir.OpMakeClosure {
						continue
					}
					for _, c := range callees(in) {
						if j, ok := of[c]; ok && j != i && level[j]+1 > level[i] {
							level[i] = level[j] + 1
						}
					}
				}
			}
		}
		if level[i] > top {
			top = level[i]
		}
	}
	out := make([][][]*sg.Func, top+1)
	for i, scc := range sccs {
		out[level[i]] = append(out[level[i]], scc)
	}
	return out
}

// commitGlobals merges what the functions of the level wrote to the global
// state, and marks the functions that read a global that grew.
func (e *Engine) commitGlobals() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range e.pendingReaders {
		e.globals.noteReader(r.g, r.fn)
	}
	// The components of a level are analysed at the same time and commit in the
	// order they finish: the writes are put in a fixed order (cell, path, element,
	// writer), so that the writer a path starts at (noteWriter: the first) is the
	// same on every run (ADR-049). Which writer it is has no meaning.
	sort.SliceStable(e.pendingGlobals, func(i, j int) bool { return lessGlobalWrite(e.pendingGlobals[i], e.pendingGlobals[j]) })
	for _, w := range e.pendingGlobals {
		e.globals.noteWriterFn(w.g, w.path, w.e, w.fn)
		if e.globals.merge(w.g, w.path, w.e) {
			e.globals.noteWriter(w.g, w.path, w.e, w.steps)
			e.traceGlobal(w.g, w.path, w.e)
			for _, other := range e.globals.readers(w.g) {
				e.dirty[other] = true
			}
		}
	}
	e.pendingGlobals, e.pendingReaders = e.pendingGlobals[:0], e.pendingReaders[:0]
}

// lessGlobalWrite orders the writes to the global state: by cell (the name; the cells
// of one name are the same global), path, element, and the ID of the writing function.
func lessGlobalWrite(a, b globalWrite) bool {
	if a.g != b.g && a.g.Name != b.g.Name {
		return a.g.Name < b.g.Name
	}
	if a.path != b.path {
		return a.path < b.path
	}
	if a.e != b.e {
		if lessElem(a.e, b.e) {
			return true
		}
		if lessElem(b.e, a.e) {
			return false
		}
	}
	return a.fn < b.fn
}

func (e *Engine) anyDirty(dirty map[*sg.Func]bool) bool {
	for _, d := range dirty {
		if d {
			return true
		}
	}
	return false
}

func selfCalls(fn *sg.Func, callees func(*sg.Instr) []*sg.Func) bool {
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			if in.Op == ir.OpCall {
				for _, c := range callees(in) {
					if c == fn {
						return true
					}
				}
			}
		}
	}
	return false
}

// analyzeFunc analyses one function and reports whether its summary or the
// global state changed. Dependents are marked dirty.
func (e *Engine) analyzeFunc(fn *sg.Func, callers map[*sg.Func][]*sg.Func) bool {
	return e.commit(fn, e.compute(fn), callers)
}

// fnResult is what the analysis of a function leaves, before it is published.
type fnResult struct {
	s       *state
	sum     *Summary
	took    time.Duration
	changed bool // the global state would grow
	writes  []globalWrite
	reads   []globalReader
}

// compute analyses a function. It writes nothing that another function reads:
// it reads the summaries of the callees and the global state as they are.
func (e *Engine) compute(fn *sg.Func) *fnResult {
	s := &state{
		eng: e, fn: fn,
		cells:       map[Root]map[string]Set{},
		paramRoot:   map[Root]int{},
		findings:    map[findingKey]*rawFinding{},
		sinkReaches: map[sinkKey]*SinkReach{},
	}
	var cpu0 time.Duration
	var haveCPU bool
	if e.opts.SlowFunc != nil {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		cpu0, haveCPU = threadCPU()
	}
	began := time.Now()
	s.analyze()
	took := time.Since(began)
	// the summary is joined with the one of the earlier visits
	sum := joinSummaries(e.summaryOf(fn), s.summarize())
	// Too many guards matter only where taint is told apart by them: where
	// something is sanitized, or a source's own label is returned.
	if s.guardOverflow && sanitizes(sum) {
		e.noteGuardLimit(fn)
	}
	if s.truncated || s.widened {
		e.noteCoarsened(fn, s.truncated, s.widened)
	}
	if e.opts.SlowFunc != nil {
		if d := time.Since(began); d > e.opts.SlowThreshold {
			var cpu time.Duration
			if haveCPU {
				if c1, ok := threadCPU(); ok {
					cpu = c1 - cpu0
				}
			}
			e.opts.SlowFunc(e.funcID(fn), d, cpu, len(sum.Outputs), len(s.cells))
		}
	}
	// What the function writes to global variables waits for the end of the
	// level; whether it adds anything is read from the global state, which
	// does not change while a level runs.
	r := &fnResult{s: s, sum: sum, took: took}
	for root, cm := range s.cells {
		g := root.V
		if g == nil || g.Kind != sg.KGlobal {
			continue
		}
		if e.rules.HoldsNothing(g.Name) {
			continue // what is written to it is read back by nobody (ADR-056): not stored, not read
		}
		r.reads = append(r.reads, globalReader{g, fn})
		for path, set := range cm {
			for el := range set {
				if el.Kind != Real {
					continue
				}
				if set[el] == nil {
					continue // read from the global state, not produced here
				}
				gw := globalWrite{g: g, path: path, e: el, fn: e.funcID(fn)}
				if e.tracing(el) {
					s.traceWrite(fn, g, root, path, el)
				}
				if !e.globals.has(g, path, el) {
					r.changed = true
					// the steps the writer took, for the path of whoever reads it
					gw.steps = s.trace(Loc{root, path}, el)
				}
				r.writes = append(r.writes, gw)
			}
		}
	}
	return r
}

// commit publishes the result of the analysis of a function, and reports
// whether its summary or the global state changed. Dependents are marked.
func (e *Engine) commit(fn *sg.Func, r *fnResult, callers map[*sg.Func][]*sg.Func) bool {
	s, sum, changed := r.s, r.sum, r.changed
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spent[fn] += r.took
	e.notePackage(fn, r.took)
	if s.exceeded {
		e.degraded[fn] |= s.exceededBy
	}
	// A function that keeps costing time (its callees' summaries change and
	// it is visited again) is not visited again once it has used what one
	// analysis is allowed (four times the limit of one): its summary may miss flows,
	// and says so.
	if e.spent[fn] > 4*e.opts.FuncTimeout {
		e.degraded[fn] |= degFuncTimeout
	}
	if old := e.summaries[fn]; old == nil || old.Sig != sum.Sig {
		e.notePassChange(fn, old, sum)
		e.traceSummary(fn, sum)
		changed = true
		for _, c := range callers[fn] {
			e.dirty[c] = true
		}
	}
	e.summaries[fn] = sum
	e.pendingGlobals = append(e.pendingGlobals, r.writes...)
	e.pendingReaders = append(e.pendingReaders, r.reads...)
	// What the result needs of the analysis is the findings, the
	// suppressions and the uncertainties. The cells and the caches are the
	// bulk of the memory, and the function is analysed afresh if it is
	// visited again.
	// A visit that was cut short (the budget, the time limit) saw part of the
	// function: it does not take away what an earlier visit found. A complete
	// visit replaces the earlier one, as before.
	if prev := e.states[fn]; s.exceeded && prev != nil {
		keepEarlier(s, prev)
	}
	e.states[fn] = &state{eng: e, fn: fn, findings: s.findings, suppressions: s.suppressions, uncertain: s.uncertain, numericStops: s.numericStops}
	return changed
}

// order computes the strongly connected components of the call graph with
// callees before callers, and the reverse edges.
func order(fns []*sg.Func, callees func(*sg.Instr) []*sg.Func) ([][]*sg.Func, map[*sg.Func][]*sg.Func) {
	inSet := map[*sg.Func]bool{}
	for _, fn := range fns {
		inSet[fn] = true
	}
	succ := map[*sg.Func][]*sg.Func{}
	callers := map[*sg.Func][]*sg.Func{}
	for _, fn := range fns {
		seen := map[*sg.Func]bool{}
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if in.Op != ir.OpCall && in.Op != ir.OpMakeClosure {
					continue
				}
				for _, c := range callees(in) {
					if inSet[c] && !seen[c] {
						seen[c] = true
						succ[fn] = append(succ[fn], c)
						callers[c] = append(callers[c], fn)
					}
				}
			}
		}
	}
	// Tarjan's algorithm yields components in reverse topological order,
	// that is, callees first.
	index := map[*sg.Func]int{}
	low := map[*sg.Func]int{}
	on := map[*sg.Func]bool{}
	var stack []*sg.Func
	var sccs [][]*sg.Func
	n := 0
	var visit func(v *sg.Func)
	visit = func(v *sg.Func) {
		index[v], low[v] = n, n
		n++
		stack = append(stack, v)
		on[v] = true
		for _, w := range succ[v] {
			if _, seen := index[w]; !seen {
				visit(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if on[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}
		if low[v] == index[v] {
			var comp []*sg.Func
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			sort.Slice(comp, func(i, j int) bool { return comp[i].ID < comp[j].ID })
			sccs = append(sccs, comp)
		}
	}
	for _, fn := range fns {
		if _, seen := index[fn]; !seen {
			visit(fn)
		}
	}
	return sccs, callers
}

// --- globals ---

type globalStore struct {
	cells map[*sg.Value]map[string]Set
	// writers holds, for each element of a cell, the steps of the function that
	// wrote it first (CORE-B): a reader's path starts there, not at its load.
	writers  map[*sg.Value]map[string]map[Elem]*rope
	readers_ map[*sg.Value]map[*sg.Func]bool
	// writerFns holds, for each element of a cell, the functions that wrote it (all of them,
	// not only the first: the evidence of a finding, Finding.SharedState).
	writerFns map[*sg.Value]map[string]map[Elem][]string
}

// noteWriterFn records that the function wrote the element to the cell.
func (g *globalStore) noteWriterFn(v *sg.Value, path string, e Elem, fn string) {
	if g.writerFns == nil {
		g.writerFns = map[*sg.Value]map[string]map[Elem][]string{}
	}
	pm := g.writerFns[v]
	if pm == nil {
		pm = map[string]map[Elem][]string{}
		g.writerFns[v] = pm
	}
	em := pm[path]
	if em == nil {
		em = map[Elem][]string{}
		pm[path] = em
	}
	for _, f := range em[e] {
		if f == fn {
			return
		}
	}
	em[e] = append(em[e], fn)
}

func newGlobalStore() *globalStore {
	return &globalStore{cells: map[*sg.Value]map[string]Set{}, readers_: map[*sg.Value]map[*sg.Func]bool{}}
}

func (g *globalStore) noteWriter(v *sg.Value, path string, e Elem, steps *rope) {
	if steps == nil {
		return
	}
	if g.writers == nil {
		g.writers = map[*sg.Value]map[string]map[Elem]*rope{}
	}
	pm := g.writers[v]
	if pm == nil {
		pm = map[string]map[Elem]*rope{}
		g.writers[v] = pm
	}
	em := pm[path]
	if em == nil {
		em = map[Elem]*rope{}
		pm[path] = em
	}
	em[e] = steps
}

// writerSteps returns the steps of the function that first wrote the element to the cell.
func (g *globalStore) writerSteps(v *sg.Value, path string, e Elem) *rope {
	return g.writers[v][path][e]
}

func (g *globalStore) cellsOf(v *sg.Value) map[string]Set { return g.cells[v] }

// has reports whether the element is in the cell already.
func (g *globalStore) has(v *sg.Value, path string, e Elem) bool {
	_, ok := g.cells[v][path][e]
	return ok
}

func (g *globalStore) merge(v *sg.Value, path string, e Elem) bool {
	cm := g.cells[v]
	if cm == nil {
		cm = map[string]Set{}
		g.cells[v] = cm
	}
	set := cm[path]
	if set == nil {
		set = Set{}
		cm[path] = set
	}
	if _, ok := set[e]; ok {
		return false
	}
	set[e] = nil
	return true
}

func (g *globalStore) noteReader(v *sg.Value, fn *sg.Func) {
	m := g.readers_[v]
	if m == nil {
		m = map[*sg.Func]bool{}
		g.readers_[v] = m
	}
	m[fn] = true
}

func (g *globalStore) readers(v *sg.Value) []*sg.Func {
	var out []*sg.Func
	for fn := range g.readers_[v] {
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// --- results ---

func (e *Engine) collect(st Stats) *Result {
	res := &Result{Stats: st, Findings: []*Finding{}, Uncertainties: []*Uncertainty{}}

	type best struct {
		rf *rawFinding
	}
	merged := map[findingKey]*rawFinding{}
	fnList := make([]*sg.Func, 0, len(e.states))
	for fn := range e.states {
		fnList = append(fnList, fn)
	}
	sort.Slice(fnList, func(i, j int) bool { return fnList[i].ID < fnList[j].ID })

	seenU := map[string]bool{}
	for _, fn := range fnList {
		s := e.states[fn]
		keys := make([]findingKey, 0, len(s.findings))
		for k := range s.findings {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].origin != keys[j].origin {
				return keys[i].origin < keys[j].origin
			}
			return keys[i].sink < keys[j].sink
		})
		for _, k := range keys {
			rf := s.findings[k]
			cur := merged[k]
			if cur == nil {
				cp := *rf
				cp.labels = map[string]bool{}
				for l := range rf.labels {
					cp.labels[l] = true
				}
				merged[k] = &cp
				continue
			}
			for l := range rf.labels {
				cur.labels[l] = true
			}
			if rf.steps.len() < cur.steps.len() {
				cur.steps = rf.steps
				cur.guards = rf.guards
			}
		}
		if fn.Dep {
			continue
		}
		for _, sp := range s.suppressions {
			sp.Position = position(sp.pos)
			sp.Snippet = e.texts.Snippet(sp.pos)
			res.Suppressions = append(res.Suppressions, sp)
		}
		for _, u := range s.uncertain {
			key := u.Kind + "|" + u.Symbol + "|" + u.Func + "|" + posKey(u.pos)
			if seenU[key] {
				continue
			}
			seenU[key] = true
			e.fill(u)
			res.Uncertainties = append(res.Uncertainties, u)
		}
	}
	for _, pe := range e.loadErrors {
		u := newUncertainty(KindMissingSource, pe.Message, pe.Package)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	var limited []*sg.Func
	for fn := range e.guardLimit {
		limited = append(limited, fn)
	}
	sort.Slice(limited, func(i, j int) bool { return limited[i].ID < limited[j].ID })
	for _, fn := range limited {
		u := newUncertainty(KindDepthLimit, "guard_limit: a flow in this function depends on more than "+itoa(maxGuards)+
			" conditions on parameters; it is kept without them, as possible under any condition", "")
		u.Func = e.funcID(fn)
		u.pos = fn.Pos
		e.fill(u)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	// the functions whose cells are coarser than the code: one record, with the list
	if len(e.coarsened) > 0 {
		ids := make([]string, 0, len(e.coarsened))
		for fn := range e.coarsened {
			ids = append(ids, e.funcID(fn))
		}
		sort.Strings(ids)
		u := newUncertainty(KindDepthLimit, "path_limit: in "+itoa(len(ids))+" functions the access paths were cut at depth "+itoa(e.opts.K)+
			", or the cells of a parameter were merged into the whole of it: what is told apart in the code is not told apart here (more flows than there are, never fewer)", "")
		u.Candidates = ids
		res.Uncertainties = append(res.Uncertainties, u)
	}
	// where the data of a source became a number or a bool: one record for each source site
	if stops := e.numericStopRecords(); len(stops) > 0 {
		res.Uncertainties = append(res.Uncertainties, stops...)
	}
	// the package variables that a rule says hold nothing, and that functions wrote to: one record for each variable
	res.Uncertainties = append(res.Uncertainties, e.heldNothingRecords()...)
	var degraded []*sg.Func
	for fn := range e.degraded {
		degraded = append(degraded, fn)
	}
	sort.Slice(degraded, func(i, j int) bool { return degraded[i].ID < degraded[j].ID })
	for _, fn := range degraded {
		u := newUncertainty(KindDepthLimit, degradedDetail(e.degraded[fn], e.opts), "")
		u.Func = e.funcID(fn)
		u.pos = fn.Pos
		e.fill(u)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	if len(e.notConverged) > 0 {
		u := newUncertainty(KindRecursionApproximation, "the local analysis of these functions did not reach a fixed point", "")
		u.Candidates = append([]string(nil), e.notConverged...)
		res.Uncertainties = append(res.Uncertainties, u)
	}
	switch {
	case st.TimedOut:
		res.Uncertainties = append(res.Uncertainties, newUncertainty(KindDepthLimit,
			"the time limit was reached; functions not analysed yet are missing from the result", ""))
	case !st.Converged:
		res.Uncertainties = append(res.Uncertainties, newUncertainty(KindRecursionApproximation, "the summaries did not stabilise within the pass limit", ""))
	}
	sort.SliceStable(res.Suppressions, func(i, j int) bool {
		a, b := res.Suppressions[i], res.Suppressions[j]
		if a.Position.File != b.Position.File {
			return a.Position.File < b.Position.File
		}
		return a.Position.Offset < b.Position.Offset
	})
	sort.SliceStable(res.Uncertainties, func(i, j int) bool {
		a, b := res.Uncertainties[i], res.Uncertainties[j]
		if a.Position.File != b.Position.File {
			return a.Position.File < b.Position.File
		}
		if a.Position.Offset != b.Position.Offset {
			return a.Position.Offset < b.Position.Offset
		}
		return a.Kind < b.Kind
	})

	if e.opts.FuncValues {
		res.FuncCandidates = e.collectFuncCandidates(merged)
		res.FuncDropped = e.droppedAtModel()
	}
	e.suppressions = res.Suppressions
	keys := make([]findingKey, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	for _, k := range keys {
		f := e.buildFinding(merged[k])
		if f != nil {
			res.Findings = append(res.Findings, f)
		}
	}
	res.Findings = e.mergeParamSources(res.Findings)
	sort.SliceStable(res.Findings, func(i, j int) bool {
		a, b := res.Findings[i], res.Findings[j]
		if a.Sink.Position.File != b.Sink.Position.File {
			return a.Sink.Position.File < b.Sink.Position.File
		}
		if a.Sink.Position.Offset != b.Sink.Position.Offset {
			return a.Sink.Position.Offset < b.Sink.Position.Offset
		}
		return a.ID < b.ID
	})
	if nr := e.prog.IR.Narrowing; nr != nil && (nr.OpenCallers+nr.UnknownOrigin+nr.Limit > 0) {
		res.Stats.DispatchNotNarrowed = &NotNarrowed{OpenCallers: nr.OpenCallers, UnknownOrigin: nr.UnknownOrigin, Limit: nr.Limit}
	}
	if e.opts.PassStats {
		res.Stats.UnmodeledTop = e.unmodeledTop(res.Findings)
		res.Stats.PackagesTop = e.packagesTop()
		res.Stats.SummarySizes = e.summarySizes()
		res.Stats.LargestSCCPackages = e.largestSCCPackages()
		res.Stats.DispatchCandidates = e.dispatchCandidates()
	}
	return res
}

func (e *Engine) fill(u *Uncertainty) {
	if !u.pos.IsZero() {
		u.Position = position(u.pos)
		u.Snippet = e.texts.Snippet(u.pos)
	}
}

// position converts a position of the IR to the one the output has.
func position(p ir.Pos) ir.Position { return p.Position() }

func (e *Engine) siteRef(s *sites.Site) SiteRef {
	return SiteRef{ID: s.ID, Symbol: s.Symbol, Func: s.Func, Position: s.Position, Snippet: s.Snippet, Fingerprint: s.Fingerprint}
}

func (e *Engine) buildFinding(rf *rawFinding) *Finding {
	src := e.sites.ByID[rf.origin]
	if src == nil || rf.site == nil {
		return nil
	}
	f := &Finding{
		Source:     e.siteRef(src),
		Sink:       e.siteRef(rf.site),
		Category:   rf.site.Category,
		Categories: rf.site.Categories,
		Confidence: ConfidenceReview,
	}
	for l := range rf.labels {
		f.Labels = append(f.Labels, l)
	}
	sort.Strings(f.Labels)

	// Steps without a position (compiler generated hops) tell a reader
	// nothing, unless they carry an uncertainty.
	steps := make([]Step, 0, rf.steps.len())
	flat, omitted := rf.steps.flattenOmitted(maxPathSteps)
	for _, st := range flat {
		if !st.pos.IsZero() || len(st.Notes) > 0 {
			steps = append(steps, st)
		}
	}
	// What the path says (uncertainties, models, reachability) is read from every step, also from the ones
	// that a long path does not show (CORE-1): what is left out of the view is not left out of the facts.
	agg := steps
	if omitted > 0 {
		agg = nil
		full, _ := rf.steps.flattenOmitted(rf.steps.len())
		for _, st := range full {
			if !st.pos.IsZero() || len(st.Notes) > 0 {
				agg = append(agg, st)
			}
		}
	}
	// a function of a file that was read on a recovered tree (a syntax error: the extractor says so in a load error that
	// names the file): the finding is not a clear one
	if byFunc := e.errorFunctions(); len(byFunc) > 0 {
		ids := []string{src.Func, rf.site.Func}
		for i := range agg {
			ids = append(ids, agg[i].Func)
		}
		seenFn := map[string]bool{}
		for _, id := range ids {
			// a closure ("f$1") counts as the function around it (ir.ParentFuncIDs: the ID itself first, then the ones
			// around it, a trailing "$<digits>" at a time: a name with a "$" of its own is not cut)
			for _, base := range ir.ParentFuncIDs(id) {
				if msg, ok := byFunc[base]; ok {
					if !seenFn[base] {
						seenFn[base] = true
						f.Uncertainties = append(f.Uncertainties, newUncertainty(KindMissingSource, msg, base))
					}
					break
				}
			}
		}
	}
	if rec := e.recoveredFiles(); len(rec) > 0 {
		ids := []string{src.Func, rf.site.Func}
		for i := range agg {
			ids = append(ids, agg[i].Func)
		}
		seenRec := map[string]bool{}
		for _, id := range ids {
			fn := e.prog.ByID[id]
			if fn == nil || seenRec[fn.Pos.File] {
				continue
			}
			if msg, ok := rec[fn.Pos.File]; ok {
				seenRec[fn.Pos.File] = true
				f.Uncertainties = append(f.Uncertainties, newUncertainty(KindMissingSource, msg, fn.Pos.File))
			}
		}
	}
	var funcs []string // the steps that are shown (the fingerprint of the path)
	for i := range steps {
		st := &steps[i]
		if !st.pos.IsZero() {
			st.Position = position(st.pos)
			st.Snippet = e.texts.Snippet(st.pos)
		}
		funcs = append(funcs, st.Func)
	}
	var allFuncs []string
	seen := map[string]bool{}
	cuts := 0
	for i := range agg {
		st := &agg[i]
		allFuncs = append(allFuncs, st.Func)
		for _, n := range st.Notes {
			e.fill(n)
			k := n.Kind + "|" + n.Symbol + "|" + posKey(n.pos)
			if n.Kind == KindAccessPathTruncated {
				// each cut is told on its own (where and how deep), up to a cap
				k += "|" + n.Detail
				if !seen[k] {
					seen[k] = true
					if cuts++; cuts > maxCutNotes {
						continue
					}
					f.Uncertainties = append(f.Uncertainties, n)
				}
				continue
			}
			if !seen[k] {
				seen[k] = true
				f.Uncertainties = append(f.Uncertainties, n)
				if n.Kind == KindDynamicDispatch && len(n.Candidates) > 1 {
					f.DispatchWidened++
					if len(n.Candidates) > f.DispatchCandidates {
						f.DispatchCandidates = len(n.Candidates)
					}
				}
			}
		}
	}
	if cuts > maxCutNotes {
		f.Uncertainties = append(f.Uncertainties, newUncertainty(KindAccessPathTruncated, "access_path_truncated: "+itoa(cuts-maxCutNotes)+
			" more cuts on the path are not listed (the first "+itoa(maxCutNotes)+" are)", ""))
	}
	f.Paths = []Path{{Steps: steps, OmittedSteps: omitted}}
	f.PathSteps = len(steps)
	if omitted > 0 {
		// the middle of a long path is not shown: say so (it is not a path that was read in full)
		f.Uncertainties = append(f.Uncertainties, newUncertainty(KindDepthLimit, "path_steps_omitted: "+itoa(omitted)+
			" steps in the middle of the path were left out (the first "+itoa(maxPathSteps/2)+" and the last "+itoa(maxPathSteps/2)+" are shown)", ""))
	}
	if e.reach != nil {
		f.Reachability = ReachFromEntry
		for _, id := range append([]string{src.Func, rf.site.Func}, allFuncs...) {
			if fn := e.prog.ByID[id]; fn != nil && !e.reach[fn] {
				if e.reachCB[fn] {
					f.Reachability = ReachFromFrameworkCallback
					f.ReachabilityNote = FrameworkCallbackNote
					continue
				}
				f.Reachability = ReachNotFromEntry
				f.ReachabilityNote = NotFromEntryNote
				break
			}
		}
	}
	if len(f.Uncertainties) == 0 {
		f.Confidence = ConfidenceClear
	}
	f.ConditionalNodes = e.conditionalNodes(rf.guards)
	f.SharedState = e.sharedState(rf.origin, f.Uncertainties)

	// Which models decided something on the path, and who decided them.
	seenModel := map[string]bool{}
	addModel := func(m ModelRef) {
		k := m.Role + "|" + m.Symbol + "|" + m.Origin
		if seenModel[k] {
			return
		}
		seenModel[k] = true
		f.Models = append(f.Models, m)
		if m.Origin == "llm" {
			f.LLMInfluenced = true
		}
	}
	for _, s := range src.Src {
		addModel(*modelRef(src.Symbol, "source", s.Provenance))
	}
	for _, st := range agg {
		if st.Model != nil {
			addModel(*st.Model)
		}
	}
	for _, k := range rf.site.Sink {
		addModel(*modelRef(rf.site.Symbol, "sink", k.Provenance))
	}

	// a model decided by an LLM that cut another way from this source, in a function of this path: the finding
	// stands, but what it shows was decided in part by that model
	for _, sp := range e.suppressions {
		if sp.Model.Origin != "llm" || !slices.Contains(sp.Origins, rf.origin) || !slices.Contains(allFuncs, sp.Func) {
			continue
		}
		addModel(sp.Model)
	}

	fp := fingerprint.Combine(src.Fingerprint, rf.site.Fingerprint)
	fp[fingerprint.Path] = fingerprint.Hash(append([]string{src.Func}, append(funcs, rf.site.Func)...)...)
	f.Fingerprint = fp
	f.ID = "f1:" + fp[fingerprint.Structural]
	return f
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// distinctSources keeps the first finding of each place of a source, in order: the sources at one place (the same
// parameter of the same function) are one source. The twin that a method value makes (`F$bound`) has the position of
// the function it binds, and is not another input.
func distinctSources(g []*Finding) []*Finding {
	seen := map[srcPlace]bool{}
	var out []*Finding
	for _, f := range g {
		p := srcPlace{f.Source.Position, f.Source.Symbol}
		if !seen[p] {
			seen[p] = true
			out = append(out, f)
		}
	}
	return out
}

// srcPlace is where a source is (its position and symbol): sources at one place are one source.
type srcPlace struct {
	pos ir.Position
	sym string
}

// maxOtherSources bounds the other sources a finding lists.
const maxOtherSources = 5

// maxCutNotes bounds the access_path_truncated notes a finding lists (each tells one cut).
const maxCutNotes = 10

// mergeParamSources merges the findings that differ only in the function the
// source parameter is in: a parameter of a request type is a source in every
// function that takes one, so a handler and the helpers it calls (and, when a
// function value is resolved to every candidate, other handlers) each give
// the same flow. The finding that is the best to look at first stays (reachable
// from where the program starts, with the fewest uncertainties and widened calls on
// its path, then the shortest path); the others are listed in it, each with what is known of
// its own path, and none of them has a path that is better by those measures.
func (e *Engine) mergeParamSources(fs []*Finding) []*Finding {
	type key struct{ sink, category, labels, symbol string }
	groups := map[key][]*Finding{}
	var order []key
	var out []*Finding
	for _, f := range fs {
		src := e.sites.ByID[f.Source.ID]
		if src == nil || (src.Kind != model.KindParam && src.Kind != model.KindParamType) {
			out = append(out, f)
			continue
		}
		k := key{f.Sink.ID, f.Category, strings.Join(f.Labels, ","), f.Source.Symbol}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], f)
	}
	for _, k := range order {
		g := groups[k]
		sort.SliceStable(g, func(i, j int) bool {
			a, b := g[i], g[j]
			// the framework callbacks do not count as reached here: they were not before they had a name, and the
			// representative path is the same (ADR-073)
			if ra, rb := a.Reachability != ReachNotFromEntry && a.Reachability != ReachFromFrameworkCallback, b.Reachability != ReachNotFromEntry && b.Reachability != ReachFromFrameworkCallback; ra != rb {
				return ra
			}
			if ua, ub := len(a.Uncertainties), len(b.Uncertainties); ua != ub {
				return ua < ub
			}
			if da, db := a.DispatchWidened, b.DispatchWidened; da != db {
				return da < db
			}
			if la, lb := len(a.Paths[0].Steps), len(b.Paths[0].Steps); la != lb {
				return la < lb
			}
			if a.Source.Func != b.Source.Func {
				return a.Source.Func < b.Source.Func
			}
			return a.ID < b.ID
		})
		rep := g[0]
		for _, o := range distinctSources(g)[1:] {
			rep.OtherSourcesCount++
			if len(rep.OtherSources) < maxOtherSources {
				ref := o.Source
				ref.Snippet = ""
				rep.OtherSources = append(rep.OtherSources, OtherSource{
					SiteRef: ref, PathSteps: o.PathSteps, Uncertainties: len(o.Uncertainties),
					DispatchWidened: o.DispatchWidened, DispatchCandidates: o.DispatchCandidates, Reachability: o.Reachability,
					ViaSharedState: sharedPlaceNames(o.SharedState),
				})
			}
		}
		out = append(out, rep)
	}
	return out
}

// sharedPlaceNames are the names of the package variables of a finding's shared_state (nil when it has none).
func sharedPlaceNames(ss []SharedState) []string {
	var out []string
	for _, s := range ss {
		if n := s.Place.Name; len(out) == 0 || out[len(out)-1] != n {
			out = append(out, n)
		}
	}
	return out
}

// keepEarlier adds to the state of a visit that was cut short what the earlier
// visit of the function found: a finding by its key (the labels of both, the
// shorter path with its guards, as when the functions' results are merged), the
// suppressions, the uncertainties and the places where data stopped at a number.
func keepEarlier(s, prev *state) {
	if s.findings == nil {
		s.findings = map[findingKey]*rawFinding{}
	}
	for k, pf := range prev.findings {
		cur := s.findings[k]
		if cur == nil {
			s.findings[k] = pf
			continue
		}
		cp := *cur
		cp.labels = map[string]bool{}
		for l := range cur.labels {
			cp.labels[l] = true
		}
		for l := range pf.labels {
			cp.labels[l] = true
		}
		if pf.steps.len() < cp.steps.len() {
			cp.steps = pf.steps
			cp.guards = pf.guards
		}
		s.findings[k] = &cp
	}
	for k, sp := range prev.suppressions {
		if s.suppressions == nil {
			s.suppressions = map[string]*Suppression{}
		}
		if _, ok := s.suppressions[k]; !ok {
			s.suppressions[k] = sp
		}
	}
	s.uncertain = append(s.uncertain, prev.uncertain...)
	for o, st := range prev.numericStops {
		if s.numericStops == nil {
			s.numericStops = map[string]numericStop{}
		}
		if _, ok := s.numericStops[o]; !ok {
			s.numericStops[o] = st
		}
	}
}

// The limits that stop the analysis of a function (Engine.degraded is a set of them). The work
// limit is the same on any machine; the two time limits are clocks.
const (
	degBudget      uint8 = 1 << iota // Options.Budget: the number of elements added
	degFuncTimeout                   // Options.FuncTimeout: the time of one function
	degRunTimeout                    // Options.Timeout: the time of the whole run
)

// degradedDetail is the text of the uncertainty of a function that a limit stopped. A clock is
// said before the work limit: it is the one that makes the result depend on the machine.
func degradedDetail(why uint8, o Options) string {
	switch {
	case why&degFuncTimeout != 0:
		return "func_timeout: the analysis of this function used up its time (" + o.FuncTimeout.String() + ", -func-timeout) or the time that its repeated analyses may use; its summary may miss flows, and which flows depends on the load of the machine: the same run on a quieter one may find more"
	case why&degRunTimeout != 0:
		return "run_timeout: the time limit of the whole run (-timeout) was reached while this function was analysed; its summary may miss flows, and which flows depends on the load of the machine"
	}
	return "budget: the analysis of this function added more than " + itoa(o.Budget) + " elements (the work limit); its summary may miss flows (the same on any machine)"
}

func (e *Engine) noteIdentityStore(in *sg.Instr) { e.identityStores.Store(in, struct{}{}) }

// sharedState is the evidence of a finding that goes through package variables (Finding.SharedState):
// for each variable (and path in it) the path read, the functions that wrote an element of the
// source of the finding there. It reads what the run left; it changes nothing.
func (e *Engine) sharedState(origin string, notes []*Uncertainty) []SharedState {
	type key struct {
		g    *sg.Value
		path string
	}
	var keys []key
	seen := map[key]bool{}
	for _, n := range notes {
		if n.Kind != KindGlobalVar || n.global == nil {
			continue
		}
		k := key{n.global, n.globalPath}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	var out []SharedState
	for _, k := range keys {
		flow := map[string]bool{}
		for p, em := range e.globals.writerFns[k.g] {
			if p != k.path && !strings.HasPrefix(p, k.path+"/") && !strings.HasPrefix(k.path, p+"/") {
				continue
			}
			for el, fns := range em {
				if el.Kind != Real || el.Origin() != origin {
					continue
				}
				for _, fn := range fns {
					flow[fn] = true
				}
			}
		}
		st := SharedState{
			Place:   SharedPlace{Kind: "global", Name: k.g.Name},
			Path:    k.path,
			Writers: SharedWriters{Flow: len(flow), All: len(e.writersOf(k.g))},
			Readers: len(e.globals.readers_[k.g]),
		}
		for id := range flow {
			if fn := e.prog.ByID[id]; fn != nil && inTestFile(position(fn.Pos).File) {
				st.Writers.Test++
			}
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Place.Name != out[j].Place.Name {
			return out[i].Place.Name < out[j].Place.Name
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// inTestFile says, from the name of a file, that it is a test (Go, Dart; the directory of the tests of Dart and Rust).
func inTestFile(file string) bool {
	f := filepath.ToSlash(file)
	return strings.HasSuffix(f, "_test.go") || strings.HasSuffix(f, "_test.dart") ||
		strings.Contains(f, "/test/") || strings.HasPrefix(f, "test/") || strings.Contains(f, "/tests/") || strings.HasPrefix(f, "tests/")
}
