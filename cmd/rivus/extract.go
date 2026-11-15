package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
)

// runExtract writes the neutral IR of the code: as a JSON Lines stream, or
// as text a person can read.
func runExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	dir := fs.String("C", ".", "directory to extract from")
	tests := fs.Bool("tests", false, "include _test.go files")
	all := fs.Bool("all", false, "also extract the functions of dependencies")
	text := fs.Bool("text", false, "print readable text instead of the JSON Lines stream")
	filter := fs.String("func", "", "only the functions whose ID contains this text")
	impls := multiFlag{}
	fs.Var(&impls, "implementations", "interface method whose implementations are recorded (repeatable)")
	texts := fs.Bool("texts", false, "also write the texts the core needs (snippets, expressions, bodies), as an extractor in another process has to")
	check := fs.Bool("check", true, "validate the IR before writing it")
	fs.Parse(args)

	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	prog, err := gossa.Load(*dir, patterns, gossa.Options{Tests: *tests, NaiveForm: true})
	if err != nil {
		return err
	}
	for _, e := range prog.LoadErrors {
		fmt.Fprintf(os.Stderr, "rivus: %s\n", e)
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
