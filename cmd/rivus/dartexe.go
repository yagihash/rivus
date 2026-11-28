package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yagihash/rivus/internal/extractorhash"
)

// The Dart extractor as a compiled executable (`dart compile exe`). `dart run` from source spends 5 s or more of every run on
// starting (the JIT of the analyzer); the executable starts in a fraction of a second, and writes the same IR (extractor/dart/README.md).
// It is built on purpose by `rivus setup dart`, or on the spot by the tests (dartExeFor), and only from the source of rivus: the commands that read a
// project never build anything for themselves. Every input that can change what it does is in its key (dartExeKey), so a
// stale executable is never taken for a current one.

// dartSDKVersion is the version of the Dart SDK of the `dart` that `dartCmd` runs (the first word: found on PATH), read from the
// `version` file of the SDK.
func dartSDKVersion(dartCmd string) (string, error) {
	p, err := exec.LookPath(dartCmd)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(p)), "version"))
	if err != nil {
		return "", fmt.Errorf("the Dart SDK of %s has no version file: %v", p, err)
	}
	return strings.TrimSpace(string(b)), nil
}

// dartExeKey is the name of the directory of an executable: the source, the SDK that compiled it and the system.
func dartExeKey(src, dartCmd string) (string, error) {
	h, err := extractorhash.Compute("dart", src)
	if err != nil {
		return "", err
	}
	v, err := dartSDKVersion(dartCmd)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s-%s", h[:16], v, runtime.GOOS, runtime.GOARCH), nil
}

// compileDartExe compiles bin/rivus_dart.dart of src into dir/rivus-dart. `dart pub get` has been run in src (the package config is there).
func compileDartExe(src, dartCmd, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	hash, err := extractorhash.Compute("dart", src)
	if err != nil {
		return "", err
	}
	exe := filepath.Join(dir, "rivus-dart")
	// a name of its own for each compile: two compiles into the same directory (two processes, two tests) must not write one file, nor
	// remove what the other is writing
	f, err := os.CreateTemp(dir, "rivus-dart.tmp-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	f.Close()
	// (CreateTemp makes it 0600: dart writes into it, and it is the program in the end)
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return "", err
	}
	words := strings.Fields(dartCmd)
	// the hash of the source is built in: rivus_dart --version and the first line of the IR say it (ADR-126)
	cmd := exec.Command(words[0], append(append([]string{}, words[1:]...), "compile", "exe", "-DRIVUS_SRC_HASH="+hash, filepath.Join("bin", "rivus_dart.dart"), "-o", tmp)...)
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("dart compile exe: %v\n%s", err, out)
	}
	return exe, os.Rename(tmp, exe) // (renamed: another process never sees half of it)
}

// dartExeFor compiles the extractor of src into base/<key>/ unless that is already there, and returns the executable.
func dartExeFor(src, dartCmd, base string) (string, string, error) {
	key, err := dartExeKey(src, dartCmd)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(base, key)
	if exe := filepath.Join(dir, "rivus-dart"); fileExists(exe) {
		return exe, key, nil
	}
	exe, err := compileDartExe(src, dartCmd, dir)
	return exe, key, err
}

// dartInstall is what `rivus setup dart` writes next to the executables: where the source was, and the key it had then.
type dartInstall struct {
	Source string `json:"source"`
	Key    string `json:"key"`
	Exe    string `json:"exe"`
}

func dartDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "rivus", "dart"), nil
}

// installedDartExe is the executable that `rivus setup dart` built, when it is still the one of the source it was built from and of
// the Dart SDK that is there now. Otherwise it says why not (the extractor is then the command that was named, or rivus-dart on
// PATH, as before): a stale executable is not used.
func installedDartExe() (exe, why string) {
	data, err := dartDataDir()
	if err != nil {
		return "", ""
	}
	b, err := os.ReadFile(filepath.Join(data, "installed.json"))
	if err != nil {
		return "", "" // not set up: nothing to say
	}
	var in dartInstall
	if json.Unmarshal(b, &in) != nil {
		return "", "installed.json is not readable"
	}
	key, err := dartExeKey(in.Source, "dart")
	if err != nil {
		return "", fmt.Sprintf("the compiled extractor cannot be checked (%v)", err)
	}
	if key != in.Key || !fileExists(in.Exe) {
		return "", "the compiled extractor is out of date (the source of the extractor, the Dart SDK or the system changed): run rivus setup dart again"
	}
	return in.Exe, ""
}

// setupDartCmd is "rivus setup dart".
type setupDartCmd struct{ source *string }

func setupDartFlags(fs *flag.FlagSet) *setupDartCmd {
	c := &setupDartCmd{}
	c.source = fs.String("source", "", "the directory of the source of the Dart extractor (extractor/dart of the repository of rivus). Required: it is compiled here and run on this machine, so it is never taken from the directory you are in (that may be a project that you are reading)")
	usual(fs, "source")
	kind(fs, "dir", "source")
	return c
}

func runSetupDart(argv []string) error {
	fs := flag.NewFlagSet("setup dart", flag.ContinueOnError)
	c := setupDartFlags(fs)
	prepareFlags(fs, argv)
	if err := parseFlags(fs, argv); err != nil {
		return err
	}
	if *c.source == "" {
		return fmt.Errorf("-source is required: the directory of extractor/dart of the repository of rivus. It is compiled and run here, so it is not taken from the current directory (which may hold a project that you are reading)")
	}
	src, err := filepath.Abs(*c.source)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(src, "pubspec.lock")); err != nil {
		return fmt.Errorf("the source of the Dart extractor is not at %s: name it with -source (the extractor/dart directory of the repository of rivus)", src)
	}
	if _, err := exec.LookPath("dart"); err != nil {
		return fmt.Errorf("dart is not found: install the Dart SDK first (https://dart.dev/get-dart)")
	}
	data, err := dartDataDir()
	if err != nil {
		return err
	}
	// only the packages that the lock of the extractor names; no code of them runs, but the extractor that is compiled
	get := exec.Command("dart", "pub", "get", "--enforce-lockfile")
	get.Dir = src
	get.Stdout, get.Stderr = os.Stderr, os.Stderr
	if err := get.Run(); err != nil {
		return fmt.Errorf("dart pub get --enforce-lockfile: %v", err)
	}
	fmt.Fprintf(os.Stderr, "rivus: compiling the Dart extractor from %s (dart compile exe; about 10 s)\n", src)
	exe, key, err := dartExeFor(src, "dart", data)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(dartInstall{Source: src, Key: key, Exe: exe})
	if err := os.WriteFile(filepath.Join(data, "installed.json"), b, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "rivus: done: %s\n", exe)
	return nil
}
