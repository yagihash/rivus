package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// An analysis that reads no code is an error (exit code 1), not a result of "no findings": an empty result reads as "safe". Each case is a
// form of input that has nothing to read: the exit code, the message, and that no result is written are fixed.

func writeEmptyFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// runIn runs the binary in dir and gives the exit code, the standard output and the standard error.
func runEmptyIn(t *testing.T, dir string, env []string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	return code, so.String(), se.String()
}

func wantEmptyError(t *testing.T, what string, code int, stdout, stderr string, contains ...string) {
	t.Helper()
	if code != 1 {
		t.Errorf("%s: exit code %d, want 1\n%s", what, code, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("%s: a result was written to the standard output: %.200s", what, stdout)
	}
	for _, c := range contains {
		if !strings.Contains(stderr, c) {
			t.Errorf("%s: the message does not say %q:\n%s", what, c, stderr)
		}
	}
}

const goMod = "module example.com/x\n\ngo 1.22\n"

func TestEmptyInputGo(t *testing.T) {
	t.Parallel()
	modOnly := t.TempDir()
	writeEmptyFiles(t, modOnly, map[string]string{"go.mod": goMod})
	testsOnly := t.TempDir()
	writeEmptyFiles(t, testsOnly, map[string]string{"go.mod": goMod, "a_test.go": "package x\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n"})
	typesOnly := t.TempDir()
	writeEmptyFiles(t, typesOnly, map[string]string{"go.mod": goMod, "a.go": "package x\n\n// only a type, no function\ntype T struct{}\n"})

	t.Run("path does not exist", func(t *testing.T) {
		t.Parallel()
		code, so, se := runEmptyIn(t, modOnly, nil, "analyze", "-lang", "go", "-C", filepath.Join(modOnly, "nope"))
		wantEmptyError(t, "-C", code, so, se, "does not exist")
		code, so, se = runEmptyIn(t, modOnly, nil, "analyze", "-lang", "go", "./nope/...")
		wantEmptyError(t, "pattern", code, so, se, "does not exist")
	})
	t.Run("go.mod only", func(t *testing.T) {
		t.Parallel()
		for _, cmd := range []string{"analyze", "sites"} {
			code, so, se := runEmptyIn(t, modOnly, nil, cmd, "-lang", "go", "./...")
			wantEmptyError(t, cmd, code, so, se, "nothing to analyse", "no package matched ./...")
		}
	})
	t.Run("only test files says -tests", func(t *testing.T) {
		t.Parallel()
		code, so, se := runEmptyIn(t, testsOnly, nil, "analyze", "-lang", "go", "./...")
		wantEmptyError(t, "tests only", code, so, se, "-tests")
		if code, _, se := runEmptyIn(t, testsOnly, nil, "analyze", "-lang", "go", "-tests", "./..."); code != 0 {
			t.Errorf("with -tests: exit code %d\n%s", code, se)
		}
	})
	t.Run("a file with no function is a success", func(t *testing.T) {
		t.Parallel()
		if code, so, se := runEmptyIn(t, typesOnly, nil, "analyze", "-lang", "go", "./..."); code != 0 || !strings.Contains(so, `"summary"`) {
			t.Errorf("exit code %d\n%s", code, se)
		}
	})
}

// A directory of another language says what was found, and that rivus reads Go, Dart and Rust: not "go.mod file not found" alone.
func TestEmptyInputOtherLanguage(t *testing.T) {
	t.Parallel()
	ts := t.TempDir()
	writeEmptyFiles(t, ts, map[string]string{"package.json": "{}\n", "tsconfig.json": "{}\n", "src/index.ts": "export {}\n"})
	for _, args := range [][]string{{"analyze"}, {"analyze", "-lang", "go"}, {"sites"}, {"index", "-graph", filepath.Join(t.TempDir(), "g.db")}} {
		code, so, se := runEmptyIn(t, ts, nil, args...)
		wantEmptyError(t, strings.Join(args, " "), code, so, se, "no Go, Dart or Rust project found", "package.json (Node.js / TypeScript)", "tsconfig.json (TypeScript)", "reads only Go, Dart and Rust")
	}
	none := t.TempDir()
	code, so, se := runEmptyIn(t, none, nil, "analyze")
	wantEmptyError(t, "no marker at all", code, so, se, "no Go, Dart or Rust project found")
	if strings.Contains(se, "Found:") {
		t.Errorf("nothing of another language is there, and the message says it found something:\n%s", se)
	}
}

func TestEmptyInputDart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	t.Run("path does not exist", func(t *testing.T) {
		t.Parallel()
		code, so, se := runEmptyIn(t, dir, nil, "analyze", "-lang", "dart", filepath.Join(dir, "nope"))
		wantEmptyError(t, "argument", code, so, se, "does not exist")
		code, so, se = runEmptyIn(t, dir, nil, "analyze", "-lang", "dart", "-C", filepath.Join(dir, "nope"))
		wantEmptyError(t, "-C", code, so, se, "does not exist")
	})
	t.Run("pubspec.yaml only", func(t *testing.T) {
		t.Parallel()
		dart := os.Getenv("RIVUS_DART")
		if dart == "" {
			skipOrFail(t, "RIVUS_DART")
		}
		extractor, err := filepath.Abs("../../extractor/dart")
		if err != nil {
			t.Fatal(err)
		}
		pkg := t.TempDir()
		writeEmptyFiles(t, pkg, map[string]string{"pubspec.yaml": "name: empty\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n"})
		env := []string{"RIVUS_DART_EXTRACTOR=" + dart + " run " + filepath.Join(extractor, "bin", "rivus_dart.dart")}
		code, so, se := runEmptyIn(t, pkg, env, "analyze", "-lang", "dart", ".")
		wantEmptyError(t, "pubspec.yaml only", code, so, se, "no .dart file", "dart pub get")
	})
}

// Rust needs the sandbox to be read; what can be said without it is that there is no Cargo.toml.
func TestEmptyInputRust(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	code, so, se := runEmptyIn(t, dir, nil, "analyze", "-lang", "rust")
	wantEmptyError(t, "no Cargo.toml", code, so, se, "no Cargo.toml")
	code, so, se = runEmptyIn(t, dir, nil, "analyze", "-lang", "rust", "-C", filepath.Join(dir, "nope"))
	wantEmptyError(t, "-C", code, so, se, "does not exist")
}
