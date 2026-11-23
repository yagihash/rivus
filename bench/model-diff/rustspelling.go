package main

// The spelling check of the rows of a Rust model (`-lang rust`; local only, macOS with the sandbox).
//
// A row names a function or a type as the extractor writes it: the path that the crate makes public (`actix_web::HttpRequest::path`),
// not the path of the module that defines the item (`actix_web::request::HttpRequest::path`). This check does by command what
// docs/MODEL-RUNBOOK.md used to ask for by hand: it builds a small crate that depends on the real crate (the version pinned), with
// one probe per row that names the item the way the row does, reads the crate with `rivus debug extract -lang rust` (under the
// sandbox), and looks at what the extractor wrote for each probe.
//
// The probes are not found by listing the public API: a row is the thing that is asked about, so each row gets a function that
// names it. A function is named by a path used as a value (`std::hint::black_box(path)`; the IR has it as a `make_closure` with
// the symbol as the extractor spells it), a type by a parameter of a function (the IR has the type of the parameter). The compiler
// is the one that says whether the path exists: a probe that does not compile names the row in its error (`src/lib.rs:LINE`), and
// the row is dropped from the next round with the message. A path that resolves but cannot be used as a value without more type
// information (`E0282`, a method with a type parameter) is "resolves" and not "found": the compiler knows the path, the extractor
// has not been seen to write it.
//
// Not here: the other half of the differential test (two analyses and their comparison), `-write-verified`, `-write-dead-allow`.
// Rows of `field` sources are not checked (said in the report).

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/yagihash/rivus/internal/model"
)

// Exit codes of `-lang rust`.
const (
	exitRustOK         = 0 // every row is found
	exitRustMissing    = 1 // a row is not found, or the extractor writes it another way
	exitRustUsage      = 2 // the flags, or a step that failed (fetch, extract)
	exitRustSkipped    = 3 // not run: not macOS, or no extractor, or no cargo (nothing was checked)
	exitRustUnconfirmd = 4 // no row is missing, but some only resolve (the compiler knows the path; the extractor was not seen to write it)
)

// RustRow is a row of a model that is probed.
type RustRow struct {
	Kind   string // "symbol" (a function), "type" (a param_type source)
	Symbol string // the symbol, or the type with its `&`
	Where  string // the section of the file: sinks, propagators, ...
	File   string
}

// RustRows is what the rows of a crate are made into.
type RustRows struct {
	Probed  []RustRow
	Skipped []string // "<what>: <why>": a row that is not probed
	Other   int      // rows of other crates (not the one that is checked)
	// Versions: the `verified_versions` entries of the crate (the default of -version).
	Versions []model.VerifiedVersion
}

// crateIdent is the name of the crate as rustc and the IR write it (`actix-web` is `actix_web`).
func crateIdent(pkg string) string { return strings.ReplaceAll(pkg, "-", "_") }

var plainPathRe = regexp.MustCompile(`^&?(?:mut )?(?:r#)?[A-Za-z_][A-Za-z0-9_]*(?:::(?:r#)?[A-Za-z_][A-Za-z0-9_]*)+$`)

// LoadRustRows reads the model files and returns the rows that name the crate.
func LoadRustRows(files []string, pkg string) (*RustRows, error) {
	crate := crateIdent(pkg)
	out := &RustRows{}
	seen := map[string]bool{}
	add := func(file, where, kind, sym string) {
		if sym == "" {
			return
		}
		if !strings.HasPrefix(strings.TrimLeft(sym, "&"), crate+"::") && !strings.HasPrefix(strings.TrimPrefix(strings.TrimLeft(sym, "&"), "mut "), crate+"::") {
			out.Other++
			return
		}
		if !plainPathRe.MatchString(sym) {
			out.Skipped = append(out.Skipped, fmt.Sprintf("%s (%s): a shape that a probe does not name (generic arguments, a trait impl)", sym, where))
			return
		}
		if seen[kind+" "+sym] {
			return
		}
		seen[kind+" "+sym] = true
		out.Probed = append(out.Probed, RustRow{Kind: kind, Symbol: sym, Where: where, File: filepath.Base(file)})
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var rs model.RuleSet
		if err := yaml.Unmarshal(data, &rs); err != nil {
			return nil, fmt.Errorf("%s: %v", f, err)
		}
		for _, v := range rs.VerifiedVersions {
			if model.SameCrate(v.Package, pkg) {
				out.Versions = append(out.Versions, v)
			}
		}
		for _, s := range rs.Sources {
			switch s.Kind {
			case "param_type":
				add(f, "sources", "type", s.Type)
			case "field":
				if strings.HasPrefix(s.Field, crate+"::") {
					out.Skipped = append(out.Skipped, fmt.Sprintf("%s (sources, field): field rows are not checked in this version", s.Field))
				} else if s.Field != "" {
					out.Other++
				}
			default:
				add(f, "sources", "symbol", s.Symbol)
			}
		}
		for _, s := range rs.Sinks {
			add(f, "sinks", "symbol", s.Symbol)
		}
		for _, s := range rs.Sanitizers {
			add(f, "sanitizers", "symbol", s.Symbol)
		}
		for _, p := range rs.Propagators {
			add(f, "propagators", "symbol", p.Symbol)
		}
		for _, c := range rs.ConfigReads {
			add(f, "config_reads", "symbol", c.Symbol)
		}
	}
	return out, nil
}

