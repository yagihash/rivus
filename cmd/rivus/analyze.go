package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/ir"
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
	depK     int
	verbose  bool
	cpuProf  string
	lockProf string
	memProf  string
	// the diagnostics (ADR-061)
	passStats     bool
	passStatsTop  int
	passStatsSort string
	traceElement  string
	funcValues    string
	// resolveFuncValues gives the calls of function values that have no candidate the functions that the analysis of the
	// function values carries to them, before the taint analysis (docs/DESIGN-callouts.md, 5.6). Off by default.
	resolveFuncValues bool
	// lockLangs are the languages whose extractor ran in this run (Dart, Rust): their lock file, in -C, says the versions
	// (unverifiedFromLocks). Set by loadLangs, so a run from -ir or -index has none.
	lockLangs []string
	// rustIsolation is how the Rust project was read (set when it was); it goes to the summary
	rustIsolation *report.Isolation
	timeout       time.Duration
	// funcTimeout bounds the analysis of one function.
	funcTimeout time.Duration
	passes      int
	numeric     bool
	workers     int
	// lang is the language to read: auto (what the directory holds), or a list.
	lang string
	yes  bool
	// what the question of -lang auto (a Rust project in the directory) uses; a test replaces them. isTerminal: is the
	// standard input a terminal (nil: ask the system; never true in a test binary). confirmIn: where the answer is read
	// from (nil: the standard input). sandboxCheck, installCheck: nil is the system's.
	isTerminal   func() bool
	confirmIn    io.Reader
	sandboxCheck func() error
	installCheck func() error
	// the languages are decided once for a run: the answer to the question is not asked twice
	langsDone bool
	langsKey  string
	langsOut  []string
	langsErr  error
	// skipped are the languages of the directory that were not read, and why (summary.unread.skipped_languages)
	skipped                      []report.SkippedLanguage
	dartExtractor, rustExtractor string
	// the isolation of the Rust extractor (ADR-062)
	unsafeHostExec   bool
	isolationTimeout time.Duration
	noFetch          bool
	isolationEnv     multiFlag
	isolationRead    multiFlag
	isolationMaxOut  int64
	allowFetchHosts  multiFlag
	maxImpl          int
	// irFile is a stream of the neutral IR made by an extractor of another
	// language (rivus debug extract writes the same form): the analysis starts there.
	irFile string
	// afterRead, if set, is called with the program once the graph is built
	// and before the analysis (what the structure layer of the graph file is made from).
	// structureOnly stops after afterRead: analyze returns no report.
	structureOnly bool
	// categories is what `analyze` reports, by category (nil: everything, as before). It cuts the list of findings once the analysis
	// is done; the graph, the index and the analysis itself do not see it.
	categories *categoryFlags
	afterRead  func(l *loaded, rules *model.Rules, st *sites.Result, g *sg.Program, bodies map[string]string) error
}

