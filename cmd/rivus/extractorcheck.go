package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yagihash/rivus/internal/extractorhash"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/report"
)

// Before an extractor of Rust or Dart reads a project, rivus asks it once which source it was built from (`--rivus-version` for Rust, `--version` for
// Dart: `<name> rivus.ir/N source=<hash>`) and compares the answer with the hash this rivus was built with (rustSourceHash, dartSourceHash). The result
// is recorded in summary.extractor and, when it is not a match, in one line on the standard error. It never stops the run (ADR-126: a schema that
// differs stops, a source that differs is recorded). The four answers are kept apart because the repair differs: "no" and "unknown_extractor" are
// repaired by building the extractor again, "unknown_binary" by building rivus with `make build`.

const (
	matchYes              = "yes"
	matchNo               = "no"
	matchUnknownBinary    = "unknown_binary"
	matchUnknownExtractor = "unknown_extractor"
	matchUnchecked        = "unchecked"
	matchIR               = "n/a(-ir)"
)

// extractorVersionTimeout is how long the extractor has to answer. The Rust one answers from constants in about 20 ms; the Dart one is a
// compiled executable (a `dart run` command is slower to start, so it has more).
// (variables: a test shortens them)
var (
	rustVersionTimeout = 5 * time.Second
	dartVersionTimeout = 20 * time.Second
)

// errNoIsolation says that the extractor of Rust was not run to ask its version because there is no isolation to run it in on this system (or the
// sandbox cannot be made): it is not run on this machine outside the isolation unless -unsafe-host-exec says so.
var errNoIsolation = errors.New("there is no isolation on this system to run the extractor in")

// expectedSourceHash is the hash of the source of the extractor that this rivus was built with.
func expectedSourceHash(lang string) string {
	if lang == langRust {
		return rustSourceHash
	}
	return dartSourceHash
}

// parseExtractorVersion reads `<name> rivus.ir/N [key=value ...]`: the schema and the source hash ("" when the extractor does not say it).
func parseExtractorVersion(out string) (schema, source string, err error) {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 || !strings.HasPrefix(fields[1], "rivus.ir/") {
		line := strings.TrimSpace(out)
		if len(line) > 200 {
			line = line[:200] + "..."
		}
		return "", "", fmt.Errorf("its answer is not in the form `<name> rivus.ir/N source=<hash>`: %q", line)
	}
	schema = fields[1]
	for _, f := range fields[2:] {
		if v, ok := strings.CutPrefix(f, "source="); ok {
			source = v
		}
	}
	return schema, source, nil
}

// judgeExtractor is the comparison: the answer of the extractor (out, or the error of asking it) against the hash of this rivus.
func judgeExtractor(lang, path, out string, askErr error, expected string) report.ExtractorCheck {
	c := report.ExtractorCheck{Lang: lang, Path: path, ExpectedHash: expected, SourceHash: extractorhash.Unknown}
	if expected == "" {
		c.ExpectedHash = extractorhash.Unknown
	}
	if errors.Is(askErr, errNoIsolation) {
		c.Match = matchUnchecked
		c.Detail = "not checked: " + askErr.Error() + " (-unsafe-host-exec runs it on this machine, for an extractor and a project you trust)"
		return c
	}
	var why string
	if askErr != nil {
		why = askErr.Error()
	} else if schema, source, err := parseExtractorVersion(out); err != nil {
		why = err.Error()
	} else {
		c.Schema = schema
		switch source {
		case "":
			why = "its answer has no source= (an extractor older than the hash of the source)"
		case extractorhash.Unknown:
			why = "it was built without the hash of its source"
		default:
			c.SourceHash = source
		}
	}
	switch {
	case c.ExpectedHash == extractorhash.Unknown:
		c.Match = matchUnknownBinary
		c.Detail = "this rivus was built without the hash of the source of the extractors (a plain go build): build it with `make build`"
		if why != "" {
			c.Detail += "; the extractor does not say either (" + why + ")"
		}
	case why != "":
		c.Match = matchUnknownExtractor
		c.Detail = why
	case c.SourceHash == c.ExpectedHash:
		c.Match = matchYes
	default:
		c.Match = matchNo
		c.Detail = "the extractor was built from another source than the one this rivus was built with"
	}
	return c
}

// shortHash is the hash as it is shown to a person (the record has all of it).
func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// checkExtractor asks the extractor of lang which source it was built from. ok is false when there is no extractor to ask (it is not found: the
// run says so when it comes to read).
func (o *analyzeOpts) checkExtractor(lang string) (c report.ExtractorCheck, ok bool) {
	words, err := o.extractorCommand(lang)
	if err != nil || len(words) == 0 {
		return c, false
	}
	path := words[0]
	if p, err := exec.LookPath(path); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			path = abs
		}
	}
	out, askErr := o.askExtractorVersion(lang, words)
	return judgeExtractor(lang, path, out, askErr, expectedSourceHash(lang)), true
}