// ---- the probe crate ----

// A probe is one function of the generated lib.rs, on its own line.
type probe struct {
	Row     RustRow
	Variant int
	Name    string
}

const maxVariants = 4 // 0: as written; 1..3: the type of the path (or of the parameter) with that many type arguments

// typeSegment is the index of the segment that names a type in a function path (the last segment that starts with a capital before
// the final one), or -1.
func typeSegment(segs []string) int {
	for i := len(segs) - 2; i >= 1; i-- {
		if c := segs[i][0]; c >= 'A' && c <= 'Z' {
			return i
		}
	}
	return -1
}

// ProbeLine writes the function of a probe: `pub fn pN<A, B, C>() { ... }` for a symbol, `pub fn tN<A, B, C>(_x: T) {}` for a type.
// ok is false when the variant does not exist for the row (a path with no type in it has only variant 0).
func ProbeLine(n int, r RustRow, variant int) (line string, ok bool) {
	generics := "<A, B, C>"
	if r.Kind == "type" {
		pre, path := "", r.Symbol
		for _, p := range []string{"&mut ", "&"} {
			if strings.HasPrefix(path, p) {
				pre, path = p, strings.TrimPrefix(path, p)
				break
			}
		}
		args := ""
		if variant > 0 {
			args = "<" + strings.Join([]string{"A", "B", "C"}[:variant], ", ") + ">"
		}
		return fmt.Sprintf("pub fn t%d%s(_x: %s%s%s) {}", n, generics, pre, path, args), true
	}
	segs := strings.Split(r.Symbol, "::")
	path := r.Symbol
	if variant > 0 {
		i := typeSegment(segs)
		if i < 0 {
			return "", false
		}
		segs[i] += "::<" + strings.Join([]string{"A", "B", "C"}[:variant], ", ") + ">"
		path = strings.Join(segs, "::")
	}
	return fmt.Sprintf("pub fn p%d%s() { let f = %s; std::hint::black_box(f); }", n, generics, path), true
}

// ---- what the compiler says ----

// CompileError is one error of rustc at a line of the probe.
type CompileError struct {
	Line int
	Code string // E0425 ...
	Msg  string
}

var (
	errHeadRe = regexp.MustCompile(`^error(?:\[(E\d+)\])?: (.*)$`)
	errLocRe  = regexp.MustCompile(`^\s*--> src/lib\.rs:(\d+):`)
)

// ParseCompileErrors reads the stderr of the extract stage: the errors that point into src/lib.rs.
func ParseCompileErrors(stderr string) []CompileError {
	var out []CompileError
	seen := map[string]bool{}
	var head *CompileError
	for _, l := range strings.Split(stderr, "\n") {
		if m := errHeadRe.FindStringSubmatch(l); m != nil {
			head = &CompileError{Code: m[1], Msg: m[2]}
			continue
		}
		if strings.HasPrefix(l, "warning") {
			head = nil
			continue
		}
		if m := errLocRe.FindStringSubmatch(l); m != nil && head != nil {
			var n int
			fmt.Sscan(m[1], &n)
			e := *head
			e.Line = n
			if k := fmt.Sprintf("%d|%s|%s", e.Line, e.Code, e.Msg); !seen[k] {
				seen[k] = true
				out = append(out, e)
			}
			head = nil
		}
	}
	return out
}

