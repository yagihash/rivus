package main

import (
	"flag"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// leaves lists the commands that run something, with the words that name them ("graph callers").
func leaves(cs []*command, prefix []string) (out []struct {
	path []string
	c    *command
}) {
	for _, c := range cs {
		p := append(slices.Clone(prefix), c.name)
		if len(c.sub) > 0 {
			out = append(out, leaves(c.sub, p)...)
			continue
		}
		out = append(out, struct {
			path []string
			c    *command
		}{p, c})
	}
	return out
}

// A name that the table does not have is an unknown command: the table is the only way to add one.
func TestUnknownCommand(t *testing.T) {
	for _, args := range [][]string{{"nosuch"}, {"completions"}, {"Analyze"}} {
		out, err := exec.Command(binary, args...).CombinedOutput()
		ee, ok := err.(*exec.ExitError)
		if !ok || ee.ExitCode() != 2 || !strings.Contains(string(out), "unknown command") {
			t.Errorf("rivus %v: %v, %s", args, err, out)
		}
	}
}

// "rivus help" lists the commands of the table, and only those.
func TestHelpListsTheTable(t *testing.T) {
	out, err := exec.Command(binary, "help").Output()
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "  ") && !strings.Contains(l, "<") {
			listed[strings.Fields(l)[0]] = true
		}
	}
	for _, c := range commands {
		if !listed[c.name] {
			t.Errorf("rivus help does not list %s:\n%s", c.name, out)
		}
		delete(listed, c.name)
	}
	if len(listed) > 0 {
		t.Errorf("rivus help lists what the table does not have: %v", listed)
	}
}

// The commands that the tests of the flags go through (flagCommands) are the commands of the table that have flags: a command added to the table with
// flags is in them, and the other way round.
func TestFlagCommandsAreTheTable(t *testing.T) {
	var got [][]string
	for _, l := range leaves(commands, nil) {
		if l.c.flags != nil {
			got = append(got, l.path)
		}
	}
	want := slices.Clone(flagCommands)
	less := func(a, b []string) int { return strings.Compare(strings.Join(a, " "), strings.Join(b, " ")) }
	slices.SortFunc(got, less)
	slices.SortFunc(want, less)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the commands of the table that have flags:\n%v\nflagCommands:\n%v", got, want)
	}
}

// Every flag of every command has a side, and a kind of value that names a flag that exists and a set of values that exists. The flags are listed the
// way the completion lists them: on an empty flag set, without running anything.
func TestFlagsOfTheTable(t *testing.T) {
	for _, l := range leaves(commands, nil) {
		if l.c.flags == nil {
			continue
		}
		fs := flag.NewFlagSet(strings.Join(l.path, " "), flag.ContinueOnError)
		l.c.flags(fs)
		s := sidesOf(fs)
		fs.VisitAll(func(f *flag.Flag) {
			if s.usual[f.Name] == s.diag[f.Name] {
				t.Errorf("%s: -%s has no side, or two", fs.Name(), f.Name)
			}
		})
		for n, k := range s.kinds {
			if fs.Lookup(n) == nil {
				t.Errorf("%s: kind %q for -%s, which is not a flag", fs.Name(), k, n)
			}
			if e, ok := strings.CutPrefix(k, "enum:"); ok {
				if _, known := enumFlags[e]; !known {
					t.Errorf("%s: -%s: no set of values %q", fs.Name(), n, e)
				}
			} else if k != "dir" && k != "file" {
				t.Errorf("%s: -%s: unknown kind %q", fs.Name(), n, k)
			}
		}
	}
}

// The values of a flag of fixed values are the ones that its check accepts: the list that the help and the completion print is the list of the check.
func TestEnumValuesAreTheChecked(t *testing.T) {
	for name, e := range enumFlags {
		if len(e.values) == 0 {
			t.Errorf("%s: no values", name)
		}
		for _, v := range e.values {
			if err := e.check(v); err != nil {
				t.Errorf("%s: %q is listed, and the check says %v", name, v, err)
			}
		}
		if e.check("no-such-value") == nil {
			t.Errorf("%s: the check accepts a value that is not listed", name)
		}
	}
	// the flags that use the sets name them in their usage
	for _, c := range []struct{ enum, command, flag string }{
		{"lang", "analyze", "lang"}, {"detail", "analyze", "detail"}, {"pass-stats-sort", "analyze", "pass-stats-sort"}, {"role", "sites", "role"},
	} {
		fs := flag.NewFlagSet(c.command, flag.ContinueOnError)
		find(commands, c.command).flags(fs)
		if got := sidesOf(fs).kinds[c.flag]; got != "enum:"+c.enum {
			t.Errorf("%s -%s: kind %q, want enum:%s", c.command, c.flag, got, c.enum)
		}
		usage := fs.Lookup(c.flag).Usage
		for _, v := range enumFlags[c.enum].values {
			if !strings.Contains(usage, v) {
				t.Errorf("%s -%s: the help does not say %q", c.command, c.flag, v)
			}
		}
	}
}
