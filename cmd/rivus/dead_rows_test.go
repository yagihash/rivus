package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/frontend/goir"
	"github.com/yagihash/rivus/internal/frontend/gossa"
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/model/builtin"
)

// A row of the built-in model that no code ever calls is a silent miss: the spelling of its key is not the one the
// extractor writes, or the API is not what the row thinks it is, and nothing says so. This test holds every row of
// the built-in model that names a function to the IR of the zoo (bench/zoo-*) and of the curated targets
// (bench/*-curated): a row has to be hit by a call (or, for an entry point, by a function) of one of them, or be in
// internal/model/builtin/<model file>.dead-allow (next to the file of the row) with its reason. The IR of real projects is not used: a real project does not use
// every API, and a row that it does not use is not wrong.
//
// A new row is a test that fails until a cell of the zoo calls it (docs/MODELING.md, "A row has to be hit").
// RIVUS_DEAD_ROWS_PRINT=1 prints the rows that are not hit, in the form of the allow file.

const deadAllowDir = "../../internal/model/builtin"

type deadRow struct{ lang, symbol, file string }

// builtinCallRows are the rows whose key is the name of a function that a call or a definition shows.
func builtinCallRows(t *testing.T) []deadRow {
	t.Helper()
	sets, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	var rows []deadRow
	seen := map[string]bool{}
	for _, rs := range sets {
		file := strings.TrimPrefix(rs.Origin, "builtin:")
		var lang string
		switch {
		case strings.HasPrefix(file, "go-"):
			lang = "go"
		case strings.HasPrefix(file, "dart-"):
			lang = "dart"
		case strings.HasPrefix(file, "rust-"):
			lang = "rust"
		default:
			t.Fatalf("the language of %s is not known", file)
		}
		add := func(sym string) {
			if sym == "" || seen[lang+" "+sym] {
				return
			}
			seen[lang+" "+sym] = true
			rows = append(rows, deadRow{lang, sym, file})
		}
		for _, s := range rs.Sources {
			if s.Kind == model.KindCallResult || (s.Kind == model.KindParam && s.Symbol != "") {
				add(s.Symbol)
			}
		}
		for _, s := range rs.Sinks {
			add(s.Symbol)
		}
		for _, s := range rs.Sanitizers {
			add(s.Symbol)
		}
		for _, p := range rs.Propagators {
			add(p.Symbol)
		}
		for _, c := range rs.ConfigReads {
			add(c.Symbol)
		}
	}
	return rows
}

// irSymbols adds the names of what the IR calls and defines to the set.
func irSymbols(set map[string]bool, p *ir.Program) {
	for _, f := range p.Funcs {
		if f.Symbol != "" {
			set[f.Symbol] = true
		}
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpCall && in.Callee != nil && in.Callee.Symbol != "" {
					set[in.Callee.Symbol] = true
				}
			}
		}
	}
}