// Verdict of an error for the variant of a probe.
type Verdict int

const (
	NextVariant Verdict = iota // wrong number of type arguments: try the next
	Resolves                   // the path is known, the probe cannot name it as a value or a type here (type annotations, trait bounds, a private module on the path)
	Absent                     // the path does not exist (or is private)
)

// Judge classifies an error.
func Judge(e CompileError) Verdict {
	switch e.Code {
	case "E0107":
		return NextVariant
	case "E0282", "E0283", "E0790", "E0284":
		return Resolves
	case "E0599":
		if strings.Contains(e.Msg, "exists for") || strings.Contains(e.Msg, "trait bounds were not satisfied") {
			return Resolves
		}
	case "E0277", "E0308":
		return Resolves
	case "E0603":
		// the item exists, the path goes through a private module: the extractor may still write that path (it names an item by the
		// module that makes it visible), but a probe cannot use it
		return Resolves
	}
	return Absent
}

// ---- what the extractor wrote ----

// IRFacts is what is read from the IR of the probe crate and the crate: per probe function the spellings, and the ids of the
// functions of the crate (the definition paths).
type IRFacts struct {
	Probe map[string]*ProbeFacts // by the name of the function (p1, t2)
	Defs  []string               // ids of the functions of the crate (definition paths)
}

// ProbeFacts: for a probe function, the names of the functions it makes (make_closure), the symbols it calls, the types of its parameters.
type ProbeFacts struct {
	Funcs  []string
	Calls  []string
	Params []string
}

type irLine struct {
	Kind string `json:"kind"`
	Func *struct {
		ID     string `json:"id"`
		Dep    bool   `json:"dep"`
		Params []struct {
			Type string `json:"type"`
		} `json:"params"`
		Blocks []struct {
			Instrs []struct {
				Op     string `json:"op"`
				Func   string `json:"func"`
				Callee *struct {
					Symbol string `json:"symbol"`
				} `json:"callee"`
			} `json:"instrs"`
		} `json:"blocks"`
	} `json:"func"`
}

// ReadIR reads the JSON Lines of the extractor. probeCrate is the name of the probe crate, crate the crate that is checked.
func ReadIR(data []byte, probeCrate, crate string) (*IRFacts, error) {
	facts := &IRFacts{Probe: map[string]*ProbeFacts{}}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	probePrefix := `"id":"` + probeCrate + `::`
	crateIDPrefix := `"id":"` + crate + `::`
	for sc.Scan() {
		line := sc.Bytes()
		isProbe := bytes.Contains(line, []byte(probePrefix))
		if !isProbe {
			// only the ids of the functions of the crate are needed from the rest
			if i := bytes.Index(line, []byte(crateIDPrefix)); i >= 0 && bytes.Contains(line[:i+len(crateIDPrefix)], []byte(`"func":`)) {
				rest := line[i+len(`"id":"`):]
				if j := bytes.IndexByte(rest, '"'); j > 0 {
					facts.Defs = append(facts.Defs, string(rest[:j]))
				}
			}
			continue
		}
		var l irLine
		if err := json.Unmarshal(line, &l); err != nil || l.Kind != "func" || l.Func == nil {
			continue
		}
		name := strings.TrimPrefix(l.Func.ID, probeCrate+"::")
		f := &ProbeFacts{}
		for _, p := range l.Func.Params {
			f.Params = append(f.Params, p.Type)
		}
		for _, b := range l.Func.Blocks {
			for _, in := range b.Instrs {
				if in.Op == "make_closure" && in.Func != "" {
					f.Funcs = append(f.Funcs, in.Func)
				}
				if in.Op == "call" && in.Callee != nil {
					f.Calls = append(f.Calls, in.Callee.Symbol)
				}
			}
		}
		facts.Probe[name] = f
	}
	return facts, sc.Err()
}

// StripGenerics removes the `<...>` of a path (balanced; `->` does not close one), and the `::` before a turbofish: the way the
// models write a type or a function, without its type arguments.
func StripGenerics(s string) string {
	var b strings.Builder
	depth := 0
	var prev rune
	for _, r := range s {
		switch {
		case r == '<':
			if depth == 0 && strings.HasSuffix(b.String(), "::") {
				str := b.String()
				b.Reset()
				b.WriteString(str[:len(str)-2])
			}
			depth++
		case r == '>' && prev != '-':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			b.WriteRune(r)
		}
		prev = r
	}
	return b.String()
}

