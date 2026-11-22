// model-author: the steps of making the model of a Go package that a script can do (docs/DESIGN-package-models.md, 6.11; stage 1).
//
//	fetch the source of the package (fetch only: nothing of it is run), list the candidates by the rule table of layer 1 (rules.go),
//	match each candidate with a row or with an entry of `rejected:`, and stop with exit code 70 when one is neither.
//
// What it does not do (later stages): flows from the types, skeletons, cells, Dart, Rust. The decisions (sink, source, sanitizer) are the
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
)

// Exit codes. The ones that stage 1 does not use are kept for the later stages (the numbers are the ones of the design memo).
const (
	ExitOK        = 0
	ExitUsage     = 2  // the arguments, or a model file that the loader refuses
	ExitFetch     = 10 // the fetch of the package
	ExitLoad      = 20 // reading the package, or the differential test could not run
	ExitSkeleton  = 30 // (not used in stage 1)
	ExitCells     = 40 // (not used in stage 1)
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
	pkg, version, lang, model, repo, work  string
	recall, diff, writeVerified, writeDead bool
}

func realMain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("model-author", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o options
	fs.StringVar(&o.pkg, "pkg", "", "the Go package (an import path; the module path for a module whose root is the package)")
	fs.StringVar(&o.version, "version", "", "the version of the module (default: the latest)")
	fs.StringVar(&o.lang, "lang", "go", "the language (stage 1: go)")
	fs.StringVar(&o.model, "model", "", "the model file that has the rows of the package (default: the built-in files of -repo that have a row or an entry of it)")
	fs.StringVar(&o.repo, "repo", ".", "the checkout of rivus")
	fs.StringVar(&o.work, "work", "", "where the work files go (default: <repo>/.claude/work/model-author, which git ignores)")
	fs.BoolVar(&o.recall, "recall", false, "report mode: of the sink and source rows of the package, how many a rule of layer 1 finds (exit 0)")
	fs.BoolVar(&o.diff, "diff", false, "when every candidate is decided, run bench/model-diff for the package and write the material for the pull request")
	fs.BoolVar(&o.writeVerified, "write-verified", false, "with -diff, when it passes: write verified_versions of the model file")
	fs.BoolVar(&o.writeDead, "write-dead-allow", false, "add the rows of the package that the zoo does not hit to the .dead-allow of the model file (reason: dependency), and stop")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if o.pkg == "" {
		fmt.Fprintln(stderr, "model-author: -pkg is needed")
		return ExitUsage
	}
	if o.lang != "go" {
		fmt.Fprintf(stderr, "model-author: stage 1 does Go only (got -lang %s)\n", o.lang)
		return ExitUsage
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
	if target == "" && !o.recall {
		return &exitError{ExitUsage, fmt.Errorf("no row and no `rejected:` entry of %s in the built-in models: the file where the rows go is not known (use -model)", o.pkg)}
	}

	if o.writeDead {
		return writeDeadAllow(out, repo, target, o.pkg, facts)
	}

	l, err := FetchAndLoad(filepath.Join(o.work, "fetch-"+slug), o.pkg, o.version)
	if err != nil {
		return err
	}
	api := Scan(l.Types)
	m := Match(api, facts)

	if o.recall {
		PrintRecall(out, o.pkg, l, api, facts)
		return nil
	}

	fmt.Fprintf(out, "model-author go %s %s\n", o.pkg, l.Version)
	fmt.Fprintf(out, "  model file: %s\n", target)
	fmt.Fprintf(out, "  layer 1: %d candidate(s): %d with a row, %d in rejected:, %d undecided\n", len(api.Items), len(m.ByRow), len(m.ByRejected), len(m.Undecided))
	l2 := filepath.Join(o.work, slug+"-layer2.txt")
	if err := os.WriteFile(l2, []byte(strings.Join(api.Layer2, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "  layer 2: %d function(s) that take a string, bytes or any (a list only, no decision needed): %s\n", len(api.Layer2), l2)
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
	return runDiff(out, o, repo, target, slug, l, api, m)
}
