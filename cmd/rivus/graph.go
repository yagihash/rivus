package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/model"
	"github.com/yagihash/rivus/internal/sg"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/vcs"
)

// bodyHashes hashes the normalized body of each function (what "body@1" is made of), by the ID of the
// function. A function whose text the extractor cannot give (a dependency without its syntax) has none.
func bodyHashes(l *loaded) map[string]string {
	out := make(map[string]string, len(l.ir.Funcs))
	for _, f := range l.ir.Funcs {
		if f == nil {
			continue
		}
		if b := l.texts.Body(f.ID); b != "" {
			sum := sha256.Sum256([]byte(b))
			out[string(f.ID)] = hex.EncodeToString(sum[:16])
		}
	}
	return out
}

// graphCachePath is where the graph file of a directory is kept by default: in the user's cache, never
// in the analysed directory (a file there would be the code's to plant).
func graphCachePath(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	cache := os.Getenv("RIVUS_CACHE_DIR")
	if cache == "" {
		var err error
		if cache, err = os.UserCacheDir(); err != nil {
			return "", fmt.Errorf("no user cache directory (name the file with -graph, or set RIVUS_CACHE_DIR): %w", err)
		}
	}
	sum := sha256.Sum256([]byte(abs))
	if os.Getenv("RIVUS_CACHE_DIR") != "" {
		return filepath.Join(cache, hex.EncodeToString(sum[:8]), "graph.db"), nil
	}
	return filepath.Join(cache, "rivus", hex.EncodeToString(sum[:8]), "graph.db"), nil
}

// graphFile is the file the flags name: -graph, or the cache of -C.
func graphFile(explicit string, o *analyzeOpts) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	return graphCachePath(o.dir)
}

// graphIndexer makes the hook of analyze that writes the structure layer.
func (o *analyzeOpts) graphIndexer(path string, patterns []string, since time.Time, report func(*graphdb.Counts)) func(*loaded, *model.Rules, *sites.Result, *sg.Program, map[string]string) error {
	return func(l *loaded, rules *model.Rules, st *sites.Result, g *sg.Program, bodies map[string]string) error {
		dir, err := filepath.Abs(o.dir)
		if err != nil {
			return err
		}
		k := &keyCtx{o: o, dir: dir, graphPath: path, l: l, since: since, patterns: patterns}
		t := time.Now()
		key, err := k.compute()
		if err != nil {
			return err
		}
		if o.verbose {
			fmt.Fprintf(os.Stderr, "rivus: source hash %v (%d files hashed)\n", time.Since(t).Round(time.Millisecond), len(key.Files))
			t = time.Now()
		}
		meta := graphdb.Meta{
			RivusVersion: rivusVersionFn(),
			CreatedAt:    time.Now(),
			SourceHash:   key.Hash,
			Dir:          dir,
			Languages:    graphLangs(g),
			Flags:        o.flagValues(patterns),
			KeyConfig:    key.Config,
			Manifest:     key.Manifest,
			Roots:        key.Roots,
		}
		meta.Unread = &graphdb.Unread{}
		for _, lang := range meta.Languages {
			switch lang {
			case "go":
				meta.Unread.Counted = append(meta.Unread.Counted, "go:failed_packages")
			case "dart":
				meta.Unread.Counted = append(meta.Unread.Counted, "dart:failed_files") // the extractor reports the type and the syntax errors of a file
			case "rust":
				meta.Unread.Uncounted = append(meta.Unread.Uncounted, "rust:failed_crates") // nor the Rust one
			default:
				meta.Unread.Uncounted = append(meta.Unread.Uncounted, lang+":failed_units")
			}
		}
		for _, sl := range o.skipped {
			meta.Unread.SkippedLanguages = append(meta.Unread.SkippedLanguages, graphdb.SkippedLanguage{Lang: sl.Lang, Why: sl.Why})
		}
		if ts := l.testsSkipped(); ts != nil {
			meta.Unread.SkippedDirs = ts.SkippedDirs
			meta.Unread.OutsideBuildFiles = ts.OutsideBuildFiles
		}
		failed := map[string]bool{} // a package with several errors is one
		for _, e := range l.loadErrors() {
			if failed[e.Package] {
				continue
			}
			failed[e.Package] = true
			meta.Unread.FailedPackages++
			if len(meta.Unread.FailedExamples) < 5 {
				meta.Unread.FailedExamples = append(meta.Unread.FailedExamples, e.Package)
			}
		}
		if t := vcs.Detect(dir); t != nil && o.irFile == "" {
			meta.Revision = t.Revision
		}
		c, err := graphdb.Write(path, &graphdb.Input{Graph: g, Sites: st.Sites, BodyHash: bodies, Files: key.Files, Meta: meta})
		if err != nil {
			return err
		}
		if o.verbose {
			fmt.Fprintf(os.Stderr, "rivus: write %v\n", time.Since(t).Round(time.Millisecond))
		}
		report(c)
		return nil
	}
}