// Status of a row.
type Status string

const (
	Found      Status = "found"
	Spelled    Status = "spelled-differently" // it compiles, and the extractor writes another symbol
	ResolvesOK Status = "resolves"            // the compiler knows the item; no probe could name it for the extractor
	NotFound   Status = "not-found"           // the compiler does not know the path
)

// RowResult is the result of one row.
type RowResult struct {
	Row     RustRow
	Status  Status
	Wrote   string   // Spelled: what the extractor wrote
	Message string   // NotFound, ResolvesOK: the message of the compiler
	Closest []string // the ids of the crate that end in the same two segments
}

// Closest returns the ids of the functions of the crate that end in the last two segments of the symbol (`HttpRequest::path`), the
// last one if the symbol has no type; at most 5.
func Closest(defs []string, sym string) []string {
	segs := strings.Split(strings.TrimLeft(StripGenerics(sym), "&"), "::")
	n := 2
	if len(segs) < 2 {
		n = 1
	}
	tail := "::" + strings.Join(segs[len(segs)-n:], "::")
	seen := map[string]bool{}
	var out []string
	for _, d := range defs {
		d = StripGenerics(d)
		if strings.HasSuffix(d, tail) && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Strings(out)
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// Judge a compiled probe: does the extractor write the row's symbol.
func MatchProbe(r RustRow, f *ProbeFacts) (Status, string) {
	if f == nil {
		return NotFound, ""
	}
	if r.Kind == "type" {
		if len(f.Params) == 0 {
			return NotFound, ""
		}
		got := StripGenerics(f.Params[0])
		if got == r.Symbol {
			return Found, ""
		}
		return Spelled, got
	}
	var cand []string
	for _, c := range append(append([]string{}, f.Funcs...), f.Calls...) {
		c = StripGenerics(c)
		if strings.HasPrefix(c, "std::hint::black_box") {
			continue
		}
		if c == r.Symbol {
			return Found, ""
		}
		cand = append(cand, c)
	}
	if len(cand) == 0 {
		return NotFound, ""
	}
	return Spelled, cand[0]
}

// ---- the run ----

// RustOptions are the flags of `-lang rust`.
type RustOptions struct {
	Pkg, Version, Features string
	NoDefaultFeatures      bool
	Files                  []string // model files
	Repo, Out              string
	Rivus, Extractor       string
	Timeout                string
}

// SkipReason says why the check cannot run here ("" if it can).
func SkipReason(extractor string) string {
	if runtime.GOOS != "darwin" {
		return "this check runs the extractor under the sandbox of macOS; this is " + runtime.GOOS
	}
	if _, err := exec.LookPath("cargo"); err != nil {
		return "cargo is not on PATH (the probe crate needs `cargo generate-lockfile` and `cargo fetch`)"
	}
	if extractor == "" {
		return "no Rust extractor: build extractor/rust and name it with -rust-extractor or RIVUS_RUST_EXTRACTOR (never `rivus setup rust`)"
	}
	if st, err := os.Stat(extractor); err != nil || st.IsDir() {
		return "the Rust extractor is not a file: " + extractor
	}
	return ""
}

// listedInQueue reports whether the crate is named in docs/MODEL-QUEUE.md (the packages that may be fetched without asking).
func listedInQueue(repo, pkg string) bool {
	data, err := os.ReadFile(filepath.Join(repo, "docs", "MODEL-QUEUE.md"))
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(pkg)) || bytes.Contains(data, []byte(strings.ReplaceAll(pkg, "_", "-")))
}

func realRustMain(o RustOptions) int {
	say := func(f string, a ...any) { fmt.Fprintf(os.Stdout, f+"\n", a...) }
	if reason := SkipReason(o.Extractor); reason != "" {
		say("SKIPPED (exit %d): %s", exitRustSkipped, reason)
		return exitRustSkipped
	}
	if !listedInQueue(o.Repo, o.Pkg) {
		fmt.Fprintf(os.Stderr, "model-diff: %s is not in docs/MODEL-QUEUE.md: only the listed public crates are fetched without asking\n", o.Pkg)
		return exitRustUsage
	}
	rows, err := LoadRustRows(o.Files, o.Pkg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "model-diff:", err)
		return exitRustUsage
	}
	if len(rows.Probed) == 0 {
		fmt.Fprintf(os.Stderr, "model-diff: no row of %s in %s\n", o.Pkg, strings.Join(o.Files, ", "))
		return exitRustUsage
	}
	if o.Version == "" {
		for _, v := range rows.Versions {
			if v.CheckedWith != "" {
				o.Version = v.CheckedWith
			}
		}
	}
	if o.Version == "" {
		fmt.Fprintln(os.Stderr, "model-diff: -version is needed (the model has no checked_with for the crate)")
		return exitRustUsage
	}
	if o.Rivus == "" {
		o.Rivus = filepath.Join(o.Out, "rivus")
		c := exec.Command("go", "build", "-o", o.Rivus, "./cmd/rivus")
		c.Dir = o.Repo
		if b, err := c.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "model-diff: go build: %v\n%s", err, b)
			return exitRustUsage
		}
	}
	dir := filepath.Join(o.Out, "probe")
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "model-diff:", err)
		return exitRustUsage
	}
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(ProbeManifest(o.Pkg, o.Version, o.Features, o.NoDefaultFeatures)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "model-diff:", err)
		return exitRustUsage
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "lib.rs"), nil, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "model-diff:", err)
		return exitRustUsage
	}
	// fetch only: `cargo generate-lockfile` and `cargo fetch` read the index and download the sources; nothing is built or run
	for _, args := range [][]string{{"generate-lockfile"}, {"fetch"}} {
		c := exec.Command("cargo", args...)
		c.Dir = dir
		if b, err := c.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "model-diff: cargo %s: %v\n%s", args[0], err, b)
			return exitRustUsage
		}
	}
	results, facts, runs, err := probeRows(o, dir, rows.Probed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "model-diff:", err)
		return exitRustUsage
	}
	for i := range results {
		if facts != nil && (results[i].Status == NotFound || results[i].Status == ResolvesOK || results[i].Status == Spelled) {
			results[i].Closest = Closest(facts.Defs, results[i].Row.Symbol)
		}
	}
	return reportRust(os.Stdout, o, rows, results, runs)
}