// registerLanguages adds the flags that say which languages are read and how (the Rust extractor runs under a sandbox). The commands that
// read the code of a directory (analyze, index, sites, ...) have them.
func (o *analyzeOpts) registerLanguages(fs *flag.FlagSet) {
	fs.StringVar(&o.lang, "lang", langAuto, "languages to read: "+langAuto+" (what the directory holds: go.mod / go.work, pubspec.yaml, Cargo.toml; a Rust project is read after a question [Y/n] on the terminal, or with -yes, because reading it builds it; where there is no sandbox it is not read), or "+strings.Join(languageNames, ", ")+", separated by commas (-lang rust asks nothing). The answer makes a different index")
	fs.BoolVar(&o.yes, "yes", false, "-lang auto: read the Rust project of the directory without asking (what the question would say yes to; needed where there is no terminal to ask)")
	fs.StringVar(&o.dartExtractor, "dart-extractor", "", "command of the Dart extractor (default: $RIVUS_DART_EXTRACTOR, then rivus-dart)")
	fs.StringVar(&o.rustExtractor, "rust-extractor", "", "command of the Rust extractor (default: $RIVUS_RUST_EXTRACTOR, then rivus-rust)")
	fs.BoolVar(&o.unsafeHostExec, "unsafe-host-exec", false, "run the Rust extractor on this machine, with your rights: the build scripts and proc macros of the project run here. Only for code you trust (default: under the sandbox of macOS; on another system nothing is run without this flag; docs/rust-isolation.md; also RIVUS_UNSAFE_HOST_EXEC=1)")
	fs.DurationVar(&o.isolationTimeout, "isolation-timeout", 30*time.Minute, "stop a stage of the Rust extraction (fetch, extract) after this long")
	fs.BoolVar(&o.noFetch, "no-fetch", false, "do not fetch the dependencies of the Rust project (use what an earlier run fetched)")
	fs.Var(&o.isolationEnv, "isolation-env", "NAME=VALUE for the build of the Rust project (repeatable). The value is written here; nothing is taken from your environment (a build script that needs a setting that it would fetch otherwise, like LOCAL_ASSETS_PATH)")
	fs.Var(&o.isolationRead, "isolation-read", "PATH: a directory or file of this machine that the build of the Rust project may read besides the project and its dependencies (repeatable; not your home directory)")
	fs.Int64Var(&o.isolationMaxOut, "isolation-max-output", 4<<30, "the most bytes the Rust extractor may write (the stream of the IR)")
	fs.Var(&o.allowFetchHosts, "allow-fetch-host", "HOST: let the fetch stage of the Rust project (under the sandbox, with a network) get from this host, which Cargo.lock names (git dependencies on any host, other registries; by default only crates.io is allowed). The repository can send the fetch on to other places (its submodules, redirects); that is not prevented until the network of the fetch stage is narrowed (repeatable)")
	usual(fs, "lang", "yes", "dart-extractor", "rust-extractor", "unsafe-host-exec", "isolation-env", "isolation-read")
	diagnostic(fs, "isolation-timeout", "no-fetch", "isolation-max-output", "allow-fetch-host")
	kind(fs, "enum:lang", "lang")
	kind(fs, "file", "dart-extractor", "rust-extractor", "isolation-read")
}

