package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/testutil"
)

func TestDetectLangs(t *testing.T) {
	dir := t.TempDir()
	if got := detectLangs(dir); len(got) != 0 {
		t.Errorf("empty directory: %v", got)
	}
	for _, f := range []string{"go.mod", "Cargo.toml", "pubspec.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := detectLangs(dir), []string{"go", "rust", "dart"}; !reflect.DeepEqual(got, want) && !reflect.DeepEqual(got, []string{"go", "dart", "rust"}) {
		t.Errorf("detectLangs = %v, want go, dart and rust", got)
	}
}

func TestLanguagesFlag(t *testing.T) {
	o := analyzeOpts{}
	o.dir = t.TempDir()
	o.lang = "dart, rust"
	got, err := o.languages()
	if err != nil || !reflect.DeepEqual(got, []string{"dart", "rust"}) {
		t.Errorf("languages = %v, %v", got, err)
	}
	o.lang = "cobol"
	if _, err := o.languages(); err == nil {
		t.Error("an unknown language is accepted")
	}
	o.lang = "auto"
	if got, _ := o.languages(); !reflect.DeepEqual(got, []string{"go"}) {
		t.Errorf("nothing to tell by: %v", got)
	}
}

// A Cargo.toml in the directory is not a request to build the project (yagihash/rivus #57): Rust is read when it is asked for.
func TestRustIsNotReadUnlessAsked(t *testing.T) {
	write := func(dir string, files ...string) {
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Go and Rust: Go is read, and Rust is left out (and said)
	o := analyzeOpts{}
	o.isTerminal = func() bool { return false } // not a question: the test is run from a terminal too
	o.sandboxCheck, o.installCheck = func() error { return nil }, func() error { return nil }
	o.dir = t.TempDir()
	write(o.dir, "go.mod", "Cargo.toml")
	o.lang = "auto"
	got, err := o.languages()
	if err != nil || !reflect.DeepEqual(got, []string{"go"}) {
		t.Errorf("go and rust, auto: %v, %v", got, err)
	}
	// Rust alone: not an answer of "nothing found" but a request to ask for it
	o.dir = t.TempDir()
	write(o.dir, "Cargo.toml")
	if got, err := o.languages(); err == nil {
		t.Errorf("a Rust project alone is read without being asked: %v", got)
	}
	// asked for: it is read
	o.lang = "rust"
	if got, err := o.languages(); err != nil || !reflect.DeepEqual(got, []string{"rust"}) {
		t.Errorf("-lang rust: %v, %v", got, err)
	}
	o.lang = "go,rust"
	if got, err := o.languages(); err != nil || !reflect.DeepEqual(got, []string{"go", "rust"}) {
		t.Errorf("-lang go,rust: %v, %v", got, err)
	}
}

func TestMissingExtractorIsAnErrorWhenAsked(t *testing.T) {
	o := analyzeOpts{}
	o.dir = t.TempDir()
	o.lang = "rust"
	o.rustExtractor = "/nonexistent/rivus-rust"
	if _, err := o.runExtractor("rust"); err == nil {
		t.Error("a missing extractor is not an error")
	}
}

// A rule file that the analysed directory brings is not read through a link, and the message of an error in it
// does not show what it holds (yagihash/rivus #57).
func TestProjectRuleFileThatIsALinkIsNotRead(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, ".rivus", "rules")
	if err := os.MkdirAll(rules, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("root:x:0:0:root:/root:/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(rules, "z.yaml")); err != nil {
		t.Skip("no symbolic links here")
	}
	c := common{dir: dir, noBuiltin: true}
	_, err := c.loadRules()
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("a link is read as a rule file: %v", err)
	}
	if strings.Contains(err.Error(), "root:x") {
		t.Errorf("the message shows what the target holds: %v", err)
	}

	// a file that is a file, with a mistake in it: the error does not quote the value
	if err := os.Remove(filepath.Join(rules, "z.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rules, "z.yaml"), []byte("root:x:0:0:root:/root:/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = c.loadRules()
	if err == nil {
		t.Fatal("a file that is not a rule file is read")
	}
	if strings.Contains(err.Error(), "root:x") {
		t.Errorf("the message of the error quotes the value: %v", err)
	}
}

// A language found by its marker file may be skipped when its extractor is not
// installed, and the result says so; an extractor that ran and failed is an
// error, not a silent skip (#9).
func TestAutoDetectedExtractorFailure(t *testing.T) {
	o := analyzeOpts{}
	o.dir = testutil.Testdata()
	o.lang = "auto"
	rules, err := o.loadRules()
	if err != nil {
		t.Fatal(err)
	}

	o.dartExtractor = "/nonexistent/rivus-dart"
	l, err := o.loadLangs([]string{langGo, langDart}, []string{"./cases/hello"}, rules)
	if err != nil {
		t.Fatalf("a missing extractor is skipped: %v", err)
	}
	found := false
	for _, le := range l.loadErrors() {
		if le.Package == langDart && strings.Contains(le.Message, "not analysed") {
			found = true
		}
	}
	if !found {
		t.Errorf("the skipped language is not in load_errors: %+v", l.loadErrors())
	}

	o.dartExtractor = "false"
	if _, err := o.loadLangs([]string{langGo, langDart}, []string{"./cases/hello"}, rules); err == nil {
		t.Error("an extractor that failed must not be skipped")
	}
}

// The load errors of the Go packages are counted once when Go is read together
// with another language (#13).
func TestMultiLanguageLoadErrorsAreNotDoubled(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":  "module example.com/brk\n\ngo 1.21\n",
		"main.go": "package main\n\nimport _ \"example.invalid/nope\"\n\nfunc main() {}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := ir.Write(&buf, &ir.Program{}); err != nil {
		t.Fatal(err)
	}
	stream := filepath.Join(t.TempDir(), "stream.jsonl")
	if err := os.WriteFile(stream, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "extractor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ncat "+stream+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	o := analyzeOpts{}
	o.dir = dir
	o.lang = "go"
	rules, err := o.loadRules()
	if err != nil {
		t.Fatal(err)
	}
	l, err := o.loadLangs([]string{langGo}, []string{"./..."}, rules)
	if err != nil {
		t.Fatal(err)
	}
	single := len(l.loadErrors())
	if single == 0 {
		t.Fatal("the broken import is not reported: the test says nothing")
	}

	o.rustExtractor = script
	o.unsafeHostExec = true // a script of the test, not a project that is read
	l, err = o.loadLangs([]string{langGo, langRust}, []string{"./..."}, rules)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(l.loadErrors()); got != single {
		t.Errorf("%d load errors with Go and Rust, %d with Go alone: %+v", got, single, l.loadErrors())
	}
}

// The project rules are found whatever the directory is called: a name with
// the characters of a glob (p[1], q[) is not a pattern (#14).
func TestProjectRulesInADirectoryNamedLikeAGlob(t *testing.T) {
	for _, name := range []string{"p[1]", "q[", "a*b", "p?"} {
		dir := filepath.Join(t.TempDir(), name)
		rules := filepath.Join(dir, ".rivus", "rules")
		if err := os.MkdirAll(rules, 0o755); err != nil {
			t.Fatal(err)
		}
		// a file that is not valid rules: it is an error only if it is read
		if err := os.WriteFile(filepath.Join(rules, "broken.yaml"), []byte("rules: [: :\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		c := common{dir: dir}
		if _, err := c.loadRules(); err == nil {
			t.Errorf("%q: the project rules are not read", name)
		}
	}
}

// The directory of the rules that the analysed code brings is not read through a link either, and an error
// in a rule file does not show the keys of the file (yagihash/rivus #69).
func TestProjectRulesDirectoryThatIsALinkIsNotRead(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "a.yaml"), []byte("SECRETKEY_abc123: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".rivus"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, ".rivus", "rules")); err != nil {
		t.Skip("no symbolic links here")
	}
	c := common{dir: dir, noBuiltin: true}
	_, err := c.loadRules()
	if err == nil || !strings.Contains(err.Error(), "outside the analysed directory") {
		t.Fatalf("a rules directory that is a link to another place is read: %v", err)
	}
	if strings.Contains(err.Error(), "SECRETKEY") {
		t.Errorf("the message shows the file: %v", err)
	}

	// a link inside the directory is fine; a mistake in a file shows its line, not its keys
	_ = os.Remove(filepath.Join(dir, ".rivus", "rules"))
	rules := filepath.Join(dir, ".rivus", "rules")
	if err := os.MkdirAll(rules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rules, "a.yaml"), []byte("SECRETKEY_abc123: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = c.loadRules()
	if err == nil || strings.Contains(err.Error(), "SECRETKEY") || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("the error of a rule file: %v", err)
	}
}
