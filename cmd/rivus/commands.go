package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
)

// A command is a line of the table of commands. The table is the one place that says what commands rivus has: the dispatch of main, "rivus help", the
// usage of a command that has commands under it, and the completion of a shell all read it. A command that is not in the table is "unknown command", so
// adding one means adding it here, and the help and the completion know it from then on.
type command struct {
	name string
	// summary is the line of the list in "rivus help" (and of the list of the parent).
	summary string
	// usual says whether it is a command of every day: the others are listed apart in "rivus help".
	usual bool
	// flags defines the flags of the command on fs, and does nothing else (no side effect, no reading of the environment): the completion calls it
	// on an empty flag set, to list the flags. nil: the command has no flags.
	flags func(fs *flag.FlagSet)
	// run runs the command on the arguments that follow its name (it parses its own flags). Nil for a command that has commands under it.
	run func(args []string) error
	// sub are the commands under this one ("rivus models coverage").
	sub []*command
	// args is what the arguments that are not flags are: "" (none, or nothing that completes), or "packages" (files and directories).
	args string
	// usage, if set, is the usage of a command with commands under it in place of the one made from the table.
	usage string
}

// commands is the table of the commands of rivus.
var commands []*command

func init() {
	commands = []*command{
		{name: "sites", usual: true, args: "packages", flags: func(fs *flag.FlagSet) { sitesFlags(fs) }, run: runSites,
			summary: "list the places where source and sink rules match (fast)"},
		{name: "analyze", usual: true, args: "packages", flags: func(fs *flag.FlagSet) { analyzeFlags(fs) }, run: runAnalyze,
			summary: "find the flows from sources to sinks across functions and packages"},
		{name: "query", usual: true, args: "packages", flags: func(fs *flag.FlagSet) { queryFlags(fs) }, run: runQuery,
			summary: "answer a one-sided question about a sink or a source"},
		{name: "index", usual: true, args: "packages", flags: func(fs *flag.FlagSet) { indexFlags(fs) }, run: runIndex,
			summary: "read the code once and save its call graph (and with -taint, the analysis) for later queries"},
		{name: "graph", usual: true, usage: graphUsage,
			summary: "search the call graph saved by index: calls, not the flow of data",
			sub: []*command{
				{name: "info", summary: "what the file holds, when and from which source it was made, and (with -C) whether the source is still the same",
					flags: func(fs *flag.FlagSet) { graphInfoFlags(fs) }, run: runGraphInfo},
				{name: "callers", summary: "the functions that call the target (backwards)",
					flags: func(fs *flag.FlagSet) { graphQueryFlags(fs, "callers") }, run: func(a []string) error { return runGraphQuery("callers", a) }},
				{name: "callees", summary: "the functions the target calls (forwards)",
					flags: func(fs *flag.FlagSet) { graphQueryFlags(fs, "callees") }, run: func(a []string) error { return runGraphQuery("callees", a) }},
				{name: "impls", summary: "the functions that the calls through an interface method may reach",
					flags: func(fs *flag.FlagSet) { graphQueryFlags(fs, "impls") }, run: func(a []string) error { return runGraphQuery("impls", a) }},
				{name: "dispatchers", summary: "the calls through an interface (or a function value) that list the target as a candidate",
					flags: func(fs *flag.FlagSet) { graphQueryFlags(fs, "dispatchers") }, run: func(a []string) error { return runGraphQuery("dispatchers", a) }},
				{name: "path", summary: "up to K ways from one target to the other, shortest first",
					flags: func(fs *flag.FlagSet) { graphQueryFlags(fs, "path") }, run: func(a []string) error { return runGraphQuery("path", a) }},
			}},
		{name: "resolve", usual: true, args: "packages", flags: func(fs *flag.FlagSet) { resolveFlags(fs) }, run: runResolve,
			summary: "find where the places seen earlier (bookmarks) are in the current code"},
		{name: "models", usual: true, summary: "look at the models of functions",
			sub: []*command{
				{name: "coverage", summary: "which functions need a model most", args: "packages",
					flags: func(fs *flag.FlagSet) { coverageFlags(fs) }, run: runModelsCoverage},
				{name: "versions", summary: "the versions of the packages that the models were checked against (verified_versions)",
					flags: func(fs *flag.FlagSet) { versionsFlags(fs) }, run: runModelsVersions},
			}},
		{name: "setup", usual: true, summary: "prepare what reading a language needs",
			sub: []*command{
				{name: "rust", summary: "the toolchain and the extractor that reading Rust needs (on purpose, outside the sandbox)",
					flags: func(fs *flag.FlagSet) { setupRustFlags(fs) }, run: runSetupRust},
				{name: "dart", summary: "compile the Dart extractor (a start in a fraction of a second, instead of 5 s or more with dart run)",
					flags: func(fs *flag.FlagSet) { setupDartFlags(fs) }, run: runSetupDart},
			}},
		{name: "cache", usual: true, summary: "list and remove the places of the dependencies of Rust that rivus fetched",
			sub: []*command{
				{name: "list", summary: "the places (path, size, last used)", run: runCacheList},
				{name: "clean", summary: "list, and after a question remove, the places last used more than -older-than days ago",
					flags: func(fs *flag.FlagSet) { cacheCleanFlags(fs) }, run: runCacheClean},
			}},
		{name: "debug", summary: "commands for investigation and development; not for everyday use",
			sub: []*command{
				{name: "extract", summary: "write the neutral IR of the code (JSON Lines, or -text to read it)", args: "packages",
					flags: func(fs *flag.FlagSet) { extractFlags(fs) }, run: runExtract},
			}},
		{name: "completion", summary: "print the completion script of a shell (zsh)", sub: completionShells},
		{name: "version", summary: "print the version, and the source each extractor was built from (also --version)",
			flags: func(fs *flag.FlagSet) { _ = versionFlags(fs, &analyzeOpts{}) }, run: runVersionCommand},
	}
}

