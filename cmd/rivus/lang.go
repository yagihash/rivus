package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/yagihash/rivus/internal/report"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/irspill"
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

// languages is what -lang says: "auto" (the default) reads the languages the directory holds (Rust after a question, see
// autoRust), or a list separated by commas. It is decided once for a run (and the answer to the question with it): the
// commands call it more than once.
func (o *analyzeOpts) languages() ([]string, error) {
	key := o.dir + "\x00" + o.lang
	if o.langsDone && o.langsKey == key {
		return o.langsOut, o.langsErr
	}
	o.skipped = nil
	o.langsOut, o.langsErr = o.resolveLanguages()
	o.langsDone, o.langsKey = true, key
	return o.langsOut, o.langsErr
}

func (o *analyzeOpts) resolveLanguages() ([]string, error) {
	if o.lang == "" || o.lang == langAuto {
		langs := detectLangs(o.dir)
		// Reading a Rust project builds it: cargo runs its build scripts and its proc macros. A Cargo.toml that happens to
		// be in the directory is not a request for that, so it is asked (autoRust); Go runs `go list`, which runs cgo and
		// may fetch modules, but not the code of the project; Dart reads it with the analyzer, and what they ask the user
		// to prepare (go generate, build_runner) the user does.
		var chosen []string
		hasRust := false
		for _, l := range langs {
			if l == langRust {
				hasRust = true
				continue
			}
			chosen = append(chosen, l)
		}
		if hasRust {
			read, err := o.autoRust(chosen)
			if err != nil {
				return nil, err
			}
			if read {
				chosen = append(chosen, langRust)
			}
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
		if !slices.Contains(languageNames, l) {
			return nil, fmt.Errorf("-lang: unknown language %q (%s)", l, orList(langValues, false))
		}
		out = append(out, l)
	}
	return out, nil
}

// Why a language that the directory holds was not read (summary.unread.skipped_languages[].why).
const (
	whyDeclined     = "declined"      // asked, and the answer was no
	whyNoTerminal   = "no-terminal"   // it cannot be asked (no terminal) and -yes is not given
	whyNotInstalled = "not-installed" // the extractor or the toolchain is not there
	whyNoSandbox    = "no-sandbox"    // there is no sandbox on this system
)

// declinedError is the end of a run that was asked to read a Rust project alone and was told no: nothing was read. main
// gives it its own exit code (exitDeclined), apart from the error.
type declinedError struct{ msg string }

func (e *declinedError) Error() string { return e.msg }

// exitDeclined is the exit code of a run that read nothing because the user declined to read the Rust project (the others
// in use: 0 ok, 1 error, 2 usage or a bad flag, 4 incomplete (exitIncomplete), 130 interrupted).
const exitDeclined = 3

func (o *analyzeOpts) skippedLang(lang string) bool {
	for _, s := range o.skipped {
		if s.Lang == lang {
			return true
		}
	}
	return false
}

func (o *analyzeOpts) skip(lang, why string) {
	for _, s := range o.skipped {
		if s.Lang == lang {
			return
		}
	}
	o.skipped = append(o.skipped, report.SkippedLanguage{Lang: lang, Why: why})
}

// autoRust decides, for -lang auto, whether to read the Rust project of the directory. others are the other languages
// that will be read. It reads it only after a yes (the question, or -yes) and only where the sandbox is: an answer that
// cannot be had is not a yes, and a Rust project that is not read is said in the result (skipped_languages).
func (o *analyzeOpts) autoRust(others []string) (bool, error) {
	hint := strings.Join(append(append([]string(nil), others...), langRust), ",")
	alone := len(others) == 0
	leave := func(why, say string) {
		o.skip(langRust, why)
		fmt.Fprintf(os.Stderr, "rivus: %s also holds a Rust project (Cargo.toml): it is not read, %s\n", o.dir, say)
	}
	sbx := o.sandboxCheck
	if sbx == nil {
		sbx = sandboxUsable
	}
	// with -unsafe-host-exec the project would not run under the sandbox that the question speaks of: auto does not read it
	// (running without the isolation is -lang rust together with -unsafe-host-exec, both said)
	sbxErr := error(nil)
	if o.hostExec() {
		sbxErr = errors.New("-unsafe-host-exec is set: the project would run on this machine, not under the sandbox")
	} else {
		sbxErr = sbx()
	}
	if err := sbxErr; err != nil {
		if alone {
			return false, fmt.Errorf("%s holds a Rust project (Cargo.toml), and reading it builds it: its build scripts and proc macros run. There is no sandbox here (%v), so it is not read unless it is asked for with -lang rust and -unsafe-host-exec (for code you trust)", o.dir, err)
		}
		leave(whyNoSandbox, "because reading it builds it (build scripts and proc macros run) and there is no sandbox on this system; for code you trust, -lang "+hint+" with -unsafe-host-exec reads it without isolation")
		return false, nil
	}
	// (called directly: a method value `o.rustInstalled` is a closure that the dogfood count of unbound captures takes in)
	var instErr error
	if o.installCheck != nil {
		instErr = o.installCheck()
	} else {
		instErr = o.rustInstalled()
	}
	if err := instErr; err != nil {
		if alone {
			return false, fmt.Errorf("%s holds a Rust project (Cargo.toml), and what reads it is not installed: %v", o.dir, err)
		}
		leave(whyNotInstalled, "because what reads it is not installed (`rivus setup rust`, or the Homebrew package, installs it): "+err.Error())
		return false, nil
	}
	if o.yes {
		return true, nil
	}
	term := o.isTerminal
	if term == nil {
		term = stdinIsTerminal
	}
	if !term() {
		if alone {
			return false, fmt.Errorf("%s holds a Rust project (Cargo.toml), and reading it builds it: its build scripts and proc macros run (under the sandbox of macOS). There is no terminal to ask, so say it: -lang rust or -yes", o.dir)
		}
		leave(whyNoTerminal, "because reading it builds it (build scripts and proc macros run) and there is no terminal to ask; -lang "+hint+" or -yes reads it")
		return false, nil
	}
	if o.askRust(others) {
		return true, nil
	}
	o.skip(langRust, whyDeclined)
	if alone {
		return false, &declinedError{fmt.Sprintf("the Rust project in %s was not read (you said no), and it is all there is: nothing was read", o.dir)}
	}
	fmt.Fprintf(os.Stderr, "rivus: the Rust project in %s is not read (you said no); it is in summary.unread.skipped_languages, and the result is about the other languages (%s)\n", o.dir, strings.Join(others, ", "))
	return false, nil
}

// rustInstalled says whether what reads Rust is there: the extractor and the pinned toolchain (files only; nothing is run).
func (o *analyzeOpts) rustInstalled() error {
	if _, err := o.extractorCommand(langRust); err != nil {
		return err
	}
	if _, err := pinnedToolchainDir(); err != nil {
		return err
	}
	return nil
}

// askRust asks on the standard error and reads the answer; Enter is yes. What is neither yes nor no is no.
func (o *analyzeOpts) askRust(others []string) bool {
	fmt.Fprintf(os.Stderr, `rivus: %s holds a Rust project (Cargo.toml).
  Reading it builds it: the build scripts and the proc macros of the project (and of its dependencies) run.
  They run under the sandbox of macOS: no network while it is built, nothing written outside the project, its dependencies and the work directory.
  The dependencies are fetched from crates.io.
  Not to be asked: -lang rust (or -yes). Not to read it: name the languages, as -lang %s.
  The answer makes a different index: with Rust and without are not the same index.
Read the Rust project? [Y/n] `, o.dir, orGo(others))
	in := o.confirmIn
	if in == nil {
		in = os.Stdin
	}
	line, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		if line == "" {
			fmt.Fprintln(os.Stderr) // the end of the input, not Enter: no
			return false
		}
		return true
	}
	return false
}

