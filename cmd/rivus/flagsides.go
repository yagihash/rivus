package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

// The flags of a command are of two kinds. "rivus <command> -h" lists the ones that are used every day; the flags for diagnosis and tuning (profiles,
// limits of the analysis, the sandbox of the Rust build...) are listed with -help-all. No flag is removed by this and none behaves differently:
// it is only what -h prints. A flag that changes what the result means is a usual flag.
//
// The kind is set beside the definition of the flags, by usual(fs, names...) or diagnostic(fs, names...). A flag that is in neither, or in
// both, stops the command when it is run (and TestEveryFlagHasOneSide), so a new flag cannot be added without deciding its side.

const (
	helpAllFlag  = "help-all"
	helpAllUsage = "with -h: also list the flags for diagnosis and tuning"
)

type flagSides struct {
	usual, diag map[string]bool
	// kinds is what the value of a flag is ("dir", "file", "enum:<name>"), for the completion; a flag without one has no value completed.
	kinds map[string]string
	all   bool
	// info is what is said of a flag besides its side (describe, flagInfo).
	info map[string]flagInfo
	// header, if set, is printed by -h in place of "Usage of <name>:".
	header string
}

var flagSideOf = map[*flag.FlagSet]*flagSides{}

func sidesOf(fs *flag.FlagSet) *flagSides {
	s := flagSideOf[fs]
	if s == nil {
		s = &flagSides{usual: map[string]bool{}, diag: map[string]bool{}, kinds: map[string]string{}, info: map[string]flagInfo{}}
		flagSideOf[fs] = s
	}
	return s
}

// usual marks flags (by name) that "-h" lists.
func usual(fs *flag.FlagSet, names ...string) {
	for _, n := range names {
		sidesOf(fs).usual[n] = true
	}
}

// diagnostic marks flags (by name) that only "-h -help-all" lists.
func diagnostic(fs *flag.FlagSet, names ...string) {
	for _, n := range names {
		sidesOf(fs).diag[n] = true
	}
}

// flagInfo says what a flag is for. The list of the features that are off by default is made from it (TestOffByDefaultFlags); there is no other
// list, so no document says what the code does not.
//
// A feature (ChangesResult) is a flag that changes what the result means, and that is off (or on the way to a new default). It says when it was
// added and by what (Added, AddedBy), and what ends its being a flag: the condition to make it the default (Condition), the date or the work that
// ends it (Deadline), or why it stays off (KeepOff). One of the three is needed; "undecided" is not an answer.
//
// A flag that is not a feature says what it is instead (Not): the range of the analysis (-tests), a stop of a function that is on (-no-deps), a
// way to isolate or to try (-unsafe-host-exec, -body-replaced), the output (-v). Such a flag has no date and no condition.
//
// A flag for diagnosis (diagnostic) needs nothing. A switch inside the implementation (writing the IR to a disk is one) is not a flag of the
// user: it is an environment variable or a variable of a test, and is not in -h. Do not add a flag for it.
type flagInfo struct {
	ChangesResult bool
	// Not is what a flag that is not a feature is.
	Not string
	// Added is the date the flag was added (YYYY-MM-DD), AddedBy the PR (or the commit) that added it.
	Added, AddedBy string
	// Condition, Deadline and KeepOff: see above.
	Condition, Deadline, KeepOff string
}

// describe says what a flag is (see flagInfo).
func describe(fs *flag.FlagSet, name string, info flagInfo) {
	sidesOf(fs).info[name] = info
}

// kind says what the value of flags is, for the completion: "dir", "file", or "enum:<name>" (a value of fixed set, enumFlags). A flag without a
// kind has no value completed.
func kind(fs *flag.FlagSet, k string, names ...string) {
	for _, n := range names {
		sidesOf(fs).kinds[n] = k
	}
}

// prepareFlags is called after the flags are defined and before they are parsed: it checks that every flag has one side, defines -help-all, and
// sets what -h prints (the header is sidesOf(fs).header, if the command has one). -help-all alone prints the usage too.
func prepareFlags(fs *flag.FlagSet, args []string) {
	s := sidesOf(fs)
	var bad []string
	fs.VisitAll(func(f *flag.Flag) {
		if s.usual[f.Name] == s.diag[f.Name] {
			bad = append(bad, f.Name)
		}
	})
	for n := range s.usual {
		if fs.Lookup(n) == nil {
			bad = append(bad, n+" (not a flag)")
		}
	}
	for n := range s.diag {
		if fs.Lookup(n) == nil {
			bad = append(bad, n+" (not a flag)")
		}
	}
	for n, k := range s.kinds {
		if fs.Lookup(n) == nil {
			bad = append(bad, n+" (not a flag)")
		}
		if e, ok := strings.CutPrefix(k, "enum:"); ok {
			if _, known := enumFlags[e]; !known {
				bad = append(bad, n+" (kind "+k+": no such set of values)")
			}
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		panic(fmt.Sprintf("rivus: %s: flags without exactly one side (usual or diagnostic): %s", fs.Name(), strings.Join(bad, ", ")))
	}
	fs.Bool(helpAllFlag, false, helpAllUsage)
	usageOf = fs
	fs.Usage = printUsage
	help := false
	for _, a := range args {
		name, _, _ := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if a == "--" {
			break
		}
		switch name {
		case helpAllFlag:
			s.all = true
		case "h", "help":
			help = true
		}
	}
	if s.all && !help {
		printUsage()
		os.Exit(0)
	}
}

// usageOf is the flag set of the command that runs (a process runs one): printUsage is a plain function, so that no closure is stored in the flag set.
var usageOf *flag.FlagSet

// printUsage is the usage of the command that runs: what -h and a flag that is not known print.
func printUsage() {
	fs := usageOf
	if h := sidesOf(fs).header; h != "" {
		fmt.Fprint(fs.Output(), h)
	} else {
		fmt.Fprintf(fs.Output(), "Usage of %s:\n", fs.Name())
	}
	printFlags(fs)
}

// printFlags prints the usual flags, then (with -help-all) the flags for diagnosis and tuning, or else the line that says where they are.
func printFlags(fs *flag.FlagSet) {
	s := sidesOf(fs)
	printSide(fs, func(n string) bool { return s.usual[n] })
	if s.all && len(s.diag) > 0 {
		fmt.Fprint(fs.Output(), "\nFlags for diagnosis and tuning:\n")
		printSide(fs, func(n string) bool { return s.diag[n] })
		return
	}
	if len(s.diag) > 0 {
		fmt.Fprintf(fs.Output(), "\nThe flags for diagnosis and tuning are listed with -help-all (rivus %s -help-all).\n", fs.Name())
	}
}

func printSide(fs *flag.FlagSet, want func(name string) bool) {
	tmp := flag.NewFlagSet(fs.Name(), flag.ContinueOnError)
	tmp.SetOutput(fs.Output())
	fs.VisitAll(func(f *flag.Flag) {
		if f.Name == helpAllFlag || !want(f.Name) {
			return
		}
		tmp.Var(f.Value, f.Name, f.Usage)
		tmp.Lookup(f.Name).DefValue = f.DefValue
	})
	tmp.PrintDefaults()
}
