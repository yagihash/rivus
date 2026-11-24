package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// The source hash of a graph file (docs/DESIGN-graph.md, 4). It is made of what
// can change the result: the flags, the rules, the version of rivus, the
// environment, the toolchains, the files the load read, and the list of the
// files of the tree. What "read" means is not guessed: for Go it is the list of
// the files that go/packages has (the standard library and the dependencies
// included); for the other languages the extractor does not say, so the whole
// tree is hashed (with the files its IR names outside the tree, and the lock
// files, and the SDK versions).

// rivusVersionFn is the version of rivus that goes in the key: the hash of the
// running executable (every build is a different version, a "dev" build
// included), with the version Go recorded. A test changes it to see that a
// new rivus does not use the old file.
var rivusVersionFn = executableVersion

// versionStamp is set at build time (-ldflags "-X main.versionStamp=...") by a release.
var versionStamp string

var (
	exeOnce sync.Once
	exeVer  string
)

func executableVersion() string {
	exeOnce.Do(func() {
		v := "unknown"
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
			v = bi.Main.Version
		}
		sum := "unreadable"
		if p, err := os.Executable(); err == nil {
			if b, err := os.ReadFile(p); err == nil {
				sum = graphdb.HashBytes(b)[:16]
			}
		}
		exeVer = v + "+" + sum + versionStamp
	})
	return exeVer
}

// keyCtx makes the key. At build time l is the program that was read; at check
// time the files come from what the graph file recorded.
type keyCtx struct {
	o         *analyzeOpts
	dir       string // absolute
	graphPath string // absolute: not a file of the tree
	l         *loaded
	stored    *graphdb.KeyParts
	since     time.Time
	patterns  []string
	goroot    string
	gocache   string
}

// keyResult is the key and what is stored beside it.
type keyResult struct {
	graphdb.Parts
	Roots []string
	Hash  string
	Langs []string
}

func (k *keyCtx) langs() ([]string, error) {
	if k.o.irFile != "" {
		return []string{"ir"}, nil
	}
	return k.o.languages()
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (k *keyCtx) compute() (*keyResult, error) {
	o := k.o
	// the directory as it is, whatever the way it was spelled (a link in the path)
	if real, err := filepath.EvalSymlinks(k.dir); err == nil {
		k.dir = real
	}
	langs, err := k.langs()
	if err != nil {
		return nil, err
	}
	cfg := map[string]string{}
	cfg["rivus"] = rivusVersionFn()
	cfg["flags.lang"] = strings.Join(langs, ",")
	cfg["flags.tests"] = strconv.FormatBool(o.tests)
	cfg["flags.no-deps"] = strconv.FormatBool(o.noDeps)
	cfg["flags.k"] = strconv.Itoa(o.k)
	cfg["flags.numeric-taint"] = strconv.FormatBool(o.numeric)
	cfg["flags.max-implementations"] = strconv.Itoa(o.maxImpl)
	cfg["flags.no-builtin-rules"] = strconv.FormatBool(o.noBuiltin)
	cfg["flags.no-project-rules"] = strconv.FormatBool(o.noProject)
	cfg["flags.deterministic-only"] = strconv.FormatBool(o.deterministic)
	// the depth the dependencies were read at (the effective one: the default is 2, but not deeper than -k): an index made
	// at another depth, or by a version that had no such depth, is not this one
	cfg["flags.dep-k"] = strconv.Itoa(o.depDepth())
	if o.charTaint && !o.numeric {
		// the elements of an index made with it carry the mark: not one made without it. Set only when on (and when it does
		// something), so that the key of an index made without it is what it was.
		cfg["flags.char-taint"] = "true"
	}
	if len(o.bodyReplaced) > 0 {
		// the packages whose body is not read are not in the file: another list is another file. Set only when named, so
		// that the key of an index made without it is what it was. (The declarations of the rule files are in the key
		// with the files, and those of the built-in rules with their hash.)
		cfg["flags.body-replaced"] = strings.Join(sortedUnique(o.bodyReplaced), "\n")
	}
	if o.readDeclared {
		cfg["flags.read-declared-bodies"] = "true"
	}
	if len(o.depDirs) > 0 {
		cfg["flags.dep-dir"] = strings.Join(sortedUnique(o.depDirs), "\n")
	}
	allow := append([]string(nil), o.allowDepsUnder...)
	sort.Strings(allow)
	cfg["flags.allow-deps-under"] = strings.Join(allow, "\n")
	patterns := k.patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."} // what the load reads when none is named
	}
	cfg["flags.patterns"] = strings.Join(patterns, "\n")
	var entries []graphdb.Entry

	// the rules: the built-in ones by their content, the others as files
	if !o.noBuiltin {
		h, err := builtin.Hash()
		if err != nil {
			return nil, err
		}
		cfg["rules.builtin"] = h
	}
	var ruleFiles []string
	if !o.noProject {
		found, err := o.projectRuleFiles()
		if err != nil {
			return nil, err
		}
		for _, f := range found {
			ruleFiles = append(ruleFiles, f)
		}
	}
	for _, f := range o.rules {
		ruleFiles = append(ruleFiles, f)
	}
	var names []string
	for _, f := range ruleFiles {
		abs, err := realAbs(f)
		if err != nil {
			return nil, err
		}
		names = append(names, abs)
		entries = append(entries, graphdb.Entry{Path: abs, Abs: abs, Kind: "rules"})
	}
	cfg["rules.files"] = strings.Join(names, "\n")

	var roots []string
	if o.irFile != "" {
		// the stream is the input: the source it names is not read
		abs, err := realAbs(o.irFile)
		if err != nil {
			return nil, err
		}
		cfg["flags.ir"] = abs
		entries = append(entries, graphdb.Entry{Path: abs, Abs: abs, Kind: "ir"})
	} else {
		roots = []string{k.dir}
		if k.stored != nil {
			// the directories the file was made from, but for the first, which is where the check is made
			if len(k.stored.Roots) > 1 {
				roots = append(roots, k.stored.Roots[1:]...)
			}
		} else if k.l != nil && k.l.gossa != nil {
			for _, d := range k.l.gossa.Inputs.ModuleDirs {
				if d != k.dir {
					roots = append(roots, d)
				}
			}
		}
		if has(langs, langGo) {
			if err := k.goConfig(cfg); err != nil {
				return nil, err
			}
		}
		for _, lang := range []string{langDart, langRust} {
			if has(langs, lang) {
				k.extractorConfig(cfg, lang)
			}
		}
	}

	var manifest string
	if o.irFile == "" {
		exclude := map[string]bool{}
		if k.graphPath != "" {
			exclude[k.graphPath] = true
		}
		tree, err := graphdb.ListTree(roots, exclude)
		if err != nil {
			return nil, err
		}
		manifest = graphdb.Manifest(tree)
		// the languages whose extractor does not say what it read: every file of the tree
		if has(langs, langDart) || has(langs, langRust) {
			entries = append(entries, tree...)
		}
		entries = append(entries, k.lockEntries(roots, langs)...)
		more, err := k.inputEntries(roots)
		if err != nil {
			return nil, err
		}
		entries = append(entries, more...)
	}
	files, err := graphdb.HashFiles(dedupe(entries), k.since)
	if err != nil {
		return nil, err
	}
	res := &keyResult{Parts: graphdb.Parts{Config: cfg, Files: files, Manifest: manifest}, Roots: roots, Langs: langs}
	res.Hash = graphdb.Compose(cfg, files, manifest)
	return res, nil
}

