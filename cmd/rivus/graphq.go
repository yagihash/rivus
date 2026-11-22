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
// file has (a file made with -tests is not stale for a question without it). An explicit -rules wins over
// the record.
func adoptStoredFlags(fs *flag.FlagSet, stored map[string]string) {
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	for name, v := range stored {
		if name == "patterns" || name == "allow-deps-under" || given[name] {
			continue
		}
		if name == "rules" || name == "body-replaced" {
			// the rule files given to index: each is a -rules, and a file that is gone or changed shows as stale
			// (the packages given to -body-replaced, one for each, are the same)
			for _, f := range strings.Split(v, "\n") {
				if f != "" {
					fs.Set(name, f)
				}
			}
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

// graphQueryCmd is "rivus graph callers" and the others that ask the graph (op says which).
type graphQueryCmd struct {
	o                               analyzeOpts
	graph, site, from, to           *string
	depth, limit                    *int
	builtin, pretty, refs           *bool
	hubs, fromEntry, noRefs         *bool
	avoid, avoidCall, via, onlyImpl listFlag
}

func graphQueryFlags(fs *flag.FlagSet, op string) *graphQueryCmd {
	c := &graphQueryCmd{}
	c.o.register(fs)
	c.graph = fs.String("graph", "", "the graph file (default: the cache of -C)")
	c.depth = fs.Int("depth", 0, fmt.Sprintf("callers, callees: how many calls deep (default %d, at most %d)", graphq.DefaultDepth, graphq.MaxDepth))
	c.limit = fs.Int("limit", 0, fmt.Sprintf("callers, callees, impls, dispatchers: the most edges, implementations or call sites listed (default %d); path: the most paths (default %d)", graphq.DefaultLimit, graphq.DefaultPaths))
	c.builtin = fs.Bool("include-builtin", false, "keep the calls of builtins (len, append...), which are left out")
	c.pretty = fs.Bool("pretty", false, "indent the JSON")
	c.site = fs.String("site", "", "callers, callees: the target is this site of \"rivus sites\"")
	fs.Var(&c.avoid, "avoid", "the filter: never go through this function (a symbol; may be given again)")
	fs.Var(&c.avoidCall, "avoid-call", "the filter: never go through this one call site (file:line:col, as \"at\" of an answer says it; may be given again). --avoid cuts a function whole; this cuts one call")
	fs.Var(&c.via, "via", "the filter: only what goes through this function (callers, callees: only the way through it; path: in the order given; may be given again)")
	fs.Var(&c.onlyImpl, "only-impl", "the filter: a call through an interface that has a method of this type among its candidates reaches only those methods; one that has none is not narrowed (a name as in the symbols: bytes.Buffer; may be given again)")
	c.refs = fs.Bool("refs", false, "callers, callees: add the references (a function named as a value: registered, stored, passed on) as edges of kind ref; path: begin the search at the references (the fourth stage)")
	if op == "path" {
		c.from = fs.String("from", "", "the start: a function, file:line:col, file:line, or site:<id>")
		c.to = fs.String("to", "", "the end")
		c.fromEntry = fs.Bool("from-entry", false, "the start is the entry points of the program (main and init; for a library, the exported functions), in place of --from")
		c.noRefs = fs.Bool("no-refs", false, "path: do not search through the references (a function named as a value); the search ends at the calls")
		c.hubs = fs.Bool("hubs", false, "search with the calls of many candidates (hubs) at once, in place of only when there is no path without them")
	}
	sidesOf(fs).header = fmt.Sprintf("usage: rivus graph %s [flags] %s\n\n", op, map[bool]string{true: "--from <target> --to <target>", false: "<target>"}[op == "path"]) +
		strings.SplitN(graphUsage, "\nCommands:", 2)[0][len("rivus graph - search the call graph saved by \"rivus index\"\n"):] + "\nflags:\n"
	usual(fs, "graph", "depth", "limit", "include-builtin", "pretty", "site", "avoid", "avoid-call", "via", "only-impl", "refs")
	if op == "path" {
		usual(fs, "from", "to", "from-entry", "no-refs", "hubs")
	}
	kind(fs, "file", "graph")
	return c
}

func runGraphQuery(op string, args []string) error {
	fs := flag.NewFlagSet("graph "+op, flag.ExitOnError)
	c := graphQueryFlags(fs, op)
	prepareFlags(fs, args)
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	o, graph, depth, limit, builtin, pretty, site, refs := &c.o, c.graph, c.depth, c.limit, c.builtin, c.pretty, c.site, c.refs
	from, to, hubs, fromEntry, noRefs := c.from, c.to, c.hubs, c.fromEntry, c.noRefs
	avoid, avoidCall, via, onlyImpl := c.avoid, c.avoidCall, c.via, c.onlyImpl
	if *graph == "" && !o.dirSet {
		return fmt.Errorf("graph %s needs -graph <file>, or -C <dir> to find the file in the cache", op)
	}
	if (op == "impls" || op == "dispatchers") && *refs {
		return fmt.Errorf("graph %s: --refs is for callers, callees and path", op)
	}
	if *refs && noRefs != nil && *noRefs {
		return fmt.Errorf("graph path: --refs and --no-refs together")
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
	path, err := graphFile(*graph, o)
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
		if err := verifySnapshot(db, path, o, storedPatterns(info.Flags), &snap); err != nil {
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
	opts := graphq.Options{Depth: *depth, Limit: *limit, IncludeBuiltin: *builtin, Avoid: avoid, AvoidCall: avoidCall, Via: via, OnlyImpl: onlyImpl, Refs: *refs}
	if op == "path" {
		opts.Hubs, opts.FromEntry, opts.NoRefs = *hubs, *fromEntry, *noRefs
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
	case "impls":
		r, err := eng.Impls(target, opts)
		if err != nil {
			return err
		}
		r.Snapshot, r.Warnings = snap, append(warnings, r.Warnings...)
		out = r
	case "dispatchers":
		r, err := eng.Dispatchers(target, opts)
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
