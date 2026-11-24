package main

import (
	"flag"
	"regexp"
	"strings"
	"testing"
)

var dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// The commands that run the analysis, whose flags can change the result.
var analysisCommands = []string{"analyze", "index", "query", "resolve", "sites"}

// TestOffByDefaultFlags is where the list of the features that are off by default is kept (flagInfo). It fails when
//   - a flag that changes the result lacks the date it was added, what added it, or the condition, the deadline or the reason to stay off;
//   - a flag that is not a feature does not say what it is;
//   - a flag of the user (not a diagnostic flag) that is a switch, off by default, is not described at all (a new feature cannot be added unsaid);
//   - a description is of a flag that does not exist.
func TestOffByDefaultFlags(t *testing.T) {
	t.Parallel()
	for _, c := range commands {
		if !contains(analysisCommands, c.name) {
			continue
		}
		fs := flag.NewFlagSet(c.name, flag.ContinueOnError)
		c.flags(fs)
		s := sidesOf(fs)
		for name, in := range s.info {
			f := fs.Lookup(name)
			if f == nil {
				t.Errorf("%s: -%s is described but is not a flag", c.name, name)
				continue
			}
			if !in.ChangesResult {
				if in.Not == "" {
					t.Errorf("%s: -%s: not a feature, and does not say what it is (flagInfo.Not)", c.name, name)
				}
				if in.Added != "" || in.Condition != "" || in.Deadline != "" || in.KeepOff != "" {
					t.Errorf("%s: -%s: is not a feature but has a date or a condition", c.name, name)
				}
				continue
			}
			if !dateRE.MatchString(in.Added) {
				t.Errorf("%s: -%s changes the result: Added must be a date YYYY-MM-DD, got %q", c.name, name, in.Added)
			}
			if strings.TrimSpace(in.AddedBy) == "" {
				t.Errorf("%s: -%s changes the result: AddedBy (the PR or the commit) is empty", c.name, name)
			}
			if strings.TrimSpace(in.Condition+in.Deadline+in.KeepOff) == "" {
				t.Errorf("%s: -%s changes the result: it needs Condition, Deadline or KeepOff", c.name, name)
			}
			if f.DefValue != "false" && f.DefValue != "" && f.DefValue != "0" {
				t.Errorf("%s: -%s is described as off by default but its default is %q", c.name, name, f.DefValue)
			}
			if s.diag[name] {
				t.Errorf("%s: -%s changes the result but is a diagnostic flag; a flag that changes the result is a usual flag (flagsides.go)", c.name, name)
			}
		}
		fs.VisitAll(func(f *flag.Flag) {
			if _, ok := s.info[f.Name]; ok || s.diag[f.Name] || f.DefValue != "false" {
				return
			}
			if b, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !b.IsBoolFlag() {
				return
			}
			t.Errorf("%s: -%s is a switch that is off by default and is not described: say whether it changes the result (describe, flagInfo)", c.name, f.Name)
		})
	}
	// The features named in the docs of the project are in the code.
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	analyzeFlags(fs)
	for _, name := range []string{"char-taint", "numeric-taint"} {
		if !sidesOf(fs).info[name].ChangesResult {
			t.Errorf("analyze: -%s is a feature and must be described as one", name)
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
