package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yagihash/rivus/internal/report"
)

func runModels(args []string) error {
	if len(args) == 0 || args[0] != "coverage" {
		return fmt.Errorf("usage: rivus models coverage [flags] [packages]")
	}
	fs := flag.NewFlagSet("models coverage", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	index := fs.String("index", "", "use a saved index instead of analysing the source")
	top := fs.Int("top", 0, "only the first n symbols (0: all)")
	yaml := fs.Bool("yaml", false, "write a rule file to fill in instead of JSON")
	fs.Parse(args[1:])

	var rep *report.Report
	var err error
	if *index != "" {
		rep, err = report.Load(*index)
	} else {
		rep, err = o.analyze(fs.Args())
	}
	if err != nil {
		return err
	}
	cov := rep.Coverage()
	if *yaml {
		_, err := fmt.Fprint(os.Stdout, cov.YAML(*top))
		return err
	}
	if *top > 0 && len(cov.Symbols) > *top {
		cov.Symbols = cov.Symbols[:*top]
	}
	return writeJSON(os.Stdout, cov)
}
