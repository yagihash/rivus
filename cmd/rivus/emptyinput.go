package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Reading nothing is an error, not an empty result: an analysis of no code says "no findings", and that reads as "safe". The checks
// here run for every command that reads a directory (analyze, sites, index, query, resolve, models coverage: they all come through
// analyzeOpts.read), once, for all the languages. The exit code is 1, as any error of the input has (ADR-120).

// otherProjects are the marker files of the languages rivus does not read. They are only used to say what was found in a directory
// that holds no Go, Dart or Rust project; nothing guesses a language from them, and nothing reads such a project.
var otherProjects = []struct{ file, what string }{
	{"package.json", "Node.js / TypeScript"},
	{"tsconfig.json", "TypeScript"},
	{"pom.xml", "Java"},
	{"build.gradle", "Java / Kotlin"},
	{"build.gradle.kts", "Java / Kotlin"},
	{"pyproject.toml", "Python"},
	{"requirements.txt", "Python"},
	{"composer.json", "PHP"},
	{"Gemfile", "Ruby"},
}

// sourceSuffix is the name of a source file of the languages that an extractor reads. The files are counted on disk, not in the IR: a
// file of types and comments leaves no function and may leave no position in it, and it is a file that was read.
var sourceSuffix = map[string]string{langDart: ".dart", langRust: ".rs"}

// supported is what rivus reads, for the message.
const supported = "Go, Dart or Rust"

// noProjectHint is the sentence that is added to an error of reading Go when the directory holds no marker of a language that rivus
// reads and either holds the marker of another language or holds none at all. It is empty when the directory holds a go.mod or a
// go.work (the error is about something else), or when it holds the marker of Dart or Rust (the project is there).
func noProjectHint(dir string) string {
	if len(detectLangs(dir)) > 0 {
		return ""
	}
	var found []string
	for _, m := range otherProjects {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			found = append(found, fmt.Sprintf("%s (%s)", m.file, m.what))
		}
	}
	msg := fmt.Sprintf("no %s project found in %s (no go.mod, go.work, pubspec.yaml or Cargo.toml)", supported, dir)
	if len(found) > 0 {
		msg += fmt.Sprintf(". Found: %s. rivus reads only %s", strings.Join(found, ", "), strings.ReplaceAll(supported, " or ", " and "))
	}
	return msg
}

// withProjectHint adds noProjectHint to an error of reading Go.
func (o *analyzeOpts) withProjectHint(err error) error {
	if h := noProjectHint(o.dir); h != "" {
		return fmt.Errorf("%w; %s", err, h)
	}
	return err
}

// checkInputPaths says that a directory (-C) or a path argument that does not exist is not there, before any language reads it. A
// path argument of Go is a package pattern; it is checked only when it is written as a path (./x, ../x, /x, with or without /...).
// The Dart and Rust extractors take no patterns: with only them, an argument is a path and has to exist (under -C).
func (o *analyzeOpts) checkInputPaths(langs []string, patterns []string) error {
	fi, err := os.Stat(o.dir)
	if err != nil {
		return fmt.Errorf("%s: that path does not exist, so there is nothing to analyse", o.dir)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s: that is not a directory (-C takes the directory of the project)", o.dir)
	}
	for _, p := range patterns {
		pathLike := p == "." || p == ".." || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || filepath.IsAbs(p)
		if !pathLike && containsGo(langs) {
			continue
		}
		q := strings.TrimSuffix(strings.TrimSuffix(p, "..."), "/")
		if q == "" {
			q = "."
		}
		if !filepath.IsAbs(q) {
			q = filepath.Join(o.dir, q)
		}
		if _, err := os.Stat(q); err != nil {
			return fmt.Errorf("%s: that path does not exist (the argument is read from %s), so there is nothing to analyse", p, o.dir)
		}
	}
	return nil
}

func containsGo(langs []string) bool {
	for _, l := range langs {
		if l == langGo {
			return true
		}
	}
	return false
}

// checkLangInput is the check that needs no reading: the marker a language needs is there.
func (o *analyzeOpts) checkLangInput(lang string) error {
	if lang == langRust {
		if _, err := os.Stat(filepath.Join(o.dir, "Cargo.toml")); err != nil {
			return fmt.Errorf("no Cargo.toml in %s: a Rust project is a directory with a Cargo.toml (-lang rust looked for one and found none)", o.dir)
		}
	}
	return nil
}

// emptyError is the error of a read that found no code to analyse in any language: counts is the number of source files of each
// language that was read (a file of a type or a comment counts: a program of no function but of a file is read). Go: with
// tests holds the number of _test.go files that were not read.
func (o *analyzeOpts) emptyError(langs []string, counts map[string]int, testFilesSkipped int, patterns []string, loadErrs []string) error {
	var parts []string
	for _, lang := range langs {
		if n, ok := counts[lang]; ok && n > 0 {
			return nil
		}
	}
	if h := noProjectHint(o.dir); h != "" {
		return fmt.Errorf("nothing to analyse: %s", h)
	}
	for _, lang := range langs {
		if _, ok := counts[lang]; !ok {
			continue
		}
		switch lang {
		case langGo:
			pats := strings.Join(patterns, " ")
			if pats == "" {
				pats = "./..."
			}
			s := fmt.Sprintf("Go: no package matched %s in %s, or the packages hold no Go file", pats, o.dir)
			if testFilesSkipped == 0 && hasFileUnder(o.dir, "_test.go") {
				testFilesSkipped = 1
			}
			if testFilesSkipped > 0 {
				s = fmt.Sprintf("Go: %s holds only test files (_test.go): they are read with -tests", o.dir)
			}
			if len(loadErrs) > 0 {
				s += " (" + loadErrs[0] + ")"
			}
			parts = append(parts, s)
		case langDart:
			s := fmt.Sprintf("Dart: no .dart file found in %s", o.dir)
			if _, err := os.Stat(filepath.Join(o.dir, ".dart_tool", "package_config.json")); err != nil {
				s += "; there is no .dart_tool/package_config.json either: run `dart pub get` in the project first"
			}
			parts = append(parts, s)
		case langRust:
			parts = append(parts, fmt.Sprintf("Rust: no Rust source file found in %s (the crates of its Cargo.toml hold none)", o.dir))
		}
	}
	return fmt.Errorf("nothing to analyse: %s. No result is written, because an analysis of no code is not a result of \"no findings\"", strings.Join(parts, "; "))
}

// loadMessages are the first messages of what the Go load could not read.
func loadMessages(l *loaded) []string {
	var out []string
	for _, e := range l.loadErrors() {
		out = append(out, e.Message)
		if len(out) == 1 {
			break
		}
	}
	return out
}

// hasFileUnder says whether a file whose name ends with suffix is under dir (hidden directories, vendor and node_modules are not
// looked into).
func hasFileUnder(dir, suffix string) bool {
	return countFilesUnder(dir, suffix, 1) > 0
}

// countFilesUnder counts the files under dir whose name ends with suffix, up to limit.
func countFilesUnder(dir, suffix string, limit int) int {
	n := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), suffix) {
			n++
			if n >= limit {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return n
}