// find returns the command that is called name among cs, or nil.
func find(cs []*command, name string) *command {
	for _, c := range cs {
		if c.name == name {
			return c
		}
	}
	return nil
}

// names lists the names of cs.
func names(cs []*command) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.name)
	}
	return out
}

// usageText is "rivus help": made from the table.
func usageText() string {
	var b strings.Builder
	b.WriteString("rivus - taint analysis query engine\n\nUsage:\n  rivus <command> [flags] [packages]\n\nCommands:\n")
	var usual, other []*command
	for _, c := range commands {
		if c.usual {
			usual = append(usual, c)
		} else {
			other = append(other, c)
		}
	}
	width := 0
	for _, c := range commands {
		width = max(width, len(c.name))
	}
	list := func(cs []*command) {
		for _, c := range cs {
			fmt.Fprintf(&b, "  %-*s  %s\n", width, c.name, c.summary)
		}
	}
	list(usual)
	if len(other) > 0 {
		b.WriteString("\nOther commands:\n")
		list(other)
	}
	b.WriteString("\nRun \"rivus <command> -h\" for the flags of a command (the flags for diagnosis and tuning: -help-all).\n")
	b.WriteString("\nExit codes: 0 ok (a finding is not an error), 1 error, 2 usage, 3 Rust declined, 4 analysis incomplete (the time limit -timeout or the pass limit was reached: the result is written, and summary.incomplete says why), 130 interrupted.\n")
	return b.String()
}

// groupUsage is the usage of a command that has commands under it ("rivus models"): made from the table, unless the command brings its own.
func groupUsage(c *command) string {
	if c.usage != "" {
		return c.usage
	}
	var b strings.Builder
	packages := ""
	for _, s := range c.sub {
		if s.args != "" {
			packages = " [packages]"
		}
	}
	fmt.Fprintf(&b, "rivus %s - %s\n\nUsage:\n  rivus %s <command> [flags]%s\n\nCommands:\n", c.name, c.summary, c.name, packages)
	width := 0
	for _, s := range c.sub {
		width = max(width, len(s.name))
	}
	for _, s := range c.sub {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, s.name, s.summary)
	}
	fmt.Fprintf(&b, "\nRun \"rivus %s <command> -h\" for the flags of a command.\n", c.name)
	return b.String()
}

// runGroup runs the command under c that args name (c has commands under it). Without one it prints the usage and exits with 2; -h prints it and ends
// with 0 (checkSubcommand).
func runGroup(c *command, args []string) error {
	if checkSubcommand(c.name, args, groupUsage(c), names(c.sub)...) {
		return nil
	}
	return find(c.sub, args[0]).run(args[1:])
}

// runCommand runs the command of the table that name says, with the arguments after it; unknown is false when the table has none.
func runCommand(name string, args []string) (known bool, err error) {
	// A function of the IR that cannot be read stops the run with an internal error (exit code 1): the reader that met it
	// could not return the error, and panicked with it (ir.ReadError).
	defer func() {
		if r := recover(); r != nil {
			re, ok := r.(*ir.ReadError)
			if !ok {
				panic(r)
			}
			known, err = true, fmt.Errorf("internal error: %w", re)
		}
	}()
	c := find(commands, name)
	if c == nil {
		return false, nil
	}
	if len(c.sub) > 0 {
		return true, runGroup(c, args)
	}
	return true, c.run(args)
}

// unknownCommand ends the run of a name that is not in the table.
func unknownCommand(name string) {
	fmt.Fprintf(os.Stderr, "rivus: unknown command %q\n\n%s", name, usageText())
	os.Exit(2)
}
