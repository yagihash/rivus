package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/sites"
)

// The values that a flag accepts from a fixed set. Each set is written once, here (or beside the constants it is made of): the check of the
// flag, the sentence of its help and the completion all read the same list, so a value added to a list is accepted, said and completed at once.

// langAuto is the value of -lang that reads what the directory holds.
const langAuto = "auto"

// languageNames are the languages that rivus reads.
var languageNames = []string{langGo, langDart, langRust}

// langValues are the values of one element of -lang (the flag takes a list of languages separated by commas, or auto).
var langValues = append(slices.Clone(languageNames), langAuto)

// extractLangs are the languages "rivus debug extract -lang" writes the IR of (the Dart extractor is run by its own command).
var extractLangs = []string{langGo, langRust}

// passStatsSorts are the orders of packages_top (-pass-stats-sort).
var passStatsSorts = []string{"reanalyses", "time", "functions", "elems"}

// An enumFlag is a flag whose value is one of a list.
type enumFlag struct {
	values []string
	// check says whether v is accepted (nil: it is).
	check func(v string) error
}

// enumFlags are the flags of fixed values, by the name that kind(fs, "enum:<name>", ...) gives them.
var enumFlags = map[string]enumFlag{
	"lang":            {langValues, func(v string) error { return oneOf("lang", "language", v, langValues) }},
	"extract-lang":    {extractLangs, func(v string) error { return oneOf("lang", "language", v, extractLangs) }},
	"detail":          {report.DetailLevels, report.ValidDetail},
	"pass-stats-sort": {passStatsSorts, func(v string) error { return oneOf("pass-stats-sort", "order", v, passStatsSorts) }},
	"role":            {sites.Roles, func(v string) error { return oneOf("role", "role", v, sites.Roles) }},
}

// oneOf is the check of a flag that takes one of values ("" is the default of the flags that have one, and is not checked here).
func oneOf(flagName, what, v string, values []string) error {
	if v == "" || slices.Contains(values, v) {
		return nil
	}
	return fmt.Errorf("-%s: unknown %s %q (%s)", flagName, what, v, orList(values, false))
}

// orList writes values as a sentence does: "a, b or c" (with quote, "a", "b" or "c").
func orList(values []string, quote bool) string {
	q := slices.Clone(values)
	if quote {
		for i, v := range q {
			q[i] = fmt.Sprintf("%q", v)
		}
	}
	if len(q) < 2 {
		return strings.Join(q, "")
	}
	return strings.Join(q[:len(q)-1], ", ") + " or " + q[len(q)-1]
}
