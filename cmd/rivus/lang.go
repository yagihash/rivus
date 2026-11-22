package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/yagihash/rivus/internal/report"
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
		// Reading a Rust project builds it: cargo runs its build scripts and its proc macros (in a container, or with
		// the rights of whoever runs rivus under -unsafe-host-exec). A Cargo.toml that happens to be in the directory
		// is not a request for that, so Rust is read only when it is asked for (-lang rust). Go runs `go list`, which
		// runs cgo and may fetch modules, but not the code of the project; Dart reads it with the analyzer, and what
		// they ask the user to prepare (go generate, build_runner) the user does.
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
	if lang == langRust && flagVal == "" && os.Getenv(env) == "" {
		// the extractor that `rivus setup rust` built is found when none is named and none is on PATH
		if _, err := exec.LookPath(name); err != nil {
			if data, derr := rustDataDir(); derr == nil {
				if p := filepath.Join(data, "extractor", "bin", "rivus-rust"); fileExists(p) {
					return []string{p}, nil // (a path with a space in it is one word: it is not split)
				}
			}
		}
	}
	words := strings.Fields(cmd)
	if len(words) == 0 {
		return nil, fmt.Errorf("the %s extractor is named by a blank command (-%s-extractor or %s): name the command, or leave it unset", lang, lang, env)
	}
	if _, err := exec.LookPath(words[0]); err != nil {
		return nil, &extractorMissingError{fmt.Errorf("the %s extractor %q is not found: install it (%s), or name it with -%s-extractor or %s", lang, words[0], how, lang, env)}
	}
	return words, nil
}

// extractorMissingError says that the extractor of a language is not
// installed. That is the only failure of an extractor that a language found by
// its marker file may skip (ADR-044); one that was run and failed is an error.
type extractorMissingError struct{ error }

func (e *extractorMissingError) Unwrap() error { return e.error }

// runExtractor runs the extractor of a language on the directory and reads the
// IR it writes.
func (o *analyzeOpts) runExtractor(lang string) (*ir.Program, error) {
	out, err := o.extractRaw(lang)
	if err != nil {
		return nil, err
	}
	return readExtracted(lang, out)
}

// extractRaw runs the extractor of a language (Rust: in the container unless -unsafe-host-exec) and returns the stream of the IR as it wrote it.
func (o *analyzeOpts) extractRaw(lang string) ([]byte, error) {
	if lang == langRust && !o.hostExec() {
		o.rustIsolation = &report.Isolation{Mode: "sandbox", Detail: "the build scripts and the macros of the project ran under the sandbox of macOS: no network while it was built (the fetch stage has one, and runs no code of the project), no files but the project, its dependencies and a work directory"}
		return o.rawRustIsolated()
	}
	if lang == langRust {
		o.rustIsolation = &report.Isolation{Mode: "host", Detail: "-unsafe-host-exec: the build scripts and the macros of the project ran on this machine, with your rights, without any isolation"}
		if d, err := filepath.Abs(o.dir); err == nil {
			if _, err := os.Stat(filepath.Join(d, hostileMarker)); err == nil {
				return nil, fmt.Errorf("%s is a project that is hostile on purpose (a test of the isolation): it is never run on this machine", d)
			}
		}
	}
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
	if (lang == langRust || lang == langDart) && o.tests {
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
	return out, nil
}

func readExtracted(lang string, out []byte) (*ir.Program, error) {
	prog, err := ir.Read(bufio.NewReaderSize(bytes.NewReader(out), 1<<20))
	if err != nil {
		return nil, fmt.Errorf("the %s extractor: %w", lang, err)
	}
	prog.Files = prog.SourceFiles()
	prog.PruneDeps()
	return prog, nil
}

// hostExec says that the Rust extractor runs on this machine, with the rights of the user, and not in a container.
func (o *analyzeOpts) hostExec() bool {
	return o.unsafeHostExec || os.Getenv("RIVUS_UNSAFE_HOST_EXEC") == "1"
}

// runRustIsolated reads a Rust project in a container (ADR-064).
func (o *analyzeOpts) rawRustIsolated() ([]byte, error) {
	dir, err := filepath.Abs(o.dir)
	if err != nil {
		return nil, err
	}
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	iso := o.isolation()
	var args []string
	if o.noDeps {
		args = append(args, "--no-deps")
	}
	if o.tests {
		args = append(args, "--include-tests")
	}
	args = append(args, dir)
	return iso.extractRustIsolated(context.Background(), dir, args)
}

func (o *analyzeOpts) isolation() *isolationOpts {
	image := o.rustImage
	if image == "" {
		image = os.Getenv("RIVUS_RUST_IMAGE")
	}
	if image == "" {
		image = rustImageName
	}
	extractor, _ := o.extractorCommand(langRust)
	return &isolationOpts{extractor: extractor, image: image, memory: o.containerMemory, cpus: o.containerCPUs, pids: o.containerPids, timeout: o.containerTimeout, noFetch: o.noFetch, env: o.containerEnv, mounts: o.containerMounts, maxOutput: o.containerMaxOut, allowHosts: o.allowFetchHosts}
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
				var missing *extractorMissingError
				if (o.lang != "" && o.lang != "auto") || !errors.As(err, &missing) {
					return nil, err
				}
				// found by its marker file, not asked for, and not installed: a
				// project of one language can hold a file of another (a tool, a
				// site). It is said in the result too: the language was not read.
				fmt.Fprintf(os.Stderr, "rivus: skipping %s: %v\n", lang, err)
				merged.LoadErrors = append(merged.LoadErrors, ir.LoadError{Package: lang, Message: fmt.Sprintf("%s was not analysed: %v", lang, err)})
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
		merged.TestsSkipped = merged.TestsSkipped.Add(p.TestsSkipped)
		// (a program of several languages is not read as a closed world: what the other language calls is not said)
		merged.CallerMarks = len(langs) == 1 && p.CallerMarks
		merged.Files = append(merged.Files, p.Files...)
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

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
