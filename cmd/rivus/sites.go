package main

import (
	"flag"
	"os"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/vcs"
)

// sitesOutput is the JSON written by "rivus sites".
type sitesOutput struct {
	SchemaVersion string         `json:"schema_version"`
	Target        *vcs.Target    `json:"target,omitempty"`
	Sites         []*sites.Site  `json:"sites"`
	LoadErrors    []ir.LoadError `json:"load_errors,omitempty"`
}

func runSites(args []string) error {
	fs := flag.NewFlagSet("sites", flag.ExitOnError)
	var c common
	c.register(fs)
	role := fs.String("role", "", `only "source" or "sink" sites`)
	irFile := fs.String("ir", "", "list the sites of the neutral IR in this JSON Lines file (from an extractor of another language)")
	fs.Parse(args)

	rules, err := c.loadRules()
	if err != nil {
		return err
	}
	var l *loaded
	if *irFile != "" {
		l, err = readIR(*irFile)
	} else {
		l, err = c.load(fs.Args(), false)
		if err == nil {
			err = l.extract(false)
		}
	}
	if err != nil {
		return err
	}
	expandSubtypes(rules, l.ir)
	res := sites.Enumerate(l.ir, l.texts, rules)
	out := sitesOutput{SchemaVersion: "rivus.sites/1", Target: vcs.Detect(c.dir), LoadErrors: l.loadErrors(), Sites: []*sites.Site{}}
	for _, s := range res.Sites {
		if *role == "" || s.Role == *role {
			out.Sites = append(out.Sites, s)
		}
	}
	return writeJSON(os.Stdout, out)
}
