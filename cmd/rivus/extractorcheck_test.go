package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yagihash/rivus/internal/report"
)

// The four answers of the check are kept apart (ADR-126): a different thing is repaired in each.
func TestJudgeExtractorKeepsTheFourStatesApart(t *testing.T) {
	t.Parallel()
	const h1, h2 = "1111111111111111111111111111111111111111111111111111111111111111", "2222222222222222222222222222222222222222222222222222222222222222"
	cases := []struct {
		name     string
		out      string
		askErr   error
		expected string
		match    string
		detail   string // a part of the detail
	}{
		{"match", "rivus-rust rivus.ir/1 source=" + h1 + "\n", nil, h1, matchYes, ""},
		{"different", "rivus-rust rivus.ir/1 source=" + h2 + "\n", nil, h1, matchNo, "another source"},
		{"this rivus has no hash", "rivus-rust rivus.ir/1 source=" + h1 + "\n", nil, "unknown", matchUnknownBinary, "make build"},
		{"this rivus has no hash (empty)", "rivus-rust rivus.ir/1 source=" + h1 + "\n", nil, "", matchUnknownBinary, "make build"},
		{"old extractor: no source=", "rivus_dart rivus.ir/0\n", nil, h1, matchUnknownExtractor, "no source="},
		{"extractor without the hash", "rivus-rust rivus.ir/1 source=unknown\n", nil, h1, matchUnknownExtractor, "without the hash"},
		{"old extractor: does not know the flag", "", errString("it failed (exit status 2): rivus-rust: --rivus-version: No such file or directory"), h1, matchUnknownExtractor, "exit status 2"},
		{"timeout", "", errString("it did not answer in time and was stopped"), h1, matchUnknownExtractor, "did not answer"},
		{"answer of another form", "Dart SDK version: 3.13.6\n", nil, h1, matchUnknownExtractor, "not in the form"},
		{"no isolation", "", errNoIsolation, h1, matchUnchecked, "-unsafe-host-exec"},
		{"both unknown: the repair of rivus first", "rivus_dart rivus.ir/0\n", nil, "unknown", matchUnknownBinary, "does not say either"},
	}
	for _, c := range cases {
		got := judgeExtractor(langRust, "/x/rivus-rust", c.out, c.askErr, c.expected)
		if got.Match != c.match {
			t.Errorf("%s: match = %q, want %q (%+v)", c.name, got.Match, c.match, got)
		}
		if !strings.Contains(got.Detail, c.detail) || (c.detail == "" && got.Detail != "") {
			t.Errorf("%s: detail = %q, want it to hold %q", c.name, got.Detail, c.detail)
		}
		if (got.Match == matchYes || got.Match == matchNo) && (got.SourceHash == "" || got.ExpectedHash == "") {
			t.Errorf("%s: hashes not recorded: %+v", c.name, got)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// summary.extractor has these keys, and the warning says the extractor and how to build it again.
func TestExtractorSummaryShapeAndWarning(t *testing.T) {
	t.Parallel()
	c := judgeExtractor(langDart, "/x/rivus-dart", "rivus_dart rivus.ir/1 source=aaaaaaaaaaaaaaaa\n", nil, "bbbbbbbbbbbbbbbb")
	b, _ := json.Marshal(c)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"lang": "dart", "path": "/x/rivus-dart", "schema": "rivus.ir/1", "source_hash": "aaaaaaaaaaaaaaaa", "expected_hash": "bbbbbbbbbbbbbbbb", "match": "no"}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v (%s)", k, m[k], v, b)
		}
	}
	w := extractorWarning(c)
	if !strings.Contains(w, "/x/rivus-dart") || !strings.Contains(w, "rivus setup dart") || strings.Contains(w, "\n") {
		t.Errorf("warning = %q: want one line with the path and how to build it again", w)
	}
	if extractorWarning(judgeExtractor(langDart, "/x", "rivus_dart rivus.ir/1 source=a\n", nil, "a")) != "" {
		t.Error("a match says nothing")
	}
	if w := extractorWarning(judgeExtractor(langRust, "/x", "", errNoIsolation, "a")); w != "" {
		t.Errorf("an extractor that could not be asked in this run is not warned about (the run stops with its own message): %q", w)
	}
}

