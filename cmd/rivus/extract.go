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
func runExtract(args []string) error {
	fs := flag.NewFlagSet("debug extract", flag.ExitOnError)
	dir := fs.String("C", ".", "directory to extract from")
	tests := fs.Bool("tests", false, "include _test.go files")
	all := fs.Bool("all", false, "also extract the functions of dependencies")
	text := fs.Bool("text", false, "print readable text instead of the JSON Lines stream")
	filter := fs.String("func", "", "only the functions whose ID contains this text")
	impls := multiFlag{}
	fs.Var(&impls, "implementations", "interface method whose implementations are recorded (repeatable)")
	texts := fs.Bool("texts", false, "also write the texts the core needs (snippets, expressions, bodies), as an extractor in another process has to")
	check := fs.Bool("check", true, "validate the IR before writing it")
	lang := fs.String("lang", "go", "go, or rust: the IR of the Rust extractor as it writes it (under the sandbox, as analyze -lang rust does; the flags below are the ones of analyze)")
	var ro analyzeOpts
	fs.BoolVar(&ro.unsafeHostExec, "unsafe-host-exec", false, "-lang rust: run the extractor on this machine (code you trust only; also RIVUS_UNSAFE_HOST_EXEC=1)")
	fs.StringVar(&ro.rustExtractor, "rust-extractor", "", "-lang rust: command of the extractor for -unsafe-host-exec")
	fs.DurationVar(&ro.isolationTimeout, "isolation-timeout", 30*time.Minute, "-lang rust: stop each stage after this long")
	fs.BoolVar(&ro.noFetch, "no-fetch", false, "-lang rust: do not fetch the dependencies")
	fs.BoolVar(&ro.noDeps, "no-deps", false, "-lang rust: do not read the dependencies")
	fs.Var(&ro.isolationEnv, "isolation-env", "-lang rust: NAME=VALUE for the build (repeatable)")
	fs.Var(&ro.isolationRead, "isolation-read", "-lang rust: PATH that the build may read (repeatable)")
	fs.Int64Var(&ro.isolationMaxOut, "isolation-max-output", 4<<30, "-lang rust: the most bytes the extractor may write")
	fs.Var(&ro.allowFetchHosts, "allow-fetch-host", "-lang rust: HOST the fetch stage may get from although it is not crates.io (git dependencies on any host, other registries). The repository can send the fetch on to other places (its submodules, redirects); that is not prevented until the network of the fetch stage is narrowed (repeatable)")
	usual(fs, "C", "tests", "all", "text", "func", "implementations", "lang", "unsafe-host-exec", "rust-extractor", "no-deps", "isolation-env", "isolation-read")
	diagnostic(fs, "texts", "check", "isolation-timeout", "no-fetch", "isolation-max-output", "allow-fetch-host")
	prepareFlags(fs, args)
	fs.Parse(args)

	if *lang == langRust {
		ro.dir = *dir
		ro.tests = *tests
		out, err := ro.extractRaw(langRust)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(out)
		return err
	}
	if *lang != langGo {
		return fmt.Errorf("-lang: %q is go or rust here (the Dart extractor is run by its own command)", *lang)
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
