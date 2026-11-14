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
	fs.Parse(args)

	rules, err := c.loadRules()
	if err != nil {
		return err
	}
	l, err := c.load(fs.Args(), false)
	if err != nil {
		return err
	}
	if err := l.extract(false); err != nil {
		return err
	}
	res := sites.Enumerate(l.ir, l.texts, rules)
	out := sitesOutput{SchemaVersion: "rivus.sites/1", Target: vcs.Detect(c.dir), LoadErrors: l.gossa.LoadErrors, Sites: []*sites.Site{}}
	for _, s := range res.Sites {
		if *role == "" || s.Role == *role {
			out.Sites = append(out.Sites, s)
		}
	}
	return writeJSON(os.Stdout, out)
}