func graphLangs(g *sg.Program) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range g.Funcs {
		if l := graphdb.LangOf(f.ID); !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}

// flagValues are the analysis flags as the file records them (for a reader).
func (o *analyzeOpts) flagValues(patterns []string) map[string]string {
	lang := o.lang
	if (lang == "" || lang == "auto") && o.langsDone && o.langsErr == nil && (has(o.langsOut, langRust) || o.skippedLang(langRust)) {
		// A Rust project in the directory made the answer of a question (or of -yes): the file holds what the answer
		// was, so that a later question about it (graph -C) neither asks again nor holds the file against another answer.
		lang = strings.Join(o.langsOut, ",")
	}
	m := map[string]string{
		"lang":                lang,
		"tests":               fmt.Sprint(o.tests),
		"no-deps":             fmt.Sprint(o.noDeps),
		"k":                   fmt.Sprint(o.k),
		"numeric-taint":       fmt.Sprint(o.numeric),
		"max-implementations": fmt.Sprint(o.maxImpl),
		"no-builtin-rules":    fmt.Sprint(o.noBuiltin),
		"no-project-rules":    fmt.Sprint(o.noProject),
		"deterministic-only":  fmt.Sprint(o.deterministic),
	}
	if o.resolveFuncValues {
		m["resolve-func-values"] = "true"
	}
	if len(patterns) > 0 {
		m["patterns"] = fmt.Sprint(patterns)
	}
	if len(o.allowDepsUnder) > 0 {
		m["allow-deps-under"] = o.allowDepsUnder.String()
	}
	return m
}

// graphInfo is the JSON that "rivus graph info" prints.
type graphInfo struct {
	SchemaVersion string        `json:"schema_version"`
	Kind          string        `json:"kind"`
	Graph         *graphdb.Info `json:"graph"`
	Snapshot      snapshot      `json:"snapshot"`
	Layers        layerStatus   `json:"layers"`
}

// snapshot says which code the file was made from, and whether that is the code there is now.
type snapshot struct {
	SourceHash   string `json:"source_hash"`
	CreatedAt    string `json:"created_at"`
	RivusVersion string `json:"rivus_version"`
	// Verified is true when the key was made again from the current source (-C) and held against the file's.
	Verified bool  `json:"verified"`
	Fresh    *bool `json:"fresh,omitempty"`
	// Reasons are the differences, when the file is stale.
	Reasons []string `json:"stale_reasons,omitempty"`
	Note    string   `json:"note,omitempty"`
}

type layerStatus struct {
	Present []string `json:"present"`
	// Taint says what a reader may not conclude: the taint layer is not in the file.
	Taint string `json:"taint"`
}

const maxReasons = 20

func runGraph(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(graphUsage)
		if len(args) == 0 {
			os.Exit(2)
		}
		return nil
	}
	switch args[0] {
	case "info":
		return runGraphInfo(args[1:])
	case "callers", "callees", "path", "impls", "dispatchers":
		return runGraphQuery(args[0], args[1:])
	}
	fmt.Fprintf(os.Stderr, "rivus: unknown graph command %q\n\n%s", args[0], graphUsage)
	os.Exit(2)
	return nil
}