func (o *analyzeOpts) register(fs *flag.FlagSet) {
	o.common.register(fs)
	o.registerLanguages(fs)
	fs.IntVar(&o.k, "k", taint.DefaultOptions().K, "maximum access path depth")
	fs.IntVar(&o.depK, "dep-k", 0, "maximum access path depth in the functions of the dependencies (an experiment; 0: the same as -k). A smaller depth makes the analysis of heavy dependencies cheaper and adds flows (a path that is cut stands for everything below it)")
	fs.IntVar(&o.workers, "workers", 0, "functions analysed at the same time (0: one per CPU; 1: one at a time). The result does not depend on it")
	fs.BoolVar(&o.numeric, "numeric-taint", false, "follow taint through numbers and bools (by default it stops there: they cannot carry an injection)")
	fs.BoolVar(&o.verbose, "v", false, "print the time of each phase to stderr")
	fs.IntVar(&o.maxImpl, "max-implementations", model.DefaultMaxImplementations, "default limit of match: implementations")
	fs.IntVar(&o.passes, "max-passes", taint.DefaultOptions().MaxPasses, "maximum number of passes over the program while summaries change")
	fs.DurationVar(&o.timeout, "timeout", 0, "stop the analysis after this long and report what was found (0: no limit)")
	fs.DurationVar(&o.funcTimeout, "func-timeout", taint.DefaultOptions().FuncTimeout, "stop the analysis of one function after this long (it is then recorded as depth_limit, func_timeout: a clock, so the result then depends on the load of the machine)")
	fs.StringVar(&o.irFile, "ir", "", "analyse the neutral IR in this JSON Lines file (from an extractor of another language) instead of Go packages")
	fs.BoolVar(&o.passStats, "pass-stats", false, "add to stats what changed in the summaries in each pass (pass_changes) and the functions without a model that tainted values reached the most (unmodeled_top); the findings are the same (development)")
	fs.IntVar(&o.passStatsTop, "pass-stats-top", taint.DefaultOptions().PassStatsTop, "with -pass-stats: how many packages packages_top lists (0: all of them) (development)")
	fs.StringVar(&o.passStatsSort, "pass-stats-sort", "reanalyses", "with -pass-stats: what packages_top is ordered by: "+orList(passStatsSorts, false)+" (equal values go by name; time is a clock, so it differs from run to run) (development)")
	fs.StringVar(&o.funcValues, "func-values", "", "write to this file (JSON Lines) the diagnostic of the function values: for each call of a function value, the functions that the elements carried by the analysis say it may call, beside the candidates it has otherwise (the analysis is run once more, about twice the time; the findings and the output are the same) (development, docs/DESIGN-callouts.md 5)")
	fs.BoolVar(&o.resolveFuncValues, "resolve-func-values", false, "give the calls of function values that have no candidate the functions that the analysis of the function values carries to them (functions made in the code of the project), then run the taint analysis; the calls keep their unresolved_callee uncertainty. It runs the analysis of the function values up to 4 times more (nothing runs when no such call exists), and stats.func_values says what it did (docs/DESIGN-callouts.md 5.6)")
	fs.StringVar(&o.traceElement, "trace-element", "", "print to stderr where the taint of this source (a site ID) first leaves its function and enters a package variable; the findings are the same (development)")
	fs.StringVar(&o.memProf, "memprofile", "", "write a heap profile (allocations) to this file after the taint phase (development)")
	fs.StringVar(&o.lockProf, "lockprofile", "", "write the mutex and block profiles of the taint phase to <this>.mutex and <this>.block (development)")
	fs.StringVar(&o.cpuProf, "cpuprofile", "", "write a CPU profile of the taint phase to this file (development)")
	usual(fs, "workers", "numeric-taint", "v", "timeout", "resolve-func-values")
	diagnostic(fs, "k", "dep-k", "max-implementations", "max-passes", "func-timeout", "ir", "pass-stats", "pass-stats-top", "pass-stats-sort", "func-values", "trace-element", "memprofile", "lockprofile", "cpuprofile")
	kind(fs, "enum:pass-stats-sort", "pass-stats-sort")
	kind(fs, "file", "ir", "func-values", "memprofile", "lockprofile", "cpuprofile")
}

