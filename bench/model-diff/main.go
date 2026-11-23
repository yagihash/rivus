// model-diff: the check of a model of a package. A function of the library is called with a source, once per position, and what
// comes out of it (the results, what it writes into) goes to a sink of its own. The same code is analysed twice: with the bodies of
// the package read and the rows of the package left out of the models (A), and with the models (B).
//
//	MISS   A has the flow and B does not: the model lost it.
//	EXTRA  B has the flow and A does not: the model is wider than the body (allowed by `# model-diff: extra <symbol> <reason>` in the model file).
//
// A `sinks` row is not compared with a body (a sink inside a dependency is not a finding): it is compared with its declaration:
// the source in each position, one at a time, and a control with constants.
// See docs/MODEL-RUNBOOK.md ("differential test") and bench/README.md.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yagihash/rivus/internal/model"
)

func main() { os.Exit(realMain(os.Args[1:])) }

func realMain(args []string) int {
	fs := flag.NewFlagSet("model-diff", flag.ContinueOnError)
	lang := fs.String("lang", "auto", "go, dart or rust (auto: dart for package:<name>, else go). rust: only the spelling of the rows (local, macOS sandbox; exit 0 all found, 1 a row missing or spelled otherwise, 2 failed, 3 skipped, 4 rows that only resolve)")
	changed := fs.String("changed", "", "print the packages whose rows differ from the ones at this commit (what a pull request checks), and stop")
	runCh := fs.String("run-changed", "", "run the differential test of every package whose rows differ from the ones at this commit (what a pull request checks), one table at the end; exit 1 when one failed")
	langs := fs.String("langs", "go", "with -run-changed: the languages that are run (go, dart); a changed package of another one is listed as not run")
	deadAllow := fs.String("dead-allow", "", "count the rows of the .dead-allow files that a differential test judged, using the report*.json files under this directory (what -run-changed writes), and stop")
	reviewed := fs.String("changed-reviewed", "", "print the miss-reviewed notes added since this commit (one line each, `miss-reviewed-added <file> <symbol>@<position> <kind>`; a notice for the pull request, never a failure), and stop")
	gaps := fs.Bool("coverage-gaps", false, "Go standard library: with -pkg, list the exported functions and methods that take a string, bytes or `any` and have no row in the built-in models (the probes start from rows: a function without a row is not found by them), and stop")
	pkg := fs.String("pkg", "", "the package: a Go import path, or package:<name> for Dart")
	modelFile := fs.String("model", "", "the model file (a built-in one or a -rules file); default: the built-in files that have rows of the package")
	rivus := fs.String("rivus", "", "the rivus binary (default: built from -repo)")
	repo := fs.String("repo", ".", "the checkout of rivus")
	out := fs.String("out", "", "where the code and the results go (default: a directory under the temporary one)")
	jsonOut := fs.String("json", "", "write the report (JSON) here (default: <out>/report.json)")
	version := fs.String("version", "", "the version of the package (default: the one that go.mod / pubspec.lock of the checkout has if any, else the latest)")
	local := fs.String("local", "", "Go: a module on this machine in place of a fetched one (the tests)")
	timeout := fs.String("timeout", "600s", "limit of each analysis")
	rustExtractor := fs.String("rust-extractor", os.Getenv("RIVUS_RUST_EXTRACTOR"), "Rust: the extractor (built from extractor/rust; default RIVUS_RUST_EXTRACTOR)")
	features := fs.String("features", "", "Rust: features of the crate that the probe crate asks for (comma separated)")
	noDefault := fs.Bool("no-default-features", false, "Rust: the probe crate asks for default-features = false")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *deadAllow != "" {
		abs, _ := filepath.Abs(*repo)
		if err := deadAllowCount(abs, *deadAllow); err != nil {
			return fail(err)
		}
		return 0
	}
	if *runCh != "" {
		abs, _ := filepath.Abs(*repo)
		var extra []string
		if *rivus != "" {
			extra = append(extra, "-rivus", *rivus)
		}
		if *version != "" || *local != "" {
			fmt.Fprintln(os.Stderr, "model-diff: -run-changed does not take -version or -local (each package has its own)")
			return 2
		}
		return runChanged(abs, *runCh, *langs, *out, append(extra, "-timeout", *timeout))
	}
	if *reviewed != "" {
		abs, _ := filepath.Abs(*repo)
		lines, err := addedReviewed(abs, *reviewed)
		if err != nil {
			return fail(err)
		}
		for _, l := range lines {
			fmt.Println(l)
		}
		return 0
	}
	if *changed != "" {
		abs, _ := filepath.Abs(*repo)
		pkgs, err := changedPackages(abs, *changed)
		if err != nil {
			return fail(err)
		}
		fmt.Println(strings.Join(pkgs, "\n"))
		return 0
	}
	if *gaps {
		if *pkg == "" {
			fmt.Fprintln(os.Stderr, "model-diff: -coverage-gaps needs -pkg")
			return 2
		}
		abs, _ := filepath.Abs(*repo)
		files, err := BuiltinFiles(abs)
		if err != nil {
			return fail(err)
		}
		list, err := coverageGaps(*pkg, files)
		if err != nil {
			return fail(err)
		}
		for _, l := range list {
			fmt.Println("gap", l)
		}
		fmt.Printf("coverage-gaps %s: %d function(s) without a row\n", *pkg, len(list))
		return 0
	}
	if *lang == "auto" {
		*lang = "go"
		if strings.HasPrefix(*pkg, "package:") {
			*lang = "dart"
		}
	}
	if *pkg == "" {
		fmt.Fprintln(os.Stderr, "model-diff: -pkg is needed")
		return 2
	}
	// a package named by /v* (any major of a Go module): the check is of one major, the one of -version
	if model.HasMajorWildcard(*pkg) {
		major, ok := majorOfVersion(*version)
		if !ok {
			fmt.Fprintf(os.Stderr, "model-diff: %s has /v* (any major): give the version to check with -version (v93.0.0): its major is the one that is checked\n", *pkg)
			return 2
		}
		*pkg = model.ConcreteMajor(*pkg, major)
	}
	if *lang == "rust" {
		absRepo, _ := filepath.Abs(*repo)
		if *out == "" {
			d, err := os.MkdirTemp("", "model-diff-rust-")
			if err != nil {
				return fail(err)
			}
			*out = d
		}
		*out, _ = filepath.Abs(*out)
		files := []string{*modelFile}
		if *modelFile == "" {
			var err error
			if files, err = BuiltinFiles(absRepo); err != nil {
				return fail(err)
			}
		}
		return realRustMain(RustOptions{Pkg: *pkg, Version: strings.TrimPrefix(*version, "v"), Features: *features, NoDefaultFeatures: *noDefault,
			Files: files, Repo: absRepo, Out: *out, Rivus: *rivus, Extractor: *rustExtractor, Timeout: *timeout})
	}
	if *lang != "go" && *lang != "dart" {
		fmt.Fprintln(os.Stderr, "model-diff: -lang is go or dart")
		return 2
	}
	absRepo, _ := filepath.Abs(*repo)
	if *out == "" {
		d, err := os.MkdirTemp("", "model-diff-")
		if err != nil {
			return fail(err)
		}
		*out = d
	}
	*out, _ = filepath.Abs(*out)
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return fail(err)
	}

	builtins, err := BuiltinFiles(absRepo)
	if err != nil {
		return fail(err)
	}
	// the files whose rows are checked
	var files []string
	builtinIn := false
	if *modelFile != "" {
		files = []string{*modelFile}
		for _, b := range builtins {
			if abs, _ := filepath.Abs(*modelFile); abs == b {
				builtinIn = true
			}
		}
	} else {
		files = builtins
		builtinIn = true
	}
	var rows []Row
	var notes []Note
	var errs []string
	var used []string
	for _, f := range files {
		rs, text, err := LoadRows(f, *lang, *pkg)
		if err != nil {
			return fail(err)
		}
		n, e := ParseNotes(f, text)
		if len(rs) > 0 {
			used = append(used, f)
			rows = append(rows, rs...)
			for _, x := range n {
				// a note of a package other than the one that is checked is not judged here (it would be stale in every other run)
				if x.Kind == "value" || model.MatchMajor(SymbolPackage(*lang, strings.SplitN(x.Symbol, "@", 2)[0]), *pkg) {
					notes = append(notes, x)
				}
			}
			errs = append(errs, e...)
		}
	}
	// Dart: only the flows rows and the sinks are checked; the other rows are listed as not judged (never as agreeing)
	var skipped []RowState
	if *lang == "dart" {
		var keep []Row
		for _, r := range rows {
			if r.Kind == RowPure {
				skipped = append(skipped, RowState{Symbol: r.Symbol, Row: r.Kind, Reason: "a pure row is not checked for Dart"})
				continue
			}
			keep = append(keep, r)
		}
		rows = keep
	}
	if len(rows) == 0 {
		fmt.Fprintf(os.Stderr, "model-diff: no row of %s in %s\n", *pkg, strings.Join(files, ", "))
		return 2
	}

	if *rivus == "" {
		*rivus = filepath.Join(*out, "rivus")
		c := exec.Command("go", "build", "-o", *rivus, "./cmd/rivus")
		c.Dir = absRepo
		if o, err := c.CombinedOutput(); err != nil {
			return fail(fmt.Errorf("go build: %v\n%s", err, o))
		}
	}
	var gen *Gen
	switch *lang {
	case "go":
		ver := *version
		if ver == "" {
			ver = repoVersion(absRepo, *pkg)
		}
		gen, err = GenGo(GoOptions{Pkg: *pkg, Out: *out, Local: *local, Version: ver, Rows: rows})
	case "dart":
		gen, err = GenDart(DartOptions{Pkg: *pkg, Out: *out, Version: *version, Rows: rows, Notes: notes, Repo: absRepo})
	}
	if err != nil {
		return fail(err)
	}

	// A: the built-in files without the rows of the package
	var filtered []string
	for i, b := range builtins {
		text, _ := os.ReadFile(b)
		f := filepath.Join(*out, fmt.Sprintf("builtin-A-%02d-%s", i, filepath.Base(b)))
		if err := os.WriteFile(f, []byte(DropPackage(string(text), *lang, *pkg)), 0o644); err != nil {
			return fail(err)
		}
		filtered = append(filtered, f)
	}
	ro := RunOptions{Rivus: *rivus, Lang: *lang, Repo: absRepo, Timeout: *timeout}
	var mf string
	if *modelFile != "" {
		mf, _ = filepath.Abs(*modelFile)
	}
	a, b, runs, err := runPair(ro, gen, mf, builtinIn, filtered)
	if err != nil {
		return fail(err)
	}
	rep := Compare(gen.Probes, gen.Skips, a, b, notes)
	rep.Judge(rows, gen.States)
	for _, st := range skipped {
		rep.Rows = append(rep.Rows, st)
		rep.Counts["rows_not_judged"]++
	}
	rep.Language, rep.Package, rep.Models, rep.Runs = *lang, *pkg, used, runs
	rep.Errors = append(append(rep.Errors, errs...), gen.Errors...)
	rep.Print(os.Stdout)
	if *jsonOut == "" {
		*jsonOut = filepath.Join(*out, "report.json")
	}
	data, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(*jsonOut, data, 0o644); err != nil {
		return fail(err)
	}
	fmt.Printf("  report: %s\n", *jsonOut)
	if rep.Failed() {
		return 1
	}
	return 0
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "model-diff:", err)
	return 2
}

// repoVersion: the version of the module of the package that go.mod of the checkout asks for ("" if it is not there).
func repoVersion(repo, pkg string) string {
	c := exec.Command("go", "list", "-m", "-f", "{{.Path}} {{.Version}}", "all")
	c.Dir = repo
	out, err := c.Output()
	if err != nil {
		return ""
	}
	best, ver := "", ""
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(l)
		if len(f) == 2 && (pkg == f[0] || strings.HasPrefix(pkg, f[0]+"/")) && len(f[0]) > len(best) {
			best, ver = f[0], f[1]
		}
	}
	return ver
}
