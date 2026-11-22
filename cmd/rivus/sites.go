package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/vcs"
)

// sitesOutput is the JSON written by "rivus sites".
type sitesOutput struct {
	SchemaVersion string         `json:"schema_version"`
	Target        *vcs.Target    `json:"target,omitempty"`
	Sites         []*sites.Site  `json:"sites"`
	LoadErrors    []ir.LoadError `json:"load_errors,omitempty"`
	// SkippedLanguages are the languages of the directory that were not read, and why (the same as
	// summary.unread.skipped_languages of analyze): a list of sites without them is not the sites of the directory.
	SkippedLanguages []report.SkippedLanguage `json:"skipped_languages,omitempty"`
}

func runSites(args []string) error {
	fs := flag.NewFlagSet("sites", flag.ExitOnError)
	var o analyzeOpts
	o.common.register(fs)
	o.registerLanguages(fs)
	role := fs.String("role", "", `only "source" or "sink" sites`)
	irFile := fs.String("ir", "", "list the sites of the neutral IR in this JSON Lines file (from an extractor of another language)")
	maxImpl := fs.Int("max-implementations", model.DefaultMaxImplementations, "default limit of match: implementations")
	usual(fs, "role")
	diagnostic(fs, "ir", "max-implementations")
	prepareFlags(fs, args)
	fs.Parse(args)
	o.irFile, o.maxImpl = *irFile, *maxImpl
	if *role != "" && *role != sites.RoleSource && *role != sites.RoleSink {
		return fmt.Errorf("-role: unknown role %q (source or sink)", *role)
	}

	rules, err := o.loadRules()
	if err != nil {
		return err
	}
	// the languages are chosen as analyze chooses them (-lang, auto: what the directory holds)
	l, err := o.read(fs.Args(), rules, func(string) {})
	if err != nil {
		return err
	}
	expandSubtypes(rules, l.ir)
	res := sites.Enumerate(l.ir, l.texts, rules)
	out := sitesOutput{SchemaVersion: "rivus.sites/1", Target: o.target(*irFile != ""), LoadErrors: l.loadErrors(), Sites: []*sites.Site{}, SkippedLanguages: o.skipped}
	for _, s := range res.Sites {
		if *role == "" || s.Role == *role {
			out.Sites = append(out.Sites, s)
		}
	}
	return writeJSON(os.Stdout, out)
}
