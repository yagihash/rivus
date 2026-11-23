package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
)

// runExtract ("rivus debug extract") writes the neutral IR of the code: as a JSON Lines stream, or
// as text a person can read.
// extractCmd is "rivus debug extract".
type extractCmd struct {
	dir, filter, lang              *string
	tests, all, text, texts, check *bool
	impls                          multiFlag
	bodyReplaced                   multiFlag
	ro                             analyzeOpts
}

func extractFlags(fs *flag.FlagSet) *extractCmd {
	c := &extractCmd{}
	c.dir = fs.String("C", ".", "directory to extract from")
	c.tests = fs.Bool("tests", false, "include _test.go files")
	c.all = fs.Bool("all", false, "also extract the functions of dependencies")
	c.text = fs.Bool("text", false, "print readable text instead of the JSON Lines stream")
	c.filter = fs.String("func", "", "only the functions whose ID contains this text")
	fs.Var(&c.impls, "implementations", "interface method whose implementations are recorded (repeatable)")
	c.texts = fs.Bool("texts", false, "also write the texts the core needs (snippets, expressions, bodies), as an extractor in another process has to")
	c.check = fs.Bool("check", true, "validate the IR before writing it")
	c.lang = fs.String("lang", langGo, orList(extractLangs, false)+": the IR of the Rust extractor as it writes it (under the sandbox, as analyze -lang rust does; the flags below are the ones of analyze)")
	ro := &c.ro
	fs.BoolVar(&ro.unsafeHostExec, "unsafe-host-exec", false, "-lang rust: run the extractor on this machine (code you trust only; also RIVUS_UNSAFE_HOST_EXEC=1)")
	fs.StringVar(&ro.rustExtractor, "rust-extractor", "", "-lang rust: command of the extractor for -unsafe-host-exec")
	fs.DurationVar(&ro.isolationTimeout, "isolation-timeout", 30*time.Minute, "-lang rust: stop each stage after this long")
	fs.BoolVar(&ro.noFetch, "no-fetch", false, "-lang rust: do not fetch the dependencies")
	fs.BoolVar(&ro.noDeps, "no-deps", false, "-lang rust: do not read the dependencies")
	fs.Var(&ro.isolationEnv, "isolation-env", "-lang rust: NAME=VALUE for the build (repeatable)")
	fs.Var(&ro.isolationRead, "isolation-read", "-lang rust: PATH that the build may read (repeatable)")
	fs.Int64Var(&ro.isolationMaxOut, "isolation-max-output", 4<<30, "-lang rust: the most bytes the extractor may write")
	fs.Var(&c.bodyReplaced, "body-replaced", "-lang rust: the crate whose body the extractor does not read (the name with - or _; the extractor says how many functions it has, as body_not_read) (repeatable)")
	fs.Var(&ro.allowFetchHosts, "allow-fetch-host", "-lang rust: HOST the fetch stage may get from although it is not crates.io (git dependencies on any host, other registries). The repository can send the fetch on to other places (its submodules, redirects); that is not prevented until the network of the fetch stage is narrowed (repeatable)")
	usual(fs, "C", "tests", "all", "text", "func", "implementations", "lang", "unsafe-host-exec", "rust-extractor", "no-deps", "isolation-env", "isolation-read")
	diagnostic(fs, "texts", "check", "isolation-timeout", "no-fetch", "isolation-max-output", "allow-fetch-host", "body-replaced")
	kind(fs, "dir", "C")
	kind(fs, "enum:extract-lang", "lang")
	return c
}

func runExtract(args []string) error {
	fs := flag.NewFlagSet("debug extract", flag.ExitOnError)
	c := extractFlags(fs)
	prepareFlags(fs, args)
	fs.Parse(args)
	dir, tests, all, text, filter, impls, texts, check, lang, ro := c.dir, c.tests, c.all, c.text, c.filter, c.impls, c.texts, c.check, c.lang, &c.ro

	if *lang == langRust {
		ro.dir = *dir
		ro.tests = *tests
		ro.extractorBodyReplaced = c.bodyReplaced
		out, err := ro.extractRaw(langRust)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(out)
		return err
	}
	if err := enumFlags["extract-lang"].check(*lang); err != nil || *lang == "" {
		return fmt.Errorf("-lang: %q is %s here (the Dart extractor is run by its own command)", *lang, orList(extractLangs, false))
	}

	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	prog, err := gossa.Load(*dir, patterns, gossa.Options{Tests: *tests, NaiveForm: true})
	if err != nil {
		return err
	}
	for _, e := range prog.LoadErrors {
		fmt.Fprintf(os.Stderr, "rivus: %s: %s\n", e.Package, e.Message)
	}
	out, err := goir.Extract(prog, goir.Options{All: *all, Implementations: impls})
	if err != nil {
		return err
	}
	if *texts {
		goir.AddTexts(goir.NewTexts(prog), out)
	}
	if *check {
		if err := out.Validate(); err != nil {
			return fmt.Errorf("the extracted IR is not valid: %w", err)
		}
	}
	if *filter != "" {
		var keep []*ir.Func
		for _, f := range out.Funcs {
			if strings.Contains(string(f.ID), *filter) {
				keep = append(keep, f)
			}
		}
		out.Funcs = keep
	}
	if *text {
		for _, f := range out.Funcs {
			fmt.Println(f.Format())
		}
		return nil
	}
	return ir.Write(os.Stdout, out)
}
