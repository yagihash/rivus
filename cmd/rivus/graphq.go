package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yagihash/rivus/internal/graphdb"
	"github.com/yagihash/rivus/internal/graphq"
)

// parseInterspersed parses the flags that stand among the positional arguments
// ("callers <target> --depth 2" as well as "callers --depth 2 <target>").
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

// adoptStoredFlags sets the analysis flags the person did not give to what the
// file was made with, so that "-C" holds the source against the settings the
// file has (a file made with -tests is not stale for a question without it).
func adoptStoredFlags(fs *flag.FlagSet, stored map[string]string) {
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	for name, v := range stored {
		if name == "patterns" || name == "allow-deps-under" || given[name] {
			continue
		}
		if fs.Lookup(name) != nil {
			fs.Set(name, v)
		}
	}
}

// storedPatterns are the package patterns the file was made with.
func storedPatterns(stored map[string]string) []string {
	p, ok := stored["patterns"]
	if !ok {
		return nil
	}
	return strings.Fields(strings.Trim(p, "[]"))
}

// listFlag is a flag that may be given more than once.
type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

func runGraphQuery(op string, args []string) error {
	fs := flag.NewFlagSet("graph "+op, flag.ExitOnError)
	var o analyzeOpts
	o.register(fs)
	graph := fs.String("graph", "", "the graph file (default: the cache of -C)")
	depth := fs.Int("depth", 0, fmt.Sprintf("callers, callees: how many calls deep (default %d, at most %d)", graphq.DefaultDepth, graphq.MaxDepth))
	limit := fs.Int("limit", 0, fmt.Sprintf("callers, callees: the most edges listed (default %d); path: the most paths (default %d)", graphq.DefaultLimit, graphq.DefaultPaths))
	builtin := fs.Bool("include-builtin", false, "keep the calls of builtins (len, append...), which are left out")
	pretty := fs.Bool("pretty", false, "indent the JSON")
	site := fs.String("site", "", "callers, callees: the target is this site of \"rivus sites\"")
	var avoid, via, onlyImpl listFlag
	fs.Var(&avoid, "avoid", "the filter: never go through this function (a symbol; may be given again)")
	fs.Var(&via, "via", "the filter: only what goes through this function (callers, callees: only the way through it; path: in the order given; may be given again)")
	fs.Var(&onlyImpl, "only-impl", "the filter: a call through an interface reaches only the methods of this type (a name as in the symbols: bytes.Buffer; may be given again)")
	var from, to *string
	var hubs, fromEntry *bool
	if op == "path" {
		from = fs.String("from", "", "the start: a function, file:line:col, file:line, or site:<id>")
		to = fs.String("to", "", "the end")
		fromEntry = fs.Bool("from-entry", false, "the start is the entry points of the program (main and init; for a library, the exported functions), in place of --from")
		hubs = fs.Bool("hubs", false, "search with the calls of many candidates (hubs) at once, in place of only when there is no path without them")
	}
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rivus graph %s [flags] %s\n\n", op, map[bool]string{true: "--from <target> --to <target>", false: "<target>"}[op == "path"])
		fmt.Fprint(os.Stderr, strings.SplitN(graphUsage, "\nCommands:", 2)[0][len("rivus graph - search the call graph saved by \"rivus index\"\n"):])
		fmt.Fprint(os.Stderr, "\nflags:\n")
		fs.PrintDefaults()
	}
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if *graph == "" && !o.dirSet {
		return fmt.Errorf("graph %s needs -graph <file>, or -C <dir> to find the file in the cache", op)
	}
	target := ""
	switch op {
	case "path":
		if len(positional) > 0 || *to == "" || (*from == "") == !*fromEntry {
			return fmt.Errorf("graph path needs --to <target>, and --from <target> or --from-entry (one of them)")
		}
	default:
		if *site != "" {
			if len(positional) > 0 {
				return fmt.Errorf("graph %s: name the target or --site, not both", op)
			}
			target = "site:" + *site
		} else if len(positional) == 1 {
			target = positional[0]
		} else {
			return fmt.Errorf("graph %s needs one target (a symbol, file:line:col, file:line, or --site <id>)", op)
		}
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
	snap := snapshot{SourceHash: info.SourceHash, CreatedAt: info.CreatedAt, RivusVersion: info.RivusVersion}
	var warnings []string
	if !o.dirSet {
		snap.Note = "not checked against the current source: pass -C <dir> to make the key again from it"
	} else {
		adoptStoredFlags(fs, info.Flags)
		if err := verifySnapshot(db, path, &o, storedPatterns(info.Flags), &snap); err != nil {
			return err
		}
		if snap.Fresh != nil && !*snap.Fresh {
			w := "the graph file is stale: it was made from other source or settings, and the answer describes that"
			if len(snap.Reasons) > 0 {
				w += " (" + snap.Reasons[0] + ")"
			}
			warnings = append(warnings, w+"; make it again with \"rivus index\"")
		}
	}

	eng := graphq.New(db)
	opts := graphq.Options{Depth: *depth, Limit: *limit, IncludeBuiltin: *builtin, Avoid: avoid, Via: via, OnlyImpl: onlyImpl}
	if op == "path" {
		opts.Hubs, opts.FromEntry = *hubs, *fromEntry
	}
	var out any
	switch op {
	case "callers":
		r, err := eng.Callers(target, opts)
		if err != nil {
			return err
		}
		r.Snapshot, r.Warnings = snap, append(warnings, r.Warnings...)
		out = r
	case "callees":
		r, err := eng.Callees(target, opts)
		if err != nil {
			return err
		}
		r.Snapshot, r.Warnings = snap, append(warnings, r.Warnings...)
		out = r
	case "path":
		r, err := eng.Path(*from, *to, opts)
		if err != nil {
			return err
		}
		r.Snapshot, r.Warnings = snap, append(warnings, r.Warnings...)
		out = r
	}
	if *pretty {
		return writeJSON(os.Stdout, out)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}
