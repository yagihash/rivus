package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/report"
)

// The version of a release is versionStamp (graphkey.go: `go build -ldflags "-X main.versionStamp=1.2.3"`; it is also
// part of the key of an index). A build from a checkout leaves it empty, and rivus says what the build information of Go
// knows (the module version, or the revision and whether the tree was modified).

// versionString is what `rivus version` prints (one line, so that a script and a formula's test can read it).
func versionString() string {
	v := versionStamp
	extra := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		var rev, modified string
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				modified = s.Value
			}
		}
		if rev != "" {
			if len(rev) > 12 {
				rev = rev[:12]
			}
			extra = " (" + rev
			if modified == "true" {
				extra += ", modified"
			}
			extra += ")"
		}
	}
	if v == "" {
		v = "dev"
	}
	return fmt.Sprintf("rivus %s%s %s/%s %s", strings.TrimPrefix(v, "v"), extra, runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// versionFlags are the flags of `rivus version`: the same ones that name the extractors in the commands that read code.
func versionFlags(fs *flag.FlagSet, o *analyzeOpts) {
	fs.StringVar(&o.dartExtractor, "dart-extractor", "", "command of the Dart extractor (default: $RIVUS_DART_EXTRACTOR, then rivus-dart)")
	fs.StringVar(&o.rustExtractor, "rust-extractor", "", "command of the Rust extractor (default: $RIVUS_RUST_EXTRACTOR, then rivus-rust)")
	fs.BoolVar(&o.unsafeHostExec, "unsafe-host-exec", false, "ask the Rust extractor for its version by running it on this machine, without isolation (only where the sandbox of macOS is not there; also RIVUS_UNSAFE_HOST_EXEC=1)")
	usual(fs, "dart-extractor", "rust-extractor", "unsafe-host-exec")
	describe(fs, "unsafe-host-exec", flagInfo{Not: "takes the isolation off (ADR-064); never a default (docs/rust-isolation.md)"})
	kind(fs, "file", "dart-extractor", "rust-extractor")
}

// runVersionCommand is `rivus version`: the first line is versionString (what a script reads); then the version of the schema of the IR, and the
// source that each extractor was built from, against the one this rivus was built with (summary.extractor says the same for a run).
func runVersionCommand(args []string) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	o := &analyzeOpts{}
	versionFlags(fs, o)
	prepareFlags(fs, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	printVersion(os.Stdout, o)
	return nil
}

// printVersion writes the output of `rivus version`.
func printVersion(w io.Writer, o *analyzeOpts) {
	fmt.Fprintln(w, versionString())
	fmt.Fprintf(w, "ir schema %s\n", ir.SchemaVersion)
	for _, lang := range []string{langRust, langDart} {
		c, ok := o.checkExtractor(lang)
		if !ok {
			fmt.Fprintf(w, "%s extractor: not found (name it with -%s-extractor or RIVUS_%s_EXTRACTOR, or build it: rivus setup %s)\n", lang, lang, strings.ToUpper(lang), lang)
			continue
		}
		fmt.Fprintln(w, versionLine(c))
	}
}

// versionLine is the line of an extractor in `rivus version`.
func versionLine(c report.ExtractorCheck) string {
	head := fmt.Sprintf("%s extractor %s", c.Lang, c.Path)
	switch c.Match {
	case matchYes:
		return fmt.Sprintf("%s  schema %s  source %s  (match)", head, c.Schema, shortHash(c.SourceHash))
	case matchNo:
		return fmt.Sprintf("%s  schema %s  source %s  (MISMATCH: this rivus was built with %s; build the extractor again: %s)", head, c.Schema, shortHash(c.SourceHash), shortHash(c.ExpectedHash), rebuildCommand(c.Lang))
	case matchUnknownBinary:
		return fmt.Sprintf("%s  schema %s  source %s  (UNKNOWN: this rivus was built without the hash of the source of the extractors; build it with `make build`)", head, orDash(c.Schema), shortHash(c.SourceHash))
	case matchUnknownExtractor:
		return fmt.Sprintf("%s  schema %s  (UNKNOWN: %s; build the extractor again: %s)", head, orDash(c.Schema), c.Detail, rebuildCommand(c.Lang))
	case matchUnchecked:
		return fmt.Sprintf("%s  not checked (this system has no isolation to run it in; -unsafe-host-exec runs it here, for an extractor you trust)", head)
	}
	return head
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