const graphUsage = `rivus graph - search the call graph saved by "rivus index"

It speaks of calls (who calls whom), not of the flow of data (what reaches what):
for the flow, use "rivus analyze" and "rivus query".

Usage:
  rivus graph info    [-graph <file> | -C <dir>]
  rivus graph callers [-graph <file> | -C <dir>] [--depth N] [--limit K] [--include-builtin] [--refs] [filters] <target>
  rivus graph callees [-graph <file> | -C <dir>] [--depth N] [--limit K] [--include-builtin] [--refs] [filters] <target>
  rivus graph impls   [-graph <file> | -C <dir>] [--limit K] [--avoid <symbol>] [--avoid-call <file:line:col>] [--only-impl <type>] <interface method>
  rivus graph dispatchers [-graph <file> | -C <dir>] [--limit K] [--avoid <symbol>] [--avoid-call <file:line:col>] [--only-impl <type>] <implementation>
  rivus graph path    [-graph <file> | -C <dir>] (--from <target> | --from-entry) --to <target> [--limit K] [--hubs | --refs | --no-refs] [filters]

Filters (they may be given again, and the answer's "filters" says each one):
  --avoid <symbol>      never go through this function (all of its calls, and all calls to it)
  --avoid-call <file:line:col>
                        never go through this one call site (the function that holds it, and its other calls, stay):
                        to cut one dubious call of a path through a hub, and ask again
  --via <symbol>        only what goes through this function (path: in the order given)
  --only-impl <type>    a call through an interface that has a method of this type among its candidates reaches only
                        those methods; a call through an interface that has none is not narrowed (the answer says how many)

Commands:
  info     what the file holds, when and from which source it was made, and (with -C) whether the source is still the same
  callers  the functions that call the target (backwards)
  callees  the functions the target calls (forwards)
           (--refs adds the references: edges of kind ref, the functions a function holds as values, or that hold it)
  path     up to K ways from one target to the other, shortest first. It looks at the static calls, then (if
           there is none) at the dynamic ones, then (if there is still none) at the calls of more than 20
           candidates, "hubs", then (if there is still none) through references: a function named as a value
           (registered with a router, stored, passed on), which is not a call ("searched" says how far it went;
           --refs starts at the references, --no-refs leaves them out). --from-entry starts from the entry points.
           A path with a reference says "relation": "calls+refs" and "certainty": "reference".

  impls    the functions that the calls through an interface method (io.Writer.Write, (error).Error) may reach:
           the union of the candidates of those call sites in the graph, not every type that implements the interface
  dispatchers  the calls through an interface (or a function value) that list the target as a candidate, each with
           the interface method it goes through ("callee"): start from a concrete implementation

A target is one of (the kind is told from its form):
  os/exec.Command, (*pkg.T).Method   a function (a Dart or a Rust name too: pkg::f). The front may be left out
                                     (GetUserByID, T.Method) when one function in the whole graph ends so;
                                     if several do, the answer lists them (the code itself first) to choose from
  file:line:col                      the call that begins there
  file:line                          the calls on that line (if several: the answer lists them to choose from)
  site:<id>, or --site <id>          a site of "rivus sites"
A file is named as the graph has it (relative to the analysed directory), or by an absolute path.

What a target means, for a function F and a call C (a call is a place in a function, where it calls another):
  callers F   who calls F                       callees F   what F calls
  callers C   who calls the function that holds C   callees C   what C calls (the functions it may reach)
  path --from C   starts with the call C         path --to C   ends in the function that holds C, with the call C

The file is read only from -graph, or from the cache of -C (never from a file in the analysed directory).
With -C the source is held against the file: "snapshot.fresh" says whether it is the same, and a warning
is printed when it is not (the answer is still given, of the old source). Without -C nothing says so.
The calls of builtins (len, append...) are left out of the answers; --include-builtin keeps them. The count
that was left out is in "omitted". A call through an interface or a function value is "dynamic": its edge
has "kind" and "candidates" (how many functions the call may reach).
`

func runGraphInfo(args []string) error {
	fs := flag.NewFlagSet("graph info", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	graph := fs.String("graph", "", "the graph file (default: the cache of -C)")
	usual(fs, "graph")
	prepareFlags(fs, args)
	fs.Parse(args)
	if *graph == "" && !o.dirSet {
		return fmt.Errorf("graph info needs -graph <file>, or -C <dir> to find the file in the cache")
	}
	path, err := graphFile(*graph, &o)
	if err != nil {
		return err
	}
	db, err := graphdb.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	info, err := db.Info()
	if err != nil {
		return err
	}
	out := graphInfo{SchemaVersion: graphdb.SchemaName, Kind: "info", Graph: info}
	out.Snapshot = snapshot{SourceHash: info.SourceHash, CreatedAt: info.CreatedAt, RivusVersion: info.RivusVersion}
	for _, l := range info.Layers {
		out.Layers.Present = append(out.Layers.Present, l.Name)
	}
	out.Layers.Taint = "absent: the file has no findings and no uncertainties, which says nothing of whether the code has any"
	if !o.dirSet {
		out.Snapshot.Note = "not checked against the current source: pass -C <dir> to make the key again from it"
		return writeJSON(os.Stdout, out)
	}
	if err := verifySnapshot(db, path, &o, fs.Args(), &out.Snapshot); err != nil {
		return err
	}
	return writeJSON(os.Stdout, out)
}

// verifySnapshot makes the key again from the current source of -C and holds it against the file's, and
// fills what the snapshot says: whether the file is fresh, and the differences when it is not.
func verifySnapshot(db *graphdb.DB, path string, o *analyzeOpts, patterns []string, snap *snapshot) error {
	stored, err := db.KeyParts()
	if err != nil {
		return err
	}
	dir, err := filepath.Abs(o.dir)
	if err != nil {
		return err
	}
	k := &keyCtx{o: o, dir: dir, graphPath: path, stored: stored, patterns: patterns}
	now, err := k.compute()
	if err != nil {
		return err
	}
	reasons := graphdb.Differences(stored, &now.Parts, maxReasons)
	fresh := len(reasons) == 0 && now.Hash == stored.Hash
	if len(reasons) == 0 && !fresh {
		// every part is the same and the hash is not: the file is not trusted
		reasons = append(reasons, "the source hash does not match, though no part differs (the file is not made by this version of the key)")
	}
	snap.Verified = true
	snap.Fresh = &fresh
	snap.Reasons = reasons
	if !fresh {
		snap.Note = "the file was made from other source or other settings: make it again with \"rivus index\" (with the same flags)"
	}
	return nil
}