// A stored stream (-ir) has no extractor to ask: one entry, no hash.
func TestExtractorSummaryOfAStoredStream(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	irFile := filepath.Join(dir, "g.jsonl")
	if err := os.WriteFile(irFile, rivus(t, "debug", "extract", "./cases/hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := rivus(t, "analyze", "-ir", irFile, "-rules", "rules.yaml")
	var res struct {
		Summary struct {
			Extractor []report.ExtractorCheck `json:"extractor"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatal(err)
	}
	ex := res.Summary.Extractor
	if len(ex) != 1 || ex[0].Match != matchIR || ex[0].SourceHash != "unknown (-ir)" || ex[0].Lang != "" {
		t.Errorf("summary.extractor of -ir = %+v, want one entry: match %q, source_hash \"unknown (-ir)\"", ex, matchIR)
	}
	// a Go-only run has no extractor to say
	out = rivus(t, "analyze", "-rules", "rules.yaml", "./cases/hello")
	if strings.Contains(string(out), `"extractor"`) {
		t.Error("a run of Go only has no summary.extractor")
	}
}

// A script that stands for an extractor: it answers --rivus-version like the new one, unless it is given RIVUS_RUST_DRIVER (which makes the real one act as
// rustc; the answer of the real one does not depend on it, and what rivus passes on must not either: it is not passed).
func fakeExtractor(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

const newExtractorBody = `
if [ "$1" != "--rivus-version" ]; then echo "unexpected argument: $1" >&2; exit 4; fi
if [ -n "${RIVUS_RUST_DRIVER+x}" ] || [ -n "${SECRET_OF_THE_USER+x}" ]; then echo "leaked into the call: the environment is $(env | sort | tr '\n' ' ')" >&2; exit 3; fi
if [ -e ./--rivus-version ]; then echo "not in an empty directory" >&2; exit 5; fi
echo "rivus-rust rivus.ir/1 source=abc123"`

// the old extractor does not know the flag: it takes it for the name of a directory and stops with 2 (as rivus-rust did)
const oldExtractorBody = `echo "rivus-rust: $1: No such file or directory (os error 2)" >&2; exit 2`

func withDriverInEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("RIVUS_RUST_DRIVER", "1")
	t.Setenv("SECRET_OF_THE_USER", "x")
}

// -unsafe-host-exec: the user chose no isolation, so the call is direct, with PATH, HOME and TMPDIR only.
func TestHostCallPassesNoDriverEnvironment(t *testing.T) {
	withDriverInEnvironment(t)
	oldHash := rustSourceHash
	defer func() { rustSourceHash = oldHash }()
	rustSourceHash = "abc123"
	o := &analyzeOpts{unsafeHostExec: true, rustExtractor: fakeExtractor(t, "rivus-rust", newExtractorBody)}
	c, ok := o.checkExtractor(langRust)
	if !ok || c.Match != matchYes || c.SourceHash != "abc123" || c.Schema != "rivus.ir/1" {
		t.Errorf("new extractor, RIVUS_RUST_DRIVER in the environment: %+v (ok %v), want a match", c, ok)
	}
	o = &analyzeOpts{unsafeHostExec: true, rustExtractor: fakeExtractor(t, "rivus-rust", oldExtractorBody)}
	c, ok = o.checkExtractor(langRust)
	if !ok || c.Match != matchUnknownExtractor || !strings.Contains(c.Detail, "exit status 2") || !strings.Contains(c.Detail, "--rivus-version") {
		t.Errorf("old extractor: %+v, want unknown_extractor with the reason (exit 2, and what it said)", c)
	}
}

func TestHostCallTimesOut(t *testing.T) {
	old, oldHash := rustVersionTimeout, rustSourceHash
	defer func() { rustVersionTimeout, rustSourceHash = old, oldHash }()
	rustVersionTimeout, rustSourceHash = 300*time.Millisecond, "abc123"
	// a child that keeps the pipe open is not waited for either
	o := &analyzeOpts{unsafeHostExec: true, rustExtractor: fakeExtractor(t, "rivus-rust", "sleep 30 &\nsleep 30")}
	start := time.Now()
	c, ok := o.checkExtractor(langRust)
	if !ok || c.Match != matchUnknownExtractor || !strings.Contains(c.Detail, "did not answer in time") {
		t.Errorf("an extractor that does not answer: %+v, want unknown_extractor, with the reason in a sentence", c)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("the call took %v", d)
	}
}

// The state of the run is in the summary and the warning is said once: noteExtractor.
func TestNoteExtractorRecordsEachState(t *testing.T) {
	oldHash := rustSourceHash
	defer func() { rustSourceHash = oldHash }()
	script := fakeExtractor(t, "rivus-rust", newExtractorBody)
	for _, c := range []struct{ expected, match string }{{"abc123", matchYes}, {"zzz", matchNo}, {"unknown", matchUnknownBinary}} {
		rustSourceHash = c.expected
		o := &analyzeOpts{unsafeHostExec: true, rustExtractor: script}
		o.noteExtractor(langRust)
		got := o.extractorSummary()
		if len(got) != 1 || got[0].Match != c.match || got[0].Lang != langRust || got[0].ExpectedHash != c.expected {
			t.Errorf("expected %q: summary.extractor = %+v, want match %q", c.expected, got, c.match)
		}
	}
	o := &analyzeOpts{unsafeHostExec: true, rustExtractor: filepath.Join(t.TempDir(), "not-there")}
	o.noteExtractor(langRust)
	if len(o.extractorSummary()) != 0 {
		t.Errorf("an extractor that is not found is not an entry (the run says so when it reads): %+v", o.extractorSummary())
	}
}

// `rivus version` says all of it, and the first line stays the one that a script reads.
func TestVersionLineOfEachState(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		c    report.ExtractorCheck
		want string
	}{
		{judgeExtractor(langRust, "/p", "rivus-rust rivus.ir/1 source=abcdefabcdef123\n", nil, "abcdefabcdef123"), "(match)"},
		{judgeExtractor(langRust, "/p", "rivus-rust rivus.ir/1 source=abcdefabcdef123\n", nil, "999999999999999"), "MISMATCH"},
		{judgeExtractor(langRust, "/p", "rivus-rust rivus.ir/1 source=abcdefabcdef123\n", nil, "unknown"), "make build"},
		{judgeExtractor(langDart, "/p", "rivus_dart rivus.ir/0\n", nil, "999999999999999"), "rivus setup dart"},
		{judgeExtractor(langRust, "/p", "", errNoIsolation, "999999999999999"), "no isolation"},
	} {
		if got := versionLine(c.c); !strings.Contains(got, c.want) || !strings.Contains(got, "/p") || strings.Contains(got, "\n") {
			t.Errorf("%s: %q, want %q in one line", c.c.Match, got, c.want)
		}
	}
}

// The hashes that make build sets are read (by the check and `rivus version`), so the linker keeps them: they are in the binary, and the line of an
// extractor of the same source says match.
func TestBuiltInHashesStayInTheBinary(t *testing.T) {
	t.Parallel()
	const rh, dh = "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111", "bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222"
	bin := filepath.Join(t.TempDir(), "rivus")
	if out, err := exec.Command("go", "build", "-ldflags", "-X main.rustSourceHash="+rh+" -X main.dartSourceHash="+dh, "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	b, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{rh, dh} {
		if !strings.Contains(string(b), h) {
			t.Errorf("the hash %s is not in the binary", h)
		}
	}
	ext := fakeExtractor(t, "rivus-rust", `echo "rivus-rust rivus.ir/1 source=`+rh+`"`)
	cmd := exec.Command(bin, "version", "-unsafe-host-exec", "-rust-extractor", ext)
	out, err := cmd.Output()
	if err != nil || !strings.Contains(string(out), "source "+rh[:12]+"  (match)") {
		t.Errorf("rivus version: %v\n%s", err, out)
	}
}
