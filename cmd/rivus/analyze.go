package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
)

// analyzeOpts are the flags common to the commands that run the analysis.
type analyzeOpts struct {
	common
	k        int
	verbose  bool
	cpuProf  string
	lockProf string
	memProf  string
	// the diagnostics (ADR-061)
	passStats    bool
	traceElement string
	timeout      time.Duration
	// funcTimeout bounds the analysis of one function.
	funcTimeout time.Duration
	passes      int
	numeric     bool
	workers     int
	// lang is the language to read: auto (what the directory holds), or a list.
	lang                         string
	dartExtractor, rustExtractor string
	maxImpl                      int
	// irFile is a stream of the neutral IR made by an extractor of another
	// language (rivus extract writes the same form): the analysis starts there.
	irFile string
	// afterRead, if set, is called with the program once the graph is built
	// and before the analysis (what the structure layer of the graph file is made from).
	// structureOnly stops after afterRead: analyze returns no report.
	structureOnly bool
	afterRead     func(l *loaded, rules *model.Rules, st *sites.Result, g *sg.Program, bodies map[string]string) error
}

func (o *analyzeOpts) register(fs *flag.FlagSet) {
	o.common.register(fs)
	fs.IntVar(&o.k, "k", taint.DefaultOptions().K, "maximum access path depth")
	fs.StringVar(&o.lang, "lang", "auto", "languages to read: auto (what the directory holds: go.mod / go.work, pubspec.yaml; Rust is read only when asked for, because reading it builds the project), or go, dart, rust, separated by commas")
	fs.StringVar(&o.dartExtractor, "dart-extractor", "", "command of the Dart extractor (default: $RIVUS_DART_EXTRACTOR, then rivus-dart)")
	fs.StringVar(&o.rustExtractor, "rust-extractor", "", "command of the Rust extractor (default: $RIVUS_RUST_EXTRACTOR, then rivus-rust)")
	fs.IntVar(&o.workers, "workers", 0, "functions analysed at the same time (0: one per CPU; 1: one at a time). The result does not depend on it")
	fs.BoolVar(&o.numeric, "numeric-taint", false, "follow taint through numbers and bools (by default it stops there: they cannot carry an injection)")
	fs.BoolVar(&o.verbose, "v", false, "print the time of each phase to stderr")
	fs.IntVar(&o.maxImpl, "max-implementations", model.DefaultMaxImplementations, "default limit of match: implementations")
	fs.IntVar(&o.passes, "max-passes", taint.DefaultOptions().MaxPasses, "maximum number of passes over the program while summaries change")
	fs.DurationVar(&o.timeout, "timeout", 0, "stop the analysis after this long and report what was found (0: no limit)")
	fs.DurationVar(&o.funcTimeout, "func-timeout", taint.DefaultOptions().FuncTimeout, "stop the analysis of one function after this long (it is then recorded as depth_limit, func_timeout: a clock, so the result then depends on the load of the machine)")
	fs.StringVar(&o.irFile, "ir", "", "analyse the neutral IR in this JSON Lines file (from an extractor of another language) instead of Go packages")
	fs.BoolVar(&o.passStats, "pass-stats", false, "add to stats what changed in the summaries in each pass (pass_changes) and the functions without a model that tainted values reached the most (unmodeled_top); the findings are the same (development)")
	fs.StringVar(&o.traceElement, "trace-element", "", "print to stderr where the taint of this source (a site ID) first leaves its function and enters a package variable; the findings are the same (development)")
	fs.StringVar(&o.memProf, "memprofile", "", "write a heap profile (allocations) to this file after the taint phase (development)")
	fs.StringVar(&o.lockProf, "lockprofile", "", "write the mutex and block profiles of the taint phase to <this>.mutex and <this>.block (development)")
	fs.StringVar(&o.cpuProf, "cpuprofile", "", "write a CPU profile of the taint phase to this file (development)")
}