func orGo(others []string) string {
	if len(others) == 0 {
		return "go"
	}
	return strings.Join(others, ",")
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
		how = "cargo +" + pinnedToolchain + " install rivus (see extractor/rust/README.md)"
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
	if lang == langDart && flagVal == "" && os.Getenv(env) == "" {
		// the extractor that `rivus setup dart` compiled, when it is current (a stale one is said and not used)
		if exe, why := installedDartExe(); exe != "" {
			if o.verbose {
				fmt.Fprintf(os.Stderr, "rivus: the Dart extractor: the compiled executable %s\n", exe)
			}
			return []string{exe}, nil
		} else if why != "" {
			fmt.Fprintf(os.Stderr, "rivus: %s\n", why)
		}
	}
	words, err := splitCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("the %s extractor command (-%s-extractor or %s) is not read: %w", lang, lang, env, err)
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("the %s extractor is named by a blank command (-%s-extractor or %s): name the command, or leave it unset", lang, lang, env)
	}
	if _, err := exec.LookPath(words[0]); err != nil {
		return nil, &extractorMissingError{fmt.Errorf("the %s extractor %q is not found: install it (%s), or name it with -%s-extractor or %s", lang, words[0], how, lang, env)}
	}
	return words, nil
}

// splitCommand splits the value of an extractor command into words (the first is the executable). A value that names an
// existing file as it is (a path with spaces in it, such as one under "Application Support") is one word. Otherwise the
// words are split at spaces, the way a shell does for quotes: 'a b' keeps everything inside, "a b" keeps the spaces and
// reads \" and \\ as the character, and a backslash before a space, a quote or a backslash makes it a part of the word.
// A value without quotes and backslashes is split at white space, as it always was.
func splitCommand(cmd string) ([]string, error) {
	if t := strings.TrimSpace(cmd); strings.ContainsAny(t, " \t") {
		if st, err := os.Stat(t); err == nil && !st.IsDir() {
			return []string{t}, nil
		}
	}
	var words []string
	var cur strings.Builder
	inWord := false
	rs := []rune(cmd)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		case c == '\'':
			j := i + 1
			for j < len(rs) && rs[j] != '\'' {
				j++
			}
			if j == len(rs) {
				return nil, fmt.Errorf("a single quote is not closed")
			}
			cur.WriteString(string(rs[i+1 : j]))
			inWord = true
			i = j
		case c == '"':
			j := i + 1
			for ; j < len(rs) && rs[j] != '"'; j++ {
				if rs[j] == '\\' && j+1 < len(rs) && (rs[j+1] == '"' || rs[j+1] == '\\') {
					j++
				}
				cur.WriteRune(rs[j])
			}
			if j == len(rs) {
				return nil, fmt.Errorf("a double quote is not closed")
			}
			inWord = true
			i = j
		case c == '\\' && i+1 < len(rs) && strings.ContainsRune(" \t'\"\\", rs[i+1]):
			cur.WriteRune(rs[i+1])
			inWord = true
			i++
		default:
			cur.WriteRune(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
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

// runExtractorSpill is runExtractor for the IR on disk (ADR-123): the extractor writes to a pipe, the core decodes a line
// at a time and puts each function into the file, and the whole output is never held in memory. What is returned is the
// program without its functions, and the Source that has them. The extractor that failed is the error that is returned,
// whatever the core had read of its output.
func (o *analyzeOpts) runExtractorSpill(lang string) (*ir.Program, *irspill.DiskSource, error) {
	pr, pw := io.Pipe()
	type read struct {
		prog *ir.Program
		disk *irspill.DiskSource
		err  error
	}
	done := make(chan read, 1)
	go func() {
		prog, disk, err := readSpill(pr)
		io.Copy(io.Discard, pr) // the extractor is not left to block on a pipe that nobody reads
		done <- read{prog, disk, err}
	}()
	_, runErr := o.extractInto(lang, pw)
	pw.Close()
	r := <-done
	if runErr != nil {
		if r.disk != nil {
			r.disk.Release()
		}
		return nil, nil, runErr
	}
	if r.err != nil {
		return nil, nil, fmt.Errorf("the %s extractor: %w", lang, r.err)
	}
	return r.prog, r.disk, nil
}

// extractRaw runs the extractor of a language (Rust: under the sandbox unless -unsafe-host-exec) and returns the stream of the IR as it wrote it.
func (o *analyzeOpts) extractRaw(lang string) ([]byte, error) {
	return o.extractInto(lang, nil)
}

// extractInto is extractRaw; with w, the stream goes to w as the extractor writes it and nothing is returned.
func (o *analyzeOpts) extractInto(lang string, w io.Writer) ([]byte, error) {
	if lang == langRust && !o.hostExec() {
		o.rustIsolation = &report.Isolation{Mode: "sandbox", Detail: "the build scripts and the macros of the project ran under the sandbox of macOS: no network while it was built (the fetch stage has one, and runs no code of the project), no files but the project, its dependencies and a work directory"}
		return o.rawRustIsolated(w)
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
	if lang == langDart {
		args = append(args, o.bodyReplacedArgs(langDart)...)
	}
	if lang == langRust {
		args = append(args, o.bodyReplacedArgs(langRust)...)
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
	var out []byte
	if w != nil {
		cmd.Stdout = w
		err = cmd.Run()
	} else {
		out, err = cmd.Output()
	}
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

// rawRustIsolated reads a Rust project under the sandbox of macOS (ADR-093); on another system nothing is run.
func (o *analyzeOpts) rawRustIsolated(w io.Writer) ([]byte, error) {
	dir, err := filepath.Abs(o.dir)
	if err != nil {
		return nil, err
	}
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	iso := o.isolation()
	iso.verbose = o.verbose
	var args []string
	if o.noDeps {
		args = append(args, "--no-deps")
	}
	if o.tests {
		args = append(args, "--include-tests")
	}
	args = append(args, o.bodyReplacedArgs(langRust)...)
	args = append(args, dir)
	out, err := iso.extractRustSandboxedTo(context.Background(), dir, args, w)
	if o.rustIsolation != nil {
		o.rustIsolation.DeveloperDir = iso.developerDir
	}
	return out, err
}

// bodyReplacedArgs are the flags that tell the extractor of the language which packages it does not read: the models
// stand for them. Go has no flag (the loader of the core is told); a Rust crate is named by the name of the crate (without a
// /... ending, which names the subpackages of a Go package and means nothing here).
func (o *analyzeOpts) bodyReplacedArgs(lang string) []string {
	var args []string
	seen := map[string]bool{}
	for _, d := range o.extractorBodyReplaced {
		if lang == langRust {
			d = strings.TrimSuffix(d, model.Subpackages)
		}
		if !seen[d] {
			seen[d] = true
			args = append(args, "--body-replaced", d)
		}
	}
	return args
}

func (o *analyzeOpts) isolation() *isolationOpts {
	extractor, _ := o.extractorCommand(langRust)
	return &isolationOpts{extractor: extractor, timeout: o.isolationTimeout, noFetch: o.noFetch, env: o.isolationEnv, read: o.isolationRead, maxOutput: o.isolationMaxOut, allowHosts: o.allowFetchHosts}
}

// loadLangs reads several languages into one program. The functions of the
// languages do not meet (their names do not overlap), so the program is the
// concatenation, and the texts all come from the stream.
func (o *analyzeOpts) loadLangs(langs []string, patterns []string, rules *model.Rules) (*loaded, error) {
	merged := &ir.Program{}
	o.extractorBodyReplaced = nil
	if !o.dropAfterRead {
		for _, d := range rules.Declared {
			o.extractorBodyReplaced = append(o.extractorBodyReplaced, d.Package)
		}
	}
	var goProg *loaded
	// A program with Go code and the IR on disk keeps the functions in Sources, not in merged.Funcs: the ones of Go are in
	// the file, the ones of the other languages (read into memory) are a Source of their own.
	var srcs []ir.Source
	spill := o.irSpill
	var otherDisks []*irspill.DiskSource
	counts := map[string]int{}
	testFilesSkipped := 0
	var loadErrs []string
	for _, lang := range langs {
		var p *ir.Program
		var disk *irspill.DiskSource
		if err := o.checkLangInput(lang); err != nil {
			return nil, err
		}
		if lang == langGo {
			l, err := o.loadGo(patterns, rules, o.maxImpl)
			if err != nil {
				return nil, o.withProjectHint(err)
			}
			counts[langGo] = l.gossa.TargetFiles
			testFilesSkipped = l.gossa.TestFilesSkipped
			loadErrs = loadMessages(l)
			if len(langs) > 1 {
				if l.src != nil {
					goir.AddTextsOf(l.texts.(*goir.Texts), l.src)
				} else {
					goir.AddTexts(l.texts.(*goir.Texts), l.ir)
				}
			}
			goProg, p = l, l.ir
		} else {
			var err error
			if spill {
				p, disk, err = o.runExtractorSpill(lang)
			} else {
				p, err = o.runExtractor(lang)
			}
			if err != nil {
				var missing *extractorMissingError
				if (o.lang != "" && o.lang != langAuto) || !errors.As(err, &missing) {
					return nil, err
				}
				// found by its marker file, not asked for, and not installed: a
				// project of one language can hold a file of another (a tool, a
				// site). It is said in the result too: the language was not read.
				fmt.Fprintf(os.Stderr, "rivus: skipping %s: %v\n", lang, err)
				merged.LoadErrors = append(merged.LoadErrors, ir.LoadError{Package: lang, Message: fmt.Sprintf("%s was not analysed: %v", lang, err)})
				o.skip(lang, whyNotInstalled)
				continue
			}
		}
		if lang != langGo {
			o.lockLangs = append(o.lockLangs, lang)
			counts[lang] = countFilesUnder(o.dir, sourceSuffix[lang], 1)
		}
		if spill {
			if lang == langGo {
				srcs = append(srcs, goProg.source())
			} else {
				otherDisks = append(otherDisks, disk)
				srcs = append(srcs, disk)
			}
		} else {
			merged.Funcs = append(merged.Funcs, p.Funcs...)
		}
		merged.Impls = append(merged.Impls, p.Impls...)
		merged.Types = append(merged.Types, p.Types...)
		merged.Globals = append(merged.Globals, p.Globals...)
		merged.Texts = append(merged.Texts, p.Texts...)
		merged.CallTexts = append(merged.CallTexts, p.CallTexts...)
		merged.FuncTexts = append(merged.FuncTexts, p.FuncTexts...)
		merged.LoadErrors = append(merged.LoadErrors, p.LoadErrors...)
		merged.TestsSkipped = merged.TestsSkipped.Add(p.TestsSkipped)
		merged.BodiesNotRead = append(merged.BodiesNotRead, p.BodiesNotRead...)
		merged.FnSkipped = append(merged.FnSkipped, p.FnSkipped...)
		// (a program of several languages is not read as a closed world: what the other language calls is not said)
		merged.CallerMarks = len(langs) == 1 && p.CallerMarks
		merged.Files = append(merged.Files, p.Files...)
	}
	if err := o.emptyError(langs, counts, testFilesSkipped, patterns, loadErrs); err != nil {
		return nil, err
	}
	if len(merged.Funcs) == 0 && len(srcs) == 0 && goProg == nil {
		return nil, fmt.Errorf("nothing to analyse: no extractor produced code (languages: %s)", strings.Join(langs, ", "))
	}
	l := &loaded{ir: merged, texts: ir.NewStreamTexts(merged)}
	if len(srcs) > 0 {
		l.src = ir.ConcatSource(merged, srcs...)
		if goProg != nil {
			l.disk = goProg.disk
		}
		if l.disk == nil && len(otherDisks) > 0 {
			l.disk, otherDisks = otherDisks[0], otherDisks[1:]
		}
		l.moreDisks = otherDisks
	}
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