// analyze runs the whole pipeline. extra rule sets are added to the ones
// loaded from the flags.
func (o *analyzeOpts) analyze(patterns []string, extra ...*model.RuleSet) (*report.Report, error) {
	if err := enumFlags["pass-stats-sort"].check(o.passStatsSort); err != nil {
		return nil, err
	}
	phase := o.phaseTimer()
	rules, err := o.loadRules(extra...)
	if err != nil {
		return nil, err
	}
	noteUnscopedSanitizers(os.Stderr, rules)
	if err := o.categories.check(rules); err != nil {
		return nil, err
	}
	l, err := o.read(patterns, rules, phase)
	if err != nil {
		return nil, err
	}
	// what the rule files declare is not read: the functions go before the sites and the graph see them, and the
	// packages that are listed as analysed
	replaced := dropReplacedBodies(l.ir, rules, os.Stderr)
	if len(replaced) > 0 {
		// what only the code that was left out reached is left out too: it is what the extractor that is told not to
		// read the package makes (the instances of generic functions and the wrappers that the code made, the packages
		// that only it imports). Nothing the project reaches is lost
		l.ir.PruneDeps()
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
	opts := taint.DefaultOptions()
	opts.K = o.k
	opts.DepK = o.depK
	opts.Timeout = o.timeout
	opts.FuncTimeout = o.funcTimeout
	opts.MaxPasses = o.passes
	opts.NumericTaint = o.numeric
	opts.Workers = o.workers
	opts.PassStats = o.passStats
	opts.PassStatsTop = o.passStatsTop
	opts.PassStatsSort = o.passStatsSort
	opts.TraceElement = o.traceElement
	if o.verbose {
		opts.SlowThreshold = 300 * time.Millisecond
		opts.SlowFunc = func(fn string, d, cpu time.Duration, outputs, roots int) {
			// cpu: the thread's own CPU time over the same span (0: no clock on this system); wall time swells with the collector
			fmt.Fprintf(os.Stderr, "rivus: slow function %s: %v (cpu %v, %d outputs, %d roots)\n", fn, d.Round(time.Millisecond), cpu.Round(time.Millisecond), outputs, roots)
		}
	}
	// -func-values and -resolve-func-values come before the structure layer is written when the flag is on (the calls that
	// get candidates are edges of the file), and after it otherwise (as they always did). They run once.
	var fvStats *taint.FuncValuesStats
	fvDone := false
	resolveFV := func() {
		if fvDone {
			return
		}
		fvDone = true
		if o.funcValues != "" {
			// before the candidates are given (-resolve-func-values): the table says what the rest of the analysis has
			if err := writeFuncValues(o.funcValues, graph, l, st, rules, opts); err != nil {
				fmt.Fprintf(os.Stderr, "rivus: -func-values: %v\n", err)
			}
		}
		if o.resolveFuncValues {
			begun := time.Now()
			fvStats = taint.ResolveFuncValues(graph, l.texts, st, rules, opts, l.loadErrors())
			phase("func-values")
			if o.timeout > 0 {
				// -timeout is the limit of the rounds and of the analysis after them together; the rounds use at most
				// taint.FuncValuesBudget of it, so what is left here is at least the rest
				opts.Timeout = max(o.timeout-time.Since(begun), time.Nanosecond)
			}
		}
	}
	if o.afterRead != nil && o.resolveFuncValues {
		resolveFV()
	}
	if o.afterRead != nil {
		if err := o.afterRead(l, rules, st, graph, bodies); err != nil {
			return nil, err
		}
		phase("graphdb")
		if o.structureOnly {
			return nil, nil
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
	resolveFV()
	eng := taint.New(graph, l.texts, st, rules, opts)
	eng.SetLoadErrors(l.loadErrors())
	res = eng.Run()
	res.Stats.FuncValues = fvStats
	if ts := l.testsSkipped(); ts != nil {
		// the directories that were not read are not tests: they are in summary.skipped_dirs, not here
		stats := *ts
		stats.SkippedDirs, stats.SkippedDirNames = 0, nil
		stats.OutsideBuildFiles, stats.OutsideBuildFileNames = 0, nil
		if !stats.Empty() || len(stats.Notes) > 0 {
			res.Stats.TestsSkipped = &stats
		}
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
	var excluded []report.CategoryExcluded
	res.Findings, excluded = o.categories.filter().Apply(res.Findings)
	rep := report.Build(packages, l.loadErrors(), st, res, rules)
	rep.Summary.CategoriesExcluded = excluded
	rep.Summary.Isolation = o.rustIsolation
	rep.Summary.BodyReplacedPackages = replaced
	if fs := report.FunctionsSkippedOf(l.ir.FnSkipped); fs != nil {
		rep.Summary.FunctionsSkipped = fs
		fmt.Fprintf(os.Stderr, "rivus: %d functions could not be read by the extractor and were skipped (%d of the project, %d of dependencies): summary.functions_skipped\n", fs.Functions, fs.Project, fs.Dependencies)
	}
	rep.Summary.BodyReplacedIgnored = o.declIgnored
	rep.Summary.DepDirs = sortedUnique(o.depDirs)
	if l.gossa != nil {
		rep.Summary.UnverifiedVersions = rules.UnverifiedVersions(l.gossa.ModuleVersions, "go.mod")
	}
	rep.Summary.UnverifiedVersions = append(rep.Summary.UnverifiedVersions, o.unverifiedFromLocks(rules)...)
	sort.SliceStable(rep.Summary.UnverifiedVersions, func(i, j int) bool {
		return rep.Summary.UnverifiedVersions[i].Package < rep.Summary.UnverifiedVersions[j].Package
	})
	if len(o.skipped) > 0 {
		if rep.Summary.Unread == nil {
			rep.Summary.Unread = &report.Unread{Top: []report.UnreadPackage{}}
		}
		rep.Summary.Unread.SkippedLanguages = append([]report.SkippedLanguage(nil), o.skipped...)
	}
	if ts := l.testsSkipped(); ts != nil {
		if ts.SkippedDirs > 0 {
			rep.Summary.SkippedDirs = &report.SkippedDirs{Total: ts.SkippedDirs, Examples: ts.SkippedDirNames}
		}
		if ts.OutsideBuildFiles > 0 {
			rep.Summary.OutsideConfiguration = &report.OutsideConfiguration{Files: ts.OutsideBuildFiles, Examples: ts.OutsideBuildFileNames}
		}
	}
	if u := rep.Summary.Unread; u != nil && u.Packages > 0 {
		// the exit code stays 0 (a flag for it is a decision of its own): a result without the packages that
		// did not build must not look like the analysis of all of them
		names := make([]string, 0, 3)
		for i, p := range u.Top {
			if i == 3 {
				break
			}
			names = append(names, fmt.Sprintf("%s (%d)", p.Package, p.Functions))
		}
		// the kinds that have a sentence of their own below: the sentence about a type error is not about them
		specific := func(why string) bool {
			for _, pre := range []string{ir.DependencyNotAllowedPrefix, ir.DependencyNotLinkedPrefix, ir.PackageConfigMissingPrefix, ir.PackageConfigOldPrefix} {
				if strings.HasPrefix(why, pre) {
					return true
				}
			}
			return false
		}
		allSpecific := len(u.Top) == u.Packages
		for _, p := range u.Top {
			allSpecific = allSpecific && specific(p.Why)
		}
		if !allSpecific {
			fmt.Fprintf(os.Stderr, "rivus: %d package(s) or file(s) were read and not built (a type error in them or in a package they import), or read on a syntax tree that the analyzer recovered (a syntax error: a declaration may be missing); %d function(s) of them are not analysed: %s; see summary.unread\n",
				u.Packages, u.Functions, strings.Join(names, ", "))
		}
		for _, p := range u.Top {
			if strings.HasPrefix(p.Why, ir.PackageConfigMissingPrefix) || strings.HasPrefix(p.Why, ir.PackageConfigOldPrefix) {
				fmt.Fprintf(os.Stderr, "rivus: the Dart package %s is not prepared (package_config.json is not there, or old): the imports of other packages do not resolve, so its dependencies are not read; run `dart pub get` in it (rivus does not run it); see summary.unread\n", p.Package)
				break
			}
		}
		for _, p := range u.Top {
			if strings.HasPrefix(p.Why, ir.DependencyNotAllowedPrefix) {
				fmt.Fprintf(os.Stderr, "rivus: a dependency (%s) is outside the places that are trusted (the project, the pub cache, the SDKs), so it is not read and its functions are not analysed; -allow-deps-under <its parent directory> reads it\n", p.Package)
				break
			}
		}
		for _, p := range u.Top {
			if strings.HasPrefix(p.Why, ir.DependencyNotLinkedPrefix) {
				fmt.Fprintf(os.Stderr, "rivus: a dependency (%s) is not read as the calls to it name it (not resolved to a package: URI, or another version of it is read in the same tree), so its functions are not analysed; point -C at one package, or run `dart pub get` in each package\n", p.Package)
				break
			}
		}
	}
	rep.Target = o.target(o.irFile != "")
	if fs := rep.Summary.FunctionsSkipped; fs != nil && fs.Project > 0 {
		// the last line of the run: the code of the project is not all in the analysis, and a flow through these functions is not seen
		fmt.Fprintf(os.Stderr, "rivus: WARNING: %d function(s) of the project were skipped (the extractor could not read them): flows through them are not seen; see summary.functions_skipped\n", fs.Project)
	}
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

// analyzeCmd is "rivus analyze": its flags (analyzeFlags defines them, and runs nothing) and what it holds of them.
type analyzeCmd struct {
	o         analyzeOpts
	detail    *string
	withSites *bool
	cats      *categoryFlags
}

func analyzeFlags(fs *flag.FlagSet) *analyzeCmd {
	c := &analyzeCmd{}
	c.o.register(fs)
	c.detail = fs.String("detail", report.DetailSnippet, "level of detail: "+orList(report.DetailLevels, false))
	c.withSites = fs.Bool("sites", false, "also list the sites")
	c.cats = &categoryFlags{}
	fs.StringVar(&c.cats.include, "categories", "", "report only the findings of these categories (names separated by commas, as the `category` of a finding; an unknown name is an error that lists the valid ones). The analysis is the same; summary.categories_excluded counts the findings that were left out")
	fs.StringVar(&c.cats.skip, "skip-categories", "", "report all the findings except those of these categories (names separated by commas; not with -categories). summary.categories_excluded counts the findings that were left out")
	usual(fs, "detail", "sites", "categories", "skip-categories")
	kind(fs, "enum:detail", "detail")
	return c
}

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	c := analyzeFlags(fs)
	prepareFlags(fs, args)
	fs.Parse(args)
	o, detail, withSites := &c.o, c.detail, c.withSites
	// What can be checked is checked before the analysis, which can take minutes.
	if err := c.cats.parse(); err != nil {
		return err
	}
	o.categories = c.cats
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

// indexCmd is "rivus index".
type indexCmd struct {
	o          analyzeOpts
	out, graph *string
	taintFlag  *bool
}

func indexFlags(fs *flag.FlagSet) *indexCmd {
	c := &indexCmd{}
	c.o.register(fs)
	c.out = fs.String("o", "", "with -taint: the JSON index to write (default: index.json beside the default graph file, in the user's cache by the absolute path of -C; never in the analysed directory)")
	c.graph = fs.String("graph", "", "the graph file to write (default: in the user's cache, by the absolute path of -C)")
	c.taintFlag = fs.Bool("taint", false, "also run the taint analysis (slow) and write its result as the JSON index that query -index and resolve -index read (-o implies it)")
	usual(fs, "o", "graph", "taint")
	kind(fs, "file", "o", "graph")
	return c
}

func runIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	c := indexFlags(fs)
	prepareFlags(fs, args)
	fs.Parse(args)
	o, out, graph, taintFlag := &c.o, c.out, c.graph, c.taintFlag
	// -o has always meant the JSON index: it is not left out silently
	taint := taintFlag
	if *out != "" {
		t := true
		taint = &t
	}
	path, err := graphFile(*graph, o)
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
		jpath, err = indexCachePath(o.dir)
		if err != nil {
			return err
		}
	}
	if err := rep.Save(jpath); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "rivus: wrote %s (%d sites, %d findings, %d uncertainties, %d ms)\n",
		jpath, len(rep.Sites), len(rep.Findings), len(rep.Uncertainties), rep.Stats.DurationMilli)
	fmt.Fprintln(os.Stderr, "rivus: the taint layer is in the JSON index, not in the graph file (it will move there)")
	return nil
}

// queryCmd is "rivus query".
type queryCmd struct {
	o                                 analyzeOpts
	sink, source, site, index, detail *string
}

func queryFlags(fs *flag.FlagSet) *queryCmd {
	c := &queryCmd{}
	c.o.register(fs)
	c.sink = fs.String("sink", "", "which sources can reach this sink symbol")
	c.source = fs.String("source", "", "which sinks can be reached from this source symbol")
	c.site = fs.String("site", "", "ID of a site (see \"rivus sites\")")
	c.index = fs.String("index", "", "answer from a saved index instead of analysing the source")
	c.detail = fs.String("detail", report.DetailSnippet, "level of detail: "+orList(report.DetailLevels, false))
	usual(fs, "sink", "source", "site", "index", "detail")
	kind(fs, "file", "index")
	kind(fs, "enum:detail", "detail")
	return c
}

func runQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	c := queryFlags(fs)
	prepareFlags(fs, args)
	fs.Parse(args)
	o, sink, source, site, index, detail := &c.o, c.sink, c.source, c.site, c.index, c.detail

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

// resolveCmd is "rivus resolve".
type resolveCmd struct {
	o                analyzeOpts
	bookmarks, index *string
}

func resolveFlags(fs *flag.FlagSet) *resolveCmd {
	c := &resolveCmd{}
	c.o.register(fs)
	c.bookmarks = fs.String("bookmarks", "", "file with the bookmarks: sites or findings as rivus printed them (- for stdin)")
	c.index = fs.String("index", "", "resolve against a saved index of the current code instead of analysing the source")
	usual(fs, "bookmarks", "index")
	kind(fs, "file", "bookmarks", "index")
	return c
}

func runResolve(args []string) error {
	fs := flag.NewFlagSet("resolve", flag.ExitOnError)
	c := resolveFlags(fs)
	prepareFlags(fs, args)
	fs.Parse(args)
	o, bookmarks, index := &c.o, c.bookmarks, c.index
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

// categoryFlags are -categories and -skip-categories of analyze.
type categoryFlags struct {
	include, skip string
	// the names, once parse has split them
	inc, exc []string
}

// splitCategories splits a list of names separated by commas (spaces around a name are left out).
func splitCategories(flagName, v string) ([]string, error) {
	if v == "" {
		return nil, nil
	}
	var out []string
	for _, n := range strings.Split(v, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			return nil, fmt.Errorf("-%s: an empty category name in %q", flagName, v)
		}
		out = append(out, n)
	}
	return out, nil
}

// parse reads the flags (what is wrong with their form is said before the analysis is started).
func (c *categoryFlags) parse() error {
	if c.include != "" && c.skip != "" {
		return fmt.Errorf("-categories and -skip-categories cannot be combined: use one of them")
	}
	var err error
	if c.inc, err = splitCategories("categories", c.include); err != nil {
		return err
	}
	if c.exc, err = splitCategories("skip-categories", c.skip); err != nil {
		return err
	}
	return nil
}

// check says an unknown name, with the valid ones: the categories of the sinks of the rules that were loaded. It is called after the
// rules are loaded, before the code is read.
func (c *categoryFlags) check(rules *model.Rules) error {
	if c == nil {
		return nil
	}
	known := map[string]bool{}
	for _, sinks := range rules.Sinks {
		for _, s := range sinks {
			known[s.Category] = true
		}
	}
	valid := make([]string, 0, len(known))
	for k := range known {
		valid = append(valid, k)
	}
	sort.Strings(valid)
	for _, n := range append(append([]string(nil), c.inc...), c.exc...) {
		if !known[n] {
			return fmt.Errorf("unknown category %q: the rules that were loaded have sinks of %s", n, strings.Join(valid, ", "))
		}
	}
	return nil
}

// filter is the filter of the report (nil: nothing is left out).
func (c *categoryFlags) filter() *report.CategoryFilter {
	if c == nil {
		return nil
	}
	return &report.CategoryFilter{Include: c.inc, Exclude: c.exc}
}

// noteUnscopedSanitizers says once, for the rule files of the project, that a sanitizer without `neutralises` hides the value from
// the sinks of every kind (ADR-115). It is the behaviour of before, kept so that the rules that were written for it go on working.
func noteUnscopedSanitizers(w io.Writer, rules *model.Rules) {
	n := 0
	var first string
	for _, rs := range rules.Sets {
		if !rs.Project {
			continue
		}
		for _, s := range rs.Sanitizers {
			if s.Neutralises == nil {
				if n == 0 {
					first = s.Symbol
				}
				n++
			}
		}
	}
	if n > 0 {
		fmt.Fprintf(w, "rivus: %d sanitizer(s) of the project rules (%s ...) have no `neutralises`: each hides the value from the sinks of every kind, not only the kind it is safe for; write `neutralises: [<categories>]` (docs/MODELING.md)\n", n, first)
	}
}