// analyze runs the whole pipeline. extra rule sets are added to the ones
// loaded from the flags.
func (o *analyzeOpts) analyze(patterns []string, extra ...*model.RuleSet) (*report.Report, error) {
	phase := o.phaseTimer()
	rules, err := o.loadRules(extra...)
	if err != nil {
		return nil, err
	}
	l, err := o.read(patterns, rules, phase)
	if err != nil {
		return nil, err
	}
	// The graph drops the IR as it is built: what was analysed is listed first.
	packages := l.packages()
	noteNotBuiltin(os.Stderr, l.ir, rules)
	// The sites read the IR, and the graph drops it as it is built.
	expandSubtypes(rules, l.ir)
	st := sites.Enumerate(l.ir, l.texts, rules)
	phase("sites")
	var bodies map[string]string
	if o.afterRead != nil {
		bodies = bodyHashes(l)
	}
	graph, err := sg.BuildConsuming(l.ir)
	if err != nil {
		return nil, err
	}
	phase("graph")
	if o.afterRead != nil {
		if err := o.afterRead(l, rules, st, graph, bodies); err != nil {
			return nil, err
		}
		phase("graphdb")
		if o.structureOnly {
			return nil, nil
		}
	}
	opts := taint.DefaultOptions()
	opts.K = o.k
	opts.Timeout = o.timeout
	opts.FuncTimeout = o.funcTimeout
	opts.MaxPasses = o.passes
	opts.NumericTaint = o.numeric
	opts.Workers = o.workers
	opts.PassStats = o.passStats
	opts.TraceElement = o.traceElement
	if o.verbose {
		opts.SlowThreshold = 300 * time.Millisecond
		opts.SlowFunc = func(fn string, d time.Duration, outputs, roots int) {
			fmt.Fprintf(os.Stderr, "rivus: slow function %s: %v (%d outputs, %d roots)\n", fn, d.Round(time.Millisecond), outputs, roots)
		}
	}
	var res *taint.Result
	if o.cpuProf != "" {
		f, err := os.Create(o.cpuProf)
		if err != nil {
			return nil, err
		}
		pprof.StartCPUProfile(f)
		defer func() { pprof.StopCPUProfile(); f.Close() }()
		// A run that does not finish still leaves a usable profile.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		go func() {
			<-sig
			pprof.StopCPUProfile()
			f.Close()
			os.Exit(130)
		}()
	}
	if o.lockProf != "" {
		runtime.SetMutexProfileFraction(5)
		runtime.SetBlockProfileRate(1000)
	}
	eng := taint.New(graph, l.texts, st, rules, opts)
	eng.SetLoadErrors(l.loadErrors())
	res = eng.Run()
	if ts := l.testsSkipped(); ts != nil {
		res.Stats.TestsSkipped = ts
		if !ts.Empty() {
			fmt.Fprintf(os.Stderr, "rivus: %s\n", ts.Describe())
		}
		for _, n := range ts.Notes {
			fmt.Fprintf(os.Stderr, "rivus: %s\n", n)
		}
	}
	phase("taint")
	if o.verbose {
		fmt.Fprintf(os.Stderr, "rivus: %d distinct strings in the elements\n", taint.InternedStrings())
	}
	if o.lockProf != "" {
		for _, n := range []string{"mutex", "block"} {
			if f, err := os.Create(o.lockProf + "." + n); err == nil {
				pprof.Lookup(n).WriteTo(f, 0)
				f.Close()
			}
		}
	}
	if o.memProf != "" {
		if f, err := os.Create(o.memProf); err == nil {
			pprof.Lookup("allocs").WriteTo(f, 0)
			f.Close()
		}
	}
	rep := report.Build(packages, l.loadErrors(), st, res, rules)
	rep.Target = o.target(o.irFile != "")
	return rep, nil
}

// read loads the code (the Go packages, the IR of an extractor, or a stream of IR) and makes the IR of it.
func (o *analyzeOpts) read(patterns []string, rules *model.Rules, phase func(string)) (*loaded, error) {
	if o.irFile != "" {
		l, err := readIR(o.irFile)
		if err != nil {
			return nil, err
		}
		phase("ir")
		return l, nil
	}
	langs, err := o.languages()
	if err != nil {
		return nil, err
	}
	if len(langs) != 1 || langs[0] != langGo {
		l, err := o.loadLangs(langs, patterns, rules)
		if err != nil {
			return nil, err
		}
		phase("load")
		return l, nil
	}
	l, err := o.loadGo(patterns, rules, o.maxImpl)
	if err != nil {
		return nil, err
	}
	phase("ir")
	return l, nil
}

// phaseTimer returns a function that prints the time since the previous call.
func (o *analyzeOpts) phaseTimer() func(string) {
	last := time.Now()
	return func(name string) {
		if o.verbose {
			// The live heap after a collection: what each phase leaves behind.
			var m runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m)
			fmt.Fprintf(os.Stderr, "rivus: %-10s %v  (heap %d MB)\n", name, time.Since(last).Round(time.Millisecond), m.HeapAlloc>>20)
		}
		last = time.Now()
	}
}

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	detail := fs.String("detail", report.DetailSnippet, "level of detail: summary or snippet")
	withSites := fs.Bool("sites", false, "also list the sites")
	fs.Parse(args)
	// What can be checked is checked before the analysis, which can take minutes.
	if err := report.ValidDetail(*detail); err != nil {
		return err
	}
	if *withSites && *detail == report.DetailSummary {
		return fmt.Errorf("-sites cannot be combined with -detail summary: the summary leaves the sites out")
	}

	rep, err := o.analyze(fs.Args())
	if err != nil {
		return err
	}
	if !*withSites {
		rep.Sites = nil
	}
	rep, err = rep.WithDetail(*detail)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, rep)
}

func runIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	out := fs.String("o", "", "with -taint: the JSON index to write (default: .rivus/index.json below -C)")
	graph := fs.String("graph", "", "the graph file to write (default: in the user's cache, by the absolute path of -C)")
	taintFlag := fs.Bool("taint", false, "also run the taint analysis (slow) and write its result as the JSON index that query -index and resolve -index read (-o implies it)")
	fs.Parse(args)
	// -o has always meant the JSON index: it is not left out silently
	taint := taintFlag
	if *out != "" {
		t := true
		taint = &t
	}
	path, err := graphFile(*graph, &o)
	if err != nil {
		return err
	}
	started := time.Now()
	var counts *graphdb.Counts
	o.afterRead = o.graphIndexer(path, fs.Args(), started, func(c *graphdb.Counts) { counts = c })
	o.structureOnly = !*taint

	rep, err := o.analyze(fs.Args())
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "rivus: wrote %s (%d functions, %d call sites, %d edges, %d sites, %d files read, %d ms)\n",
		path, counts.Funcs, counts.CallSites, counts.Edges, counts.Sites, counts.Files, time.Since(started).Milliseconds())
	if !*taint {
		fmt.Fprintln(os.Stderr, "rivus: the file has the structure layer only (functions, call sites, edges, sites): there is no taint layer, so it holds no findings and says nothing of whether the code has any. -taint runs the analysis and writes the JSON index")
		return nil
	}
	jpath := *out
	if jpath == "" {
		jpath = filepath.Join(o.dir, ".rivus", "index.json")
	}
	if err := rep.Save(jpath); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "rivus: wrote %s (%d sites, %d findings, %d uncertainties, %d ms)\n",
		jpath, len(rep.Sites), len(rep.Findings), len(rep.Uncertainties), rep.Stats.DurationMilli)
	fmt.Fprintln(os.Stderr, "rivus: the taint layer is in the JSON index, not in the graph file (it will move there)")
	return nil
}

func runQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	sink := fs.String("sink", "", "which sources can reach this sink symbol")
	source := fs.String("source", "", "which sinks can be reached from this source symbol")
	site := fs.String("site", "", "ID of a site (see \"rivus sites\")")
	index := fs.String("index", "", "answer from a saved index instead of analysing the source")
	detail := fs.String("detail", report.DetailSnippet, "level of detail: summary or snippet")
	fs.Parse(args)

	q := report.Query{Sink: *sink, Source: *source, Site: *site}
	if err := q.Validate(); err != nil {
		return err
	}
	if err := report.ValidDetail(*detail); err != nil {
		return err
	}
	var rep *report.Report
	var err error
	if *index != "" {
		// The index is the answer's source: nothing is analysed, so the flags
		// that steer an analysis, and the packages, have no effect. They are
		// refused rather than ignored.
		own := map[string]bool{"sink": true, "source": true, "site": true, "index": true, "detail": true}
		var ignored []string
		fs.Visit(func(f *flag.Flag) {
			if !own[f.Name] {
				ignored = append(ignored, "-"+f.Name)
			}
		})
		if len(ignored) > 0 || fs.NArg() > 0 {
			return fmt.Errorf("query -index answers from the saved index and analyses nothing: %s cannot be combined with it", strings.Join(append(ignored, fs.Args()...), " "))
		}
		rep, err = report.Load(*index)
	} else {
		rep, err = o.analyze(fs.Args(), adHocRules(q)...)
	}
	if err != nil {
		return err
	}
	res, err := rep.Run(q)
	if err != nil {
		return err
	}
	if *index != "" && len(res.Matched) == 0 && (q.Sink != "" || q.Source != "") {
		res.Notes = append(res.Notes, "the index holds no site of this symbol: an index answers only for the symbols that its rules name, "+
			"so this is not \"nothing reaches it\". Query the source (without -index), or index again with a rule for the symbol")
	}
	if *detail != "" && *detail != report.DetailSnippet {
		d, err := (&report.Report{SchemaVersion: report.SchemaVersion, Findings: res.Findings}).WithDetail(*detail)
		if err != nil {
			return err
		}
		res.Findings = d.Findings
	}
	return writeJSON(os.Stdout, res)
}

// adHocRules lets a one-sided query name a symbol that no rule mentions: a
// sink query makes the symbol a sink for any label, a source query makes
// its result a source.
func adHocRules(q report.Query) []*model.RuleSet {
	rs := &model.RuleSet{Name: "query", Origin: "query"}
	if q.Sink != "" {
		rs.Sinks = append(rs.Sinks, model.Sink{Symbol: q.Sink, Category: "query"})
	}
	if q.Source != "" {
		rs.Sources = append(rs.Sources, model.Source{Kind: model.KindCallResult, Symbol: q.Source, Labels: []string{"queried"}})
	}
	if len(rs.Sinks) == 0 && len(rs.Sources) == 0 {
		return nil
	}
	return []*model.RuleSet{rs}
}

func runResolve(args []string) error {
	fs := flag.NewFlagSet("resolve", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	bookmarks := fs.String("bookmarks", "", "file with the bookmarks: sites or findings as rivus printed them (- for stdin)")
	index := fs.String("index", "", "resolve against a saved index of the current code instead of analysing the source")
	fs.Parse(args)
	if *bookmarks == "" {
		return fmt.Errorf("resolve needs -bookmarks")
	}
	bms, err := report.ReadBookmarks(*bookmarks)
	if err != nil {
		return err
	}
	var rep *report.Report
	if *index != "" {
		rep, err = report.Load(*index)
	} else {
		rep, err = o.analyze(fs.Args())
	}
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, rep.Resolve(bms))
}