func dedupe(entries []graphdb.Entry) []graphdb.Entry {
	seen := map[string]bool{}
	out := entries[:0:0]
	for _, e := range entries {
		if seen[e.Path] {
			continue
		}
		seen[e.Path] = true
		out = append(out, e)
	}
	return out
}

// inputEntries are the files that are read from where the load says they are:
// at build, what the load listed; at check, what the graph file recorded.
func (k *keyCtx) inputEntries(roots []string) ([]graphdb.Entry, error) {
	var out []graphdb.Entry
	if k.stored != nil {
		for _, f := range k.stored.Files {
			switch f.Kind {
			case "source", "dep", "stdlib", "module", "foreign", "generated":
				out = append(out, graphdb.Entry{Path: f.Path, Abs: k.absOf(f.Path), Kind: f.Kind})
			}
		}
		return out, nil
	}
	if k.l == nil {
		return nil, nil
	}
	goroot := k.goroot
	if k.l.gossa != nil {
		in := k.l.gossa.Inputs
		kind := func(abs string) string {
			if goroot != "" && strings.HasPrefix(abs, goroot+string(filepath.Separator)) {
				return "stdlib"
			}
			// made by the load itself (cgo), from the sources: hashed, but written while it runs
			if k.gocache != "" && strings.HasPrefix(abs, k.gocache+string(filepath.Separator)) {
				return "generated"
			}
			for _, r := range roots {
				if strings.HasPrefix(abs, r+string(filepath.Separator)) {
					return "source"
				}
			}
			return "dep"
		}
		for _, f := range in.Files {
			out = append(out, graphdb.Entry{Path: k.l.gossa.Rel(f), Abs: f, Kind: kind(f)})
		}
		for _, f := range in.GoMods {
			out = append(out, graphdb.Entry{Path: k.l.gossa.Rel(f), Abs: f, Kind: "module"})
		}
	}
	// the files that the IR of another language names, outside the tree (a dependency in a package cache)
	if k.l.ir != nil {
		for _, f := range k.l.ir.Files {
			if !filepath.IsAbs(f) {
				continue
			}
			in := false
			for _, r := range roots {
				if strings.HasPrefix(f, r+string(filepath.Separator)) {
					in = true
				}
			}
			if !in {
				out = append(out, graphdb.Entry{Path: filepath.ToSlash(f), Abs: f, Kind: "foreign"})
			}
		}
	}
	return out, nil
}

