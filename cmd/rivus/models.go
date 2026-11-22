package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yagihash/rivus/internal/report"
)

// coverageCmd is "rivus models coverage".
type coverageCmd struct {
	o     analyzeOpts
	index *string
	top   *int
	yaml  *bool
}

func coverageFlags(fs *flag.FlagSet) *coverageCmd {
	c := &coverageCmd{}
	c.o.register(fs)
	c.index = fs.String("index", "", "use a saved index instead of analysing the source")
	c.top = fs.Int("top", 0, "only the first n symbols (0: all)")
	c.yaml = fs.Bool("yaml", false, "write a rule file to fill in instead of JSON")
	usual(fs, "index", "top", "yaml")
	kind(fs, "file", "index")
	return c
}

func runModelsCoverage(args []string) error {
	fs := flag.NewFlagSet("models coverage", flag.ExitOnError)
	c := coverageFlags(fs)
	prepareFlags(fs, args)
	fs.Parse(args)
	o, index, top, yaml := &c.o, c.index, c.top, c.yaml

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