// ProbeManifest is the Cargo.toml of the probe crate: the crate pinned (`=version`).
func ProbeManifest(pkg, version, features string, noDefault bool) string {
	dep := fmt.Sprintf("version = \"=%s\"", version)
	if noDefault {
		dep += ", default-features = false"
	}
	if features != "" {
		var q []string
		for _, f := range strings.Split(features, ",") {
			if f = strings.TrimSpace(f); f != "" {
				q = append(q, fmt.Sprintf("%q", f))
			}
		}
		dep += ", features = [" + strings.Join(q, ", ") + "]"
	}
	return fmt.Sprintf("[package]\nname = \"rivus-spelling-probe\"\nversion = \"0.0.0\"\nedition = \"2021\"\npublish = false\n\n[dependencies]\n%s = { %s }\n\n[workspace]\n", pkg, dep)
}

const probeCrate = "rivus_spelling_probe"

// probeRows runs the rounds: every pending row has a probe at its variant; the extract stage compiles them; the rows that the
// compiler points at move on (next variant) or are settled (resolves, absent); until the stage succeeds.
func probeRows(o RustOptions, dir string, rows []RustRow) ([]RowResult, *IRFacts, int, error) {
	type state struct {
		row      RustRow
		variant  int
		done     bool
		res      RowResult
		resolved string // the message of the first "resolves"
	}
	st := make([]*state, len(rows))
	for i, r := range rows {
		st[i] = &state{row: r}
	}
	for round := 1; round <= 3*maxVariants+2; round++ {
		var b strings.Builder
		line := map[int]int{} // line of lib.rs -> index in st
		n := 0
		for i, s := range st {
			if s.done {
				continue
			}
			text, ok := ProbeLine(i, s.row, s.variant)
			if !ok {
				s.done = true
				s.res = settle(s.row, s.resolved)
				continue
			}
			b.WriteString(text + "\n")
			n++
			line[n] = i
		}
		if err := os.WriteFile(filepath.Join(dir, "src", "lib.rs"), []byte(b.String()), 0o644); err != nil {
			return nil, nil, 0, err
		}
		out, stderr, err := extractStage(o, dir)
		if err == nil {
			facts, err := ReadIR(out, probeCrate, crateIdent(o.Pkg))
			if err != nil {
				return nil, nil, 0, err
			}
			for _, i := range line {
				s := st[i]
				name := fmt.Sprintf("p%d", i)
				if s.row.Kind == "type" {
					name = fmt.Sprintf("t%d", i)
				}
				status, wrote := MatchProbe(s.row, facts.Probe[name])
				s.done = true
				s.res = RowResult{Row: s.row, Status: status, Wrote: wrote}
			}
			res := make([]RowResult, len(st))
			for i, s := range st {
				res[i] = s.res
			}
			return res, facts, round, nil
		}
		errs := ParseCompileErrors(stderr)
		if len(errs) == 0 {
			return nil, nil, 0, fmt.Errorf("the extract stage failed and no error points into the probe:\n%s", tail(stderr, 40))
		}
		moved := false
		for _, e := range errs {
			i, ok := line[e.Line]
			if !ok || st[i].done {
				continue
			}
			s := st[i]
			moved = true
			switch Judge(e) {
			case NextVariant:
				s.variant++
			case Resolves:
				if s.resolved == "" {
					s.resolved = e.Code + ": " + e.Msg
				}
				s.variant++
			default:
				s.done = true
				s.res = RowResult{Row: s.row, Status: NotFound, Message: e.Code + ": " + e.Msg}
				continue
			}
			if s.variant >= maxVariants {
				s.done = true
				s.res = settle(s.row, s.resolved)
			}
		}
		if !moved {
			return nil, nil, 0, fmt.Errorf("the extract stage failed with errors that are not at a probe:\n%s", tail(stderr, 40))
		}
	}
	return nil, nil, 0, fmt.Errorf("the probes did not settle")
}

