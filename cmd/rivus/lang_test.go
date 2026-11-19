package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