func deadReadIR(t *testing.T, stream []byte) *ir.Program {
	t.Helper()
	p, err := ir.Read(bufio.NewReaderSize(bytes.NewReader(stream), 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func goZooSymbols(t *testing.T, set map[string]bool) {
	t.Helper()
	// the harnesses of the zoo (the pure rows, the rows of calls) are behind a build tag
	t.Setenv("GOFLAGS", "-tags=zooexec")
	for _, d := range []string{"bench/zoo-go", "bench/go-curated"} {
		dir, err := filepath.Abs(filepath.Join("..", "..", d))
		if err != nil {
			t.Fatal(err)
		}
		prog, err := gossa.Load(dir, []string{"./..."}, gossa.Options{NaiveForm: true})
		if err != nil {
			t.Fatal(err)
		}
		out, err := goir.Extract(prog, goir.Options{})
		if err != nil {
			t.Fatal(err)
		}
		irSymbols(set, out)
	}
}

func dartZooSymbols(t *testing.T, set map[string]bool) {
	t.Helper()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	ext, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	for _, d := range []string{"bench/zoo-dart/cells", "bench/zoo-dart/pure", "bench/dart-curated"} {
		dir, err := filepath.Abs(filepath.Join("..", "..", d))
		if err != nil {
			t.Fatal(err)
		}
		zooDartPubGet(t, dir)
		cmd := exec.Command(words[0], append(append([]string{}, words[1:]...), "run", "bin/rivus_dart.dart", dir)...)
		cmd.Dir = ext
		var stderr strings.Builder
		cmd.Stderr = &stderr
		stream, err := cmd.Output()
		if err != nil {
			t.Fatalf("the Dart extractor on %s: %v\n%s", d, err, stderr.String())
		}
		irSymbols(set, deadReadIR(t, stream))
	}
}

func rustZooSymbols(t *testing.T, set map[string]bool) {
	t.Helper()
	if os.Getenv("RIVUS_RUST") == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	// our own code: read on this machine, as the zoo tests do (nothing of a third party is read here)
	zoo := zooRustCopy(t, zooRustRoot(t))
	irSymbols(set, deadReadIR(t, zooRustStream(t, zoo)))
	cur, err := filepath.Abs("../../bench/rust-curated")
	if err != nil {
		t.Fatal(err)
	}
	irSymbols(set, deadReadIR(t, zooRustStream(t, cur)))
}

// readDeadAllow reads internal/model/builtin/<model file>.dead-allow (next to <model file>.yaml): "symbol | reason", and "#"
// for a comment. The language is the one of the file name (go-, dart-, rust-). It returns "lang symbol" -> the file of the
// row (the .yaml name) and the reason.
func readDeadAllow(t *testing.T) (reasons, files map[string]string) {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(deadAllowDir, "*.dead-allow"))
	if err != nil {
		t.Fatal(err)
	}
	reasons, files = map[string]string{}, map[string]string{}
	for _, path := range names {
		base := filepath.Base(path)
		yaml := strings.TrimSuffix(base, ".dead-allow") + ".yaml"
		if _, err := os.Stat(filepath.Join(deadAllowDir, yaml)); err != nil {
			t.Fatalf("%s: there is no %s next to it", base, yaml)
		}
		lang, _, _ := strings.Cut(base, "-")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for i, l := range strings.Split(string(b), "\n") {
			l = strings.TrimSpace(l)
			if l == "" || strings.HasPrefix(l, "#") {
				continue
			}
			sym, reason, ok := strings.Cut(l, " | ")
			if !ok || strings.TrimSpace(reason) == "" {
				t.Fatalf("%s:%d: no reason (the form is: symbol | reason)", base, i+1)
			}
			key := lang + " " + strings.TrimSpace(sym)
			if _, dup := reasons[key]; dup {
				t.Fatalf("%s:%d: %s is listed twice (in this file or in another)", base, i+1, key)
			}
			reasons[key] = strings.TrimSpace(reason)
			files[key] = yaml
		}
	}
	return reasons, files
}

func TestBuiltinRowsAreHitByTheZoo(t *testing.T) {
	rows := builtinCallRows(t)
	allow, allowFile := readDeadAllow(t)
	for _, lang := range []string{"go", "dart", "rust"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			set := map[string]bool{}
			switch lang {
			case "go":
				goZooSymbols(t, set)
			case "dart":
				dartZooSymbols(t, set)
			case "rust":
				rustZooSymbols(t, set)
			}
			var dead []string
			isRow := map[string]bool{}
			for _, r := range rows {
				if r.lang != lang {
					continue
				}
				key := lang + " " + r.symbol
				isRow[key] = true
				hit := set[r.symbol]
				_, allowed := allow[key]
				if allowed && allowFile[key] != r.file {
					t.Errorf("%s is a row of %s and is allowed in the file of %s: the allowance is in the .dead-allow of the file of the row", key, r.file, allowFile[key])
				}
				if !hit {
					dead = append(dead, key+" | ")
				}
				switch {
				case hit && allowed:
					// A row that the zoo hits now and that is still allowed is not a failure of a model PR: what hits it can be the code of a
					// dependency of a cell (a package of pub that a cell uses), and a PR that adds a package must not have to edit the
					// .dead-allow of five models. The clean-up is a run of its own: RIVUS_DEAD_ALLOW_STRICT=1 fails on it.
					msg := fmt.Sprintf("%s is hit by the zoo now: take it out of internal/model/builtin/%s", key, strings.TrimSuffix(r.file, ".yaml")+".dead-allow")
					if os.Getenv("RIVUS_DEAD_ALLOW_STRICT") == "1" {
						t.Error(msg)
					} else {
						t.Log(msg)
					}
				case !hit && !allowed:
					t.Errorf("%s (%s): no call in the zoo or the curated targets hits this row. Either the key is not the name the extractor writes "+
						"(read the IR: `rivus debug extract` / the extractor of the language), or a cell of bench/zoo-%s has to call it. "+
						"Only an API that a run cannot check (it needs a server, a device) goes in internal/model/builtin/%s, with the reason (docs/MODELING.md)", key, r.file, lang, strings.TrimSuffix(r.file, ".yaml")+".dead-allow")
				}
			}
			for key := range allow {
				if strings.HasPrefix(key, lang+" ") && !isRow[key] {
					t.Errorf("%s: %s is not a row of the built-in model: take it out", strings.TrimSuffix(allowFile[key], ".yaml")+".dead-allow", key)
				}
			}
			if os.Getenv("RIVUS_DEAD_ROWS_PRINT") == "1" {
				sort.Strings(dead)
				t.Logf("%d rows of %s are not hit (the allow file lists the ones that may stay):\n%s", len(dead), lang, strings.Join(dead, "\n"))
			}
		})
	}
}