// askExtractorVersion runs the extractor for its version. Dart: the executable as it is (it is not isolated). Rust: under the isolation that reads
// the project (rustExtractorVersionSandboxed), or directly with -unsafe-host-exec (the user chose no isolation); on a system without isolation and
// without -unsafe-host-exec it is not run (errNoIsolation).
func (o *analyzeOpts) askExtractorVersion(lang string, words []string) (string, error) {
	if lang == langDart {
		ctx, cancel := context.WithTimeout(context.Background(), dartVersionTimeout)
		defer cancel()
		return runExtractorVersion(ctx, append(append([]string(nil), words...), "--version"), nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), rustVersionTimeout)
	defer cancel()
	if o.hostExec() {
		return runExtractorVersion(ctx, []string{words[0], "--rivus-version"}, minimalEnv())
	}
	return o.isolation().rustExtractorVersionSandboxed(ctx)
}

// minimalEnv is the environment of the call that is made without isolation: nothing of the environment of the user but what a program needs to
// start (RIVUS_RUST_DRIVER and the like are not passed on).
func minimalEnv() []string {
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.TempDir()}
}

// runExtractorVersion runs argv with no standard input, from an empty directory (a file named like the flag is not there for the program to find),
// and returns what it printed. env nil: the environment of rivus.
func runExtractorVersion(ctx context.Context, argv []string, env []string) (string, error) {
	work, err := os.MkdirTemp("", "rivus-version-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = work
	if env != nil {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second // (a child that keeps the pipe open does not hold the call)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("it did not answer in time and was stopped (%v)", ctx.Err())
		}
		return "", fmt.Errorf("it failed (%v)%s", err, tailOf(stderr.String()))
	}
	return stdout.String(), nil
}

// tailOf is the end of what a program wrote to its standard error, for a message (": <text>", or "").
func tailOf(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 300 {
		s = "..." + s[len(s)-300:]
	}
	return ": " + sanitizeForTerminal([]byte(s)) // SBX-4
}

// rebuildCommand is how an extractor of the language is built again.
func rebuildCommand(lang string) string {
	if lang == langRust {
		return "rivus setup rust -source <extractor/rust of the same commit>"
	}
	return "rivus setup dart -source <extractor/dart of the same commit>"
}

// extractorWarning is the one line on the standard error that says what the check found ("" for a match, and for what is not an extractor of a
// project that is read).
func extractorWarning(c report.ExtractorCheck) string {
	name := map[string]string{langRust: "Rust", langDart: "Dart"}[c.Lang]
	switch c.Match {
	case matchNo:
		return fmt.Sprintf("rivus: WARNING: the %s extractor %s was built from another source than this rivus (extractor %s, rivus %s): its results may differ from the ones of the extractor of this rivus. Build it again: %s (summary.extractor)", name, c.Path, shortHash(c.SourceHash), shortHash(c.ExpectedHash), rebuildCommand(c.Lang))
	case matchUnknownExtractor:
		return fmt.Sprintf("rivus: WARNING: the %s extractor %s did not say which source it was built from (%s), so it is not known to be the one of this rivus: its results may differ. Build it again: %s (summary.extractor)", name, c.Path, c.Detail, rebuildCommand(c.Lang))
	case matchUnknownBinary:
		return fmt.Sprintf("rivus: WARNING: this rivus was built without the hash of the source of the extractors (a plain go build), so the %s extractor %s cannot be checked against it. Build rivus with `make build` (summary.extractor)", name, c.Path)
	}
	return ""
}

// noteExtractor checks the extractor of lang before it reads the project, records the answer for the summary, and says on the standard error what is
// not a match. A run that could not ask (no isolation) says nothing here: the extraction that follows stops with its own message.
func (o *analyzeOpts) noteExtractor(lang string) {
	c, ok := o.checkExtractor(lang)
	if !ok {
		return
	}
	o.extractorChecks = append(o.extractorChecks, c)
	if w := extractorWarning(c); w != "" {
		fmt.Fprintln(os.Stderr, w)
	}
}

// extractorSummary is summary.extractor: what the extractors that ran said, or, for a stored stream, the one entry that says it has no extractor.
func (o *analyzeOpts) extractorSummary() []report.ExtractorCheck {
	if o.irFile != "" {
		return []report.ExtractorCheck{{Match: matchIR, SourceHash: "unknown (-ir)", Detail: "the IR was read from a file (-ir): the extractor that wrote it is not asked, and the schema of its first line is checked"}}
	}
	return o.extractorChecks
}

// withExtractorPath adds the extractor to the error of a stream that was refused for its schema: which program to build again.
func (o *analyzeOpts) withExtractorPath(lang string, err error) error {
	var se *ir.SchemaError
	if !errors.As(err, &se) {
		return err
	}
	words, werr := o.extractorCommand(lang)
	if werr != nil || len(words) == 0 {
		return err
	}
	return fmt.Errorf("%w [the extractor that wrote it: %s]", err, words[0])
}

// goVersionWarning is the one line on the standard error for a module that is written for a newer Go than this rivus was verified with ("" for
// any other answer). It does not stop the run (ADR-127).
func goVersionWarning(g *report.GoVersion, mainModule string) string {
	if g == nil || g.Match != report.GoNewer {
		return ""
	}
	who := "the module " + g.NewestModule
	if g.NewestModule == mainModule {
		who = "the analysed module " + g.NewestModule
	}
	return fmt.Sprintf("rivus: WARNING: %s says go %s, newer than go %s that this rivus was verified with: the language may have what rivus was not checked against, and the result may be incomplete (summary.go_version)", who, g.NewestVersion, g.Verified)
}
