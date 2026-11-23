// model-author: the steps of making the model of a Go or Dart package that a script can do (docs/DESIGN-package-models.md, 6.11; stage 1).
//
//	fetch the source of the package (fetch only: nothing of it is run), list the candidates by the rule table of layer 1 (rules.go),
//	match each candidate with a row or with an entry of `rejected:`, and stop with exit code 70 when one is neither.
//
// With -gen-skeleton (stage 2) it makes the skeleton of the package for the Go zoo and the template of its cells (genskel.go, skeleton.go).
// Dart (-lang dart) has the same stage 1 (dart.go; the API is read by dartapi/, the analyzer). Later stages: flows from the types, the Dart skeleton, Rust. The decisions (sink, source, sanitizer) are the
// person's or the model's: they are rows of the model file, or entries of `rejected:` in the same file.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yagihash/rivus/internal/model"
)

// Exit codes. The ones that stage 1 does not use are kept for the later stages (the numbers are the ones of the design memo).
const (
	ExitOK        = 0
	ExitUsage     = 2  // the arguments, or a model file that the loader refuses
	ExitFetch     = 10 // the fetch of the package
	ExitLoad      = 20 // reading the package, or the differential test could not run
	ExitSkeleton  = 30 // -gen-skeleton: the skeleton or the module files could not be made, or the skeleton does not build
	ExitCells     = 40 // -gen-skeleton: the cell template could not be made, or does not build
	ExitMiss      = 50 // the differential test failed (a MISS, or an EXTRA without a note)
	ExitZoo       = 60 // (not used in stage 1)
	ExitUndecided = 70 // a candidate of layer 1 has no row and is not in `rejected:`
	ExitCannotRun = 80 // (not used in stage 1: Rust)
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }

func main() { os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr)) }

type options struct {
	pkg, version, lang, model, repo, work, dartLibs, base          string
	breakdown, recall, diff, writeVerified, writeDead, genSkeleton bool
}

func realMain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("model-author", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o options
	fs.StringVar(&o.pkg, "pkg", "", "the package: a Go import path (the module path for a module whose root is the package), or a Dart package (dio, or package:dio)")
	fs.StringVar(&o.version, "version", "", "the version of the module (default: the latest)")
	fs.StringVar(&o.lang, "lang", "go", "the language (go or dart)")
	fs.StringVar(&o.dartLibs, "dart-libs", "", "Dart: read only the public libraries whose path under lib/ starts with one of these (comma separated: `storage/,pubsub/`); default: all of them")
	fs.StringVar(&o.model, "model", "", "the model file that has the rows of the package (default: the built-in files of -repo that have a row or an entry of it)")
	fs.StringVar(&o.repo, "repo", ".", "the checkout of rivus")
	fs.StringVar(&o.work, "work", "", "where the work files go (default: <repo>/.claude/work/model-author, which git ignores)")
	fs.StringVar(&o.base, "base", "origin/main", "the commit that the summary of rejected: counts the added entries against")
	fs.BoolVar(&o.breakdown, "breakdown", false, "write the breakdown of the candidates (kind x rule:word/shape: total, row, decided, undecided) to <work>/<package>-breakdown.txt")
	fs.BoolVar(&o.recall, "recall", false, "report mode: of the sink and source rows of the package, how many a rule of layer 1 finds (exit 0)")
	fs.BoolVar(&o.diff, "diff", false, "when every candidate is decided, run bench/model-diff for the package and write the material for the pull request")
	fs.BoolVar(&o.writeVerified, "write-verified", false, "with -diff, when it passes: write verified_versions of the model file")
	fs.BoolVar(&o.writeDead, "write-dead-allow", false, "add the rows of the package that the zoo does not hit to the .dead-allow of the model file (reason: dependency), and stop")
	fs.BoolVar(&o.genSkeleton, "gen-skeleton", false, "make the skeleton of the package for the Go zoo (bench/zoo-go-pkg/<name>/skel), the module files, and the template of the cells when the package has none; overwrites the generated files, never a cell")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if o.pkg == "" {
		fmt.Fprintln(stderr, "model-author: -pkg is needed")
		return ExitUsage
	}
	if o.lang != "go" && o.lang != "dart" {
		fmt.Fprintf(stderr, "model-author: -lang is go or dart (got %s; Rust comes later)\n", o.lang)
		return ExitUsage
	}
	if o.lang == "dart" {
		// `dio`, `package:dio` name the same package; the symbols of a Dart model are `package:<name>/...`
		o.pkg = "package:" + dartName(o.pkg)
		if o.genSkeleton {
			fmt.Fprintln(stderr, "model-author: -gen-skeleton is Go only for now (stage 2 for Dart comes later)")
			return ExitUsage
		}
	}
	err := run(&o, stdout)
	var ee *exitError
	if errors.As(err, &ee) {
		fmt.Fprintln(stderr, "model-author:", ee.err)
		return ee.code
	}
	if err != nil {
		fmt.Fprintln(stderr, "model-author:", err)
		return ExitUsage
	}
	return ExitOK
}

var slugRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func run(o *options, out io.Writer) error {
	repo, _ := filepath.Abs(o.repo)
	if o.work == "" {
		o.work = filepath.Join(repo, ".claude", "work", "model-author")
	}
	if err := os.MkdirAll(o.work, 0o755); err != nil {
		return err
	}
	if model.HasMajorWildcard(o.pkg) {
		// any major of the module: the run is for one, the one of -version (as bench/model-diff)
		maj, ok := majorOfVersion(o.version)
		if !ok {
			return &exitError{ExitUsage, fmt.Errorf("%s has /v* (any major): give the version to check with -version (v93.0.0): its major is the one that is checked", o.pkg)}
		}
		o.pkg = model.ConcreteMajor(o.pkg, maj)
	}
	slug := slugRe.ReplaceAllString(o.pkg, "_")

	files := []string{o.model}
	if o.model == "" {
		var err error
		if files, err = BuiltinFiles(repo); err != nil {
			return err
		}
	}
	facts, err := LoadFacts(files, o.pkg)
	if err != nil {
		return &exitError{ExitUsage, err}
	}
	target := o.model
	if target == "" && len(facts.Files) > 0 {
		target = facts.Files[0]
	}
	if o.genSkeleton {
		l, err := FetchAndLoad(filepath.Join(o.work, "fetch-"+slug), o.pkg, o.version)
		if err != nil {
			return err
		}
		return runGenSkeleton(out, o, repo, facts, l)
	}
	if target == "" && !o.recall {
		return &exitError{ExitUsage, fmt.Errorf("no row and no `rejected:` entry of %s in the built-in models: the file where the rows go is not known (use -model)", o.pkg)}
	}

	if o.writeDead {
		return writeDeadAllow(out, repo, target, o.pkg, o.lang, facts)
	}

	var l *loaded
	var api *API
	if o.lang == "dart" {
		l, api, err = DartLoad(o, repo, slug, facts)
	} else {
		l, err = FetchAndLoad(filepath.Join(o.work, "fetch-"+slug), o.pkg, o.version)
		if err == nil {
			api = Scan(l.Types, !facts.BodiesRead(o.pkg))
		}
	}
	if err != nil {
		return err
	}
	m := Match(api, facts)

	if o.recall {
		PrintRecall(out, o.pkg, l, api, facts)
		return nil
	}

	fmt.Fprintf(out, "model-author %s %s %s\n", o.lang, o.pkg, l.Version)
	fmt.Fprintf(out, "  model file: %s\n", target)
	fmt.Fprintf(out, "  layer 1: %d candidate(s): %d with a row, %d in rejected:, %d deferred:, %d undecided\n", len(api.Items), len(m.ByRow), len(m.ByRejected), len(m.ByDeferred), len(m.Undecided))
	if err := checkDeferredRefs(repo, facts); err != nil {
		return &exitError{ExitUsage, err}
	}
	if o.breakdown {
		bf := filepath.Join(o.work, slug+"-breakdown.txt")
		if err := os.WriteFile(bf, []byte(Breakdown(api, m)), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "  breakdown: %s\n", bf)
	}
	summary := RejectedSummary(repo, o.base, facts)
	sf := filepath.Join(o.work, slug+"-rejected-summary.txt")
	if err := os.WriteFile(sf, []byte(summary), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "  rejected: and deferred: summary (for the pull request): %s\n", sf)
	for _, it := range m.ByDeferred {
		fmt.Fprintf(out, "  deferred: %s %s (%s)\n", it.Kind, it.Symbol, strings.Join(it.Positions, ", "))
	}
	l2 := filepath.Join(o.work, slug+"-layer2.txt")
	if err := os.WriteFile(l2, []byte(strings.Join(api.Layer2, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "  layer 2: %d function(s) that take a string, bytes or any (a list only, no decision needed): %s\n", len(api.Layer2), l2)
	if !facts.BodiesRead(o.pkg) {
		fmt.Fprintln(out, "  calls candidates: listed (the bodies of the package are not read)")
	} else {
		fmt.Fprintln(out, "  calls candidates: not listed (the bodies of the package are read: the analysis resolves the calls of a function value or an interface; a row of calls: is for a body that is not read)")
	}
	// the rows that no rule found: a material for the rule table, never a failure
	if rec := ComputeRecall(api, facts); len(rec.SinkMissed)+len(rec.SourceMissed) > 0 {
		list := append(append([]string{}, rec.SinkMissed...), rec.SourceMissed...)
		nf := filepath.Join(o.work, slug+"-rows-not-found.txt")
		if err := os.WriteFile(nf, []byte(strings.Join(list, "\n")+"\n"), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "  rows that no rule of layer 1 finds: %d sink, %d source (material for the rule table; not a failure): %s\n", len(rec.SinkMissed), len(rec.SourceMissed), nf)
	}
	for _, r := range m.NoLongerCandidate {
		fmt.Fprintf(out, "  note: %s: rejected %s (%s) is in the API but no rule of layer 1 lists it any more\n", r.File, r.Symbol, r.Kind)
	}
	for _, d := range m.StaleDeferred {
		fmt.Fprintf(out, "  warning: %s: deferred %s (%s) is not in the API of %s any more\n", d.File, d.Symbol, d.Kind, o.pkg)
	}
	for _, r := range m.Stale {
		fmt.Fprintf(out, "  warning: %s: rejected %s (%s) is not in the API of %s any more\n", r.File, r.Symbol, r.Kind, o.pkg)
	}
	if len(m.Undecided) > 0 {
		f := filepath.Join(o.work, slug+"-undecided.yaml")
		if err := os.WriteFile(f, []byte(UndecidedYAML(o.pkg, m.Undecided)), 0o644); err != nil {
			return err
		}
		for _, it := range m.Undecided {
			fmt.Fprintf(out, "  undecided: %s %s (%s)\n", it.Kind, it.Symbol, strings.Join(it.Positions, ", "))
		}
		fmt.Fprintf(out, "  the undecided list: %s\n  add a row, or an entry of rejected: with the reason, for each\n", f)
		return &exitError{ExitUndecided, fmt.Errorf("%d undecided candidate(s) (exit %d)", len(m.Undecided), ExitUndecided)}
	}
	if !o.diff {
		return nil
	}
	return runDiff(out, o, repo, target, slug, l, api, m, summary)
}

// majorOfVersion: "93" for v93.0.0 (v0 and v1 have no element in the path: "" and false for them is not needed: they give "0"/"1", which /v* does not match).
func majorOfVersion(v string) (string, bool) {
	n, _, _ := strings.Cut(strings.TrimPrefix(v, "v"), ".")
	if n == "" || strings.Trim(n, "0123456789") != "" {
		return "", false
	}
	return n, true
}
