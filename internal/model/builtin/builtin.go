// Package builtin embeds the built-in rule sets. They are ordinary rule
// files, written in the same format as user rules.
package builtin

import (
	"embed"
	"fmt"
	"sort"

	"github.com/yagihash/rivus/internal/model"
)

//go:embed *.yaml
var files embed.FS

// Load parses every built-in rule set, in file name order.
func Load() ([]*model.RuleSet, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var out []*model.RuleSet
	for _, n := range names {
		f, err := files.Open(n)
		if err != nil {
			return nil, err
		}
		rs, err := model.Parse(f, "builtin:"+n)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("built-in rules are invalid: %w", err)
		}
		out = append(out, rs)
	}
	return out, nil
}