func (k *keyCtx) absOf(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(k.dir, filepath.FromSlash(p))
}

// lockEntries are the files that fix the versions of the dependencies and of
// the toolchain, where they are: they are looked for again at every check, so
// that one that has appeared is a change.
func (k *keyCtx) lockEntries(roots []string, langs []string) []graphdb.Entry {
	var names []string
	if has(langs, langGo) {
		names = append(names, "go.sum", "go.work", "go.work.sum", filepath.Join("vendor", "modules.txt"))
	}
	if has(langs, langDart) {
		names = append(names, "pubspec.yaml", "pubspec.lock", filepath.Join(".dart_tool", "package_config.json"))
	}
	if has(langs, langRust) {
		names = append(names, "Cargo.toml", "Cargo.lock", "rust-toolchain", "rust-toolchain.toml", filepath.Join(".cargo", "config.toml"), filepath.Join(".cargo", "config"))
	}
	var out []graphdb.Entry
	for i, r := range roots {
		for _, n := range names {
			p := filepath.Join(r, n)
			if fi, err := os.Lstat(p); err == nil && !fi.IsDir() {
				display := filepath.ToSlash(p)
				if i == 0 {
					display = filepath.ToSlash(n)
				}
				out = append(out, graphdb.Entry{Path: display, Abs: p, Kind: "lock"})
			}
		}
	}
	// a go.work above the directory is found by the go command, which says where it is
	return out
}

// goConfig puts the Go environment in the key: every variable "go env" knows, so that the
// version, GOOS, GOARCH, GOFLAGS (the build tags), CGO_ENABLED, GOWORK and the others
// are in it without a list that could be missing one.
func (k *keyCtx) goConfig(cfg map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "env", "-json")
	cmd.Dir = k.dir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("go env: %w", err)
	}
	var env map[string]string
	if err := json.Unmarshal(out, &env); err != nil {
		return fmt.Errorf("go env: %w", err)
	}
	for name, v := range env {
		// where the module is is where the check is made: the file is named relative to it, as its files are
		if (name == "GOMOD" || name == "GOWORK") && filepath.IsAbs(v) {
			if rel, err := filepath.Rel(k.dir, v); err == nil && !strings.HasPrefix(rel, "..") {
				v = filepath.ToSlash(rel)
			}
		}
		// the flags of the C compiler hold the name of a temporary directory that is new at every run
		if name == "GOGCCFLAGS" {
			continue
		}
		cfg["env.go."+name] = v
	}
	k.goroot = env["GOROOT"]
	k.gocache = env["GOCACHE"]
	return nil
}

// extractorConfig puts the version of an extractor and of its SDK in the key. What cannot be found is
// written as such: it is the same at the next check, and a version that appears is a change.
func (k *keyCtx) extractorConfig(cfg map[string]string, lang string) {
	words, err := k.o.extractorCommand(lang)
	if err != nil {
		cfg["tool."+lang+".extractor"] = "missing"
		return
	}
	cfg["tool."+lang+".command"] = strings.Join(words, " ")
	if p, err := exec.LookPath(words[0]); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			sum := sha256.Sum256(b)
			cfg["tool."+lang+".extractor"] = hex.EncodeToString(sum[:])
		} else {
			cfg["tool."+lang+".extractor"] = "unreadable"
		}
	}
	if lang == langDart {
		cfg["tool.dart.extractor-version"] = runVersion(append(append([]string(nil), words...), "--version")...)
		// the line of the version only: a shim (mise) may print what it installs first
		sdk := "unrecognized"
		for _, l := range strings.Split(runVersion("dart", "--version"), "\n") {
			if strings.Contains(l, "Dart SDK version") {
				sdk = strings.TrimSpace(l)
			}
		}
		cfg["tool.dart.sdk"] = sdk
	}
}

// runVersion runs a command that prints a version and returns what it printed.
func runVersion(argv ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		return "unavailable: " + strings.TrimSpace(buf.String())
	}
	return strings.TrimSpace(buf.String())
}

// realAbs is the absolute path of a file with the links in it followed (a file that is not there is by its absolute path).
func realAbs(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real, nil
	}
	return abs, nil
}

// sortedUnique is the list sorted, without repeats.
func sortedUnique(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	n := 0
	for i, s := range out {
		if i == 0 || s != out[n-1] {
			out[n] = s
			n++
		}
	}
	return out[:n]
}
