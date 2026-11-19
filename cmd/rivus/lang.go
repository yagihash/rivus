package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
)

// The languages rivus reads. Go is read in this process; the others by an
// extractor, a program of its own that writes the neutral IR.
const (
	langGo   = "go"
	langDart = "dart"
	langRust = "rust"
)

// markers are the files that tell a directory holds a language.
var markers = []struct{ file, lang string }{
	{"go.mod", langGo}, {"go.work", langGo},
	{"pubspec.yaml", langDart},
	{"Cargo.toml", langRust},
}

// detectLangs lists the languages of the project in dir, by its marker files.
func detectLangs(dir string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range markers {
		if seen[m.lang] {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			seen[m.lang] = true
			out = append(out, m.lang)
		}
	}
	return out
}

// languages is what -lang says: "auto" (the default) reads every language the
// directory holds, or a list separated by commas.
func (o *analyzeOpts) languages() ([]string, error) {
	if o.lang == "" || o.lang == "auto" {
		langs := detectLangs(o.dir)
		// Reading a Rust project builds it: cargo runs its build scripts and its proc macros, with the rights of
		// whoever runs rivus. A Cargo.toml that happens to be in the directory is not a request for that, so Rust
		// is read only when it is asked for (-lang rust). Go does not run the code, and Dart reads it with the
		// analyzer; what they ask the user to prepare (go generate, build_runner) the user does.
		var chosen []string
		skippedRust := false
		for _, l := range langs {
			if l == langRust {
				skippedRust = true
				continue
			}
			chosen = append(chosen, l)
		}
		if skippedRust {
			if len(chosen) == 0 {
				return nil, fmt.Errorf("%s holds a Rust project (Cargo.toml), and reading it builds it: its build scripts and proc macros run, with your rights. "+
					"Ask for it with -lang rust (for code you trust, or in a sandbox)", o.dir)
			}
			fmt.Fprintf(os.Stderr, "rivus: %s also holds a Rust project (Cargo.toml): it is not read, because reading it builds it (build scripts and proc macros run); ask for it with -lang %s,rust\n",
				o.dir, strings.Join(chosen, ","))
		}
		if len(chosen) == 0 {
			// nothing to tell by: Go, which also reads a pattern outside a module
			return []string{langGo}, nil
		}
		return chosen, nil
	}
	var out []string
	for _, l := range strings.Split(o.lang, ",") {
		l = strings.TrimSpace(l)
		switch l {
		case langGo, langDart, langRust:
			out = append(out, l)
		default:
			return nil, fmt.Errorf("-lang: unknown language %q (go, dart, rust, auto)", l)
		}
	}
	return out, nil
}

// extractorCommand is the command of the extractor of a language: the flag,
// then the environment, then the name the install gives it.
func (o *analyzeOpts) extractorCommand(lang string) ([]string, error) {
	var flagVal, env, name, how string
	switch lang {
	case langDart:
		flagVal, env, name = o.dartExtractor, "RIVUS_DART_EXTRACTOR", "rivus-dart"
		how = "dart pub global activate --source path extractor/dart"
	case langRust:
		flagVal, env, name = o.rustExtractor, "RIVUS_RUST_EXTRACTOR", "rivus-rust"
		how = "cargo +nightly-2026-11-15 install rivus (see extractor/rust/README.md)"
	}
	cmd := flagVal
	if cmd == "" {
		cmd = os.Getenv(env)
	}
	if cmd == "" {
		cmd = name
	}
	words := strings.Fields(cmd)
	if _, err := exec.LookPath(words[0]); err != nil {
		return nil, fmt.Errorf("the %s extractor %q is not found: install it (%s), or name it with -%s-extractor or %s", lang, words[0], how, lang, env)
	}
	return words, nil
}

// runExtractor runs the extractor of a language on the directory and reads the
// IR it writes.
func (o *analyzeOpts) runExtractor(lang string) (*ir.Program, error) {
	words, err := o.extractorCommand(lang)
	if err != nil {
		return nil, err
	}
	args := append([]string(nil), words[1:]...)
	if o.noDeps {
		args = append(args, "--no-deps")
	}
	if lang == langDart {
		for _, d := range o.allowDepsUnder {
			args = append(args, "--allow-deps-under", d)
		}
	}
	if lang == langRust && o.tests {
		args = append(args, "--include-tests")
	}
	dir, err := filepath.Abs(o.dir)
	if err != nil {
		return nil, err
	}
	args = append(args, dir)
	cmd := exec.Command(words[0], args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	fmt.Fprintf(os.Stderr, "rivus: extracting %s: %s\n", lang, strings.Join(append([]string{words[0]}, args...), " "))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("the %s extractor failed: %v\n%s", lang, err, strings.TrimSpace(stderr.String()))
	}
	prog, err := ir.Read(bufio.NewReaderSize(bytes.NewReader(out), 1<<20))
	if err != nil {
		return nil, fmt.Errorf("the %s extractor: %w", lang, err)
	}
	prog.PruneDeps()
	return prog, nil
}

// loadLangs reads several languages into one program. The functions of the
// languages do not meet (their names do not overlap), so the program is the
// concatenation, and the texts all come from the stream.
func (o *analyzeOpts) loadLangs(langs []string, patterns []string, rules *model.Rules) (*loaded, error) {
	merged := &ir.Program{}
	var goProg *loaded
	for _, lang := range langs {
		var p *ir.Program
		if lang == langGo {
			l, err := o.loadGo(patterns, rules, o.maxImpl)
			if err != nil {
				return nil, err
			}
			if len(langs) > 1 {
				goir.AddTexts(l.texts.(*goir.Texts), l.ir)
			}
			goProg, p = l, l.ir
		} else {
			var err error
			if p, err = o.runExtractor(lang); err != nil {
				if o.lang != "" && o.lang != "auto" {
					return nil, err
				}
				// found by its marker file, not asked for: a project of one
				// language can hold a file of another (a tool, a site)
				fmt.Fprintf(os.Stderr, "rivus: skipping %s: %v\n", lang, err)
				continue
			}
		}
		merged.Funcs = append(merged.Funcs, p.Funcs...)
		merged.Impls = append(merged.Impls, p.Impls...)
		merged.Types = append(merged.Types, p.Types...)
		merged.Globals = append(merged.Globals, p.Globals...)
		merged.Texts = append(merged.Texts, p.Texts...)
		merged.CallTexts = append(merged.CallTexts, p.CallTexts...)
		merged.FuncTexts = append(merged.FuncTexts, p.FuncTexts...)
		merged.LoadErrors = append(merged.LoadErrors, p.LoadErrors...)
	}
	if len(merged.Funcs) == 0 && goProg == nil {
		return nil, fmt.Errorf("nothing to analyse: no extractor produced code (languages: %s)", strings.Join(langs, ", "))
	}
	l := &loaded{ir: merged, texts: ir.NewStreamTexts(merged)}
	if goProg != nil && len(langs) == 1 {
		return goProg, nil
	}
	if goProg != nil {
		l.gossa = goProg.gossa
	}
	return l, nil
}