func settle(r RustRow, resolved string) RowResult {
	if resolved != "" {
		return RowResult{Row: r, Status: ResolvesOK, Message: resolved}
	}
	return RowResult{Row: r, Status: NotFound, Message: "no variant of the probe compiled"}
}

func tail(s string, n int) string {
	l := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(l) > n {
		l = l[len(l)-n:]
	}
	return strings.Join(l, "\n")
}

// extractStage runs `rivus debug extract -lang rust` (the sandbox is its default; `-unsafe-host-exec` is never passed).
func extractStage(o RustOptions, dir string) (stdout []byte, stderr string, err error) {
	c := exec.Command(o.Rivus, "debug", "extract", "-lang", "rust", "-C", dir)
	c.Env = append(os.Environ(), "RIVUS_RUST_EXTRACTOR="+o.Extractor)
	var out, errb bytes.Buffer
	c.Stdout, c.Stderr = &out, &errb
	err = c.Run()
	return out.Bytes(), errb.String(), err
}

// ---- the report ----

func reportRust(w *os.File, o RustOptions, rows *RustRows, res []RowResult, rounds int) int {
	count := map[Status]int{}
	for _, r := range res {
		count[r.Status]++
	}
	fmt.Fprintf(w, "model-diff -lang rust (spelling): %s %s, %d row(s) probed in %d round(s)\n", o.Pkg, o.Version, len(res), rounds)
	for _, r := range res {
		fmt.Fprintf(w, "  %-20s %-11s %s\n", r.Status, r.Row.Where, r.Row.Symbol)
		if r.Wrote != "" {
			fmt.Fprintf(w, "      the extractor writes: %s\n", r.Wrote)
		}
		if r.Message != "" {
			fmt.Fprintf(w, "      compiler: %s\n", r.Message)
		}
		if len(r.Closest) > 0 && r.Status != Found {
			fmt.Fprintf(w, "      closest (definition paths in the crate): %s\n", strings.Join(r.Closest, ", "))
		}
	}
	for _, s := range rows.Skipped {
		fmt.Fprintf(w, "  not probed: %s\n", s)
	}
	fmt.Fprintf(w, "  found %d / %d; spelled differently %d; resolves only %d; not found %d; not probed %d; rows of other crates %d\n",
		count[Found], len(res), count[Spelled], count[ResolvesOK], count[NotFound], len(rows.Skipped), rows.Other)
	switch {
	case count[Spelled] > 0 || count[NotFound] > 0:
		fmt.Fprintf(w, "  exit %d\n", exitRustMissing)
		return exitRustMissing
	case count[ResolvesOK] > 0:
		fmt.Fprintf(w, "  exit %d\n", exitRustUnconfirmd)
		return exitRustUnconfirmd
	}
	return exitRustOK
}
