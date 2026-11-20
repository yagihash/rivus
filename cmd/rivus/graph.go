package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	m := map[string]string{
		"lang":                o.lang,
		"tests":               fmt.Sprint(o.tests),
		"no-deps":             fmt.Sprint(o.noDeps),
		"k":                   fmt.Sprint(o.k),
		"numeric-taint":       fmt.Sprint(o.numeric),
		"max-implementations": fmt.Sprint(o.maxImpl),
		"no-builtin-rules":    fmt.Sprint(o.noBuiltin),
		"no-project-rules":    fmt.Sprint(o.noProject),
		"deterministic-only":  fmt.Sprint(o.deterministic),
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
	}
	fmt.Fprintf(os.Stderr, "rivus: unknown graph command %q\n\n%s", args[0], graphUsage)
	os.Exit(2)
	return nil
}

const graphUsage = `rivus graph - search the call graph saved by "rivus index"

It speaks of calls (who calls whom), not of the flow of data (what reaches what):
for the flow, use "rivus analyze" and "rivus query".

Usage:
  rivus graph info -graph <file> [-C <dir>]

Commands:
  info   what the file holds, when and from which source it was made, and (with -C) whether the source is still the same

The file is read only from -graph, or from the cache of -C (never from a file in the analysed directory).
`

func runGraphInfo(args []string) error {
	fs := flag.NewFlagSet("graph info", flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	graph := fs.String("graph", "", "the graph file (default: the cache of -C)")
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
	stored, err := db.KeyParts()
	if err != nil {
		return err
	}
	dir, err := filepath.Abs(o.dir)
	if err != nil {
		return err
	}
	patterns := fs.Args()
	k := &keyCtx{o: &o, dir: dir, graphPath: path, stored: stored, patterns: patterns}
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
	out.Snapshot.Verified = true
	out.Snapshot.Fresh = &fresh
	out.Snapshot.Reasons = reasons
	if !fresh {
		out.Snapshot.Note = "the file was made from other source or other settings: make it again with \"rivus index\" (with the same flags)"
	}
	return writeJSON(os.Stdout, out)
}
