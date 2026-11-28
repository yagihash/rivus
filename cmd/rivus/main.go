// Command rivus is a taint analysis query engine: it reports facts about
// where untrusted data can reach, and why it could not tell in some places.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/yagihash/rivus/internal/report"
)

func main() {
	tuneGC()
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText())
		os.Exit(2)
	}
	if err := oldIsolationFlag(os.Args[2:]); err != nil {
		fmt.Fprint(os.Stderr, errorLine(err))
		os.Exit(2)
	}
	name := os.Args[1]
	switch name {
	case "-h", "--help", "help":
		fmt.Print(usageText())
		return
	case "-version", "--version":
		name = "version"
	case "extract":
		fmt.Fprint(os.Stderr, "rivus: extract is not a command any more: it is \"rivus debug extract\" (the same flags, the same output)\n")
		os.Exit(2)
	}
	known, err := runCommand(name, os.Args[2:])
	if !known {
		unknownCommand(os.Args[1])
	}
	if errors.Is(err, flag.ErrHelp) {
		// -h of a command that parses its own flags (ContinueOnError): its usage is printed; asking for it is not an error.
		return
	}
	if err != nil {
		var ue *usageError
		if !errors.As(err, &ue) {
			fmt.Fprint(os.Stderr, errorLine(err)) // (a bad flag was said by the flag package, with the usage). SBX-4: the text of the project goes through sanitizeForTerminal
		}
		os.Exit(exitCode(err))
	}
}

// checkSubcommand looks at the first argument of a command that has only commands under it ("rivus models", "rivus setup"). With no argument it
// prints the usage and exits with 2; with -h it prints the usage and says it is handled (exit code 0); with a name that is not known it exits
// with 2. With a known name it returns false: the caller goes on.
func checkSubcommand(command string, args []string, usage string, known ...string) (handled bool) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch args[0] {
	case "-h", "--help", "help", "-help":
		fmt.Print(usage)
		return true
	}
	for _, k := range known {
		if args[0] == k {
			return false
		}
	}
	fmt.Fprintf(os.Stderr, "rivus: unknown %s command %q\n\n%s", command, args[0], usage)
	os.Exit(2)
	return true
}

// exitIncomplete is the exit code of an analysis that did not finish (summary.incomplete): the whole-run time limit
// (-timeout) was reached, or the summaries did not converge (ADR-120). The output is written in full before it: findings,
// summary and stats are there, and they are a part of what a complete run would give. The other codes in use: 0 ok, 1 error,
// 2 usage or a bad flag, 3 declined (exitDeclined), 130 interrupted. A new stop of the whole run (-max-memory) is another reason of
// the same code (report.IncompleteReasons), not another code.
const exitIncomplete = 4

// exitUsage is the exit code of a usage error or a bad flag.
const exitUsage = 2

// usageError is a flag that the command did not take (not defined, or a bad value). The flag package has said what is wrong and
// printed the usage already, so main does not say it again; the exit code is exitUsage, as for the commands that exit by themselves.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// parseFlags is fs.Parse for a command that parses its own flags (ContinueOnError): a flag that is not taken is a usageError. -h
// (flag.ErrHelp) is not one: it is not an error.
func parseFlags(fs *flag.FlagSet, args []string) error {
	err := fs.Parse(args)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return err
	}
	return &usageError{err}
}

// incompleteError is what a command returns after it has written its output of an analysis that is not complete.
type incompleteError struct{ reasons []string }

func (e *incompleteError) Error() string {
	return "the analysis is incomplete (" + strings.Join(e.reasons, ", ") + "): the result is a part of what a complete run would give; see summary.incomplete_reasons and stats"
}

// incomplete is nil for a report of an analysis that finished, and an incompleteError for one that did not.
func incomplete(rep *report.Report) error {
	if rep == nil || !rep.Summary.Incomplete {
		return nil
	}
	return &incompleteError{reasons: rep.Summary.IncompleteReasons}
}

// exitCode is the exit code of a run that ended with err: exitIncomplete for an analysis that did not finish, exitDeclined for a Rust
// project that was declined and was all there was, and 1 otherwise.
func exitCode(err error) int {
	var ue *usageError
	if errors.As(err, &ue) {
		return exitUsage
	}
	var d *declinedError
	if errors.As(err, &d) {
		return exitDeclined
	}
	var inc *incompleteError
	if errors.As(err, &inc) {
		return exitIncomplete
	}
	return 1
}

// The GC percent (ADR-121, the addition of 2026-11-26). The collector keeps less garbage than its default (100): 50. When the memory
// limit is low, the garbage that 50 leaves is a larger part of what the limit allows, and the soft limit does not work until the
// heap is near it, so the collector is made stronger the lower the limit is: gogcLow at gogcLowLimit and below, gogcDefault
// from gogcHighLimit, linear between. Higher limits (a 24 GB machine and more, -max-memory 6GiB and more) are as before.
const (
	gogcDefault   = 50
	gogcLow       = 40 // the value at gogcLowLimit and below (the one number to change)
	gogcLowLimit  = 4 * gib
	gogcHighLimit = 6 * gib
)

// gogcFor is the GC percent for a memory limit in bytes; a limit of 0 (none) gets gogcDefault.
func gogcFor(limit int64) int {
	switch {
	case limit <= 0 || limit >= gogcHighLimit:
		return gogcDefault
	case limit <= gogcLowLimit:
		return gogcLow
	}
	span := float64(gogcHighLimit - gogcLowLimit)
	return gogcLow + int(float64(gogcDefault-gogcLow)*float64(limit-gogcLowLimit)/span+0.5)
}

// tuneGC makes the collector keep less garbage than its default. The analysis
// holds a few GB of live data on a large program, and the default (let the heap
// double before a collection) doubled the peak; half costs almost no time.
// GOGC set in the environment wins. applyMemoryLimit makes it stronger for a low memory limit (gogcFor).
func tuneGC() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(gogcDefault)
	}
}

// oldIsolationFlags are the flags of the container (ADR-064), which went with the container (ADR-093). They are not kept as other names: a
// flag that is given and ignored would hide that the limit it set is gone; so it is said, and nothing runs.
var oldIsolationFlags = map[string]string{
	"container-timeout":    "-isolation-timeout",
	"container-max-output": "-isolation-max-output",
	"container-env":        "-isolation-env",
	"container-ro-mount":   "-isolation-read PATH (the build may read the path; there is no DST any more)",
	"container-memory":     "(gone: the sandbox has no limit of memory; docs/rust-isolation.md)",
	"container-cpus":       "(gone: the sandbox has no limit of CPUs)",
	"container-pids":       "(gone: the sandbox has no limit of processes)",
	"rust-image":           "(gone: there is no image; the extractor runs under the sandbox of the system: Seatbelt on macOS, Landlock and seccomp on Linux)",
}

// oldIsolationFlag says, for an argument that names a flag of the container, what it is now.
func oldIsolationFlag(args []string) error {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		name, _, _ = strings.Cut(name, "=")
		if now, ok := oldIsolationFlags[name]; ok {
			return fmt.Errorf("-%s is not a flag any more: it is %s", name, now)
		}
	}
	return nil
}
