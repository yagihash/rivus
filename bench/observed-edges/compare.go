package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RawEdge is what rvobs writes: the names of the runtime.
type RawEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Count int    `json:"count"`
}

// Edge is an observed call in the form that the comparison reads, the same for every language: the function that
// called, at which line of which file (relative to the directory the graph was made from), and the function that was
// called, both as the fids of the graph ("" when the name was not found: From and To keep the names of the language).
type Edge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	FromFID string `json:"from_fid"`
	ToFID   string `json:"to_fid"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Count   int    `json:"count"`
	// Outside is why the edge is not compared ("" when it is): harness, go_stmt, other_module.
	Outside string `json:"outside,omitempty"`
}

// Observed is the file of observed calls (docs/DESIGN-observed-calls.md).
type Observed struct {
	Lang  string `json:"lang"`
	Edges []Edge `json:"edges"`
}

// Normalize turns the names of the runtime into fids. root is the directory the recorded paths are under (the copy that
// ran); module is the module path; harness are the package paths under the module that are not the code that is read
// (the harness of the cases, the commands).
func Normalize(raw []RawEdge, g *Graph, root, module string, harness []string) Observed {
	idx := NewFuncIndex(g.fids)
	isHarness := func(n string) bool {
		for _, h := range harness {
			if strings.HasPrefix(n, module+"/"+h+".") || strings.HasPrefix(n, module+"/"+h+"/") {
				return true
			}
		}
		return false
	}
	in := func(n string) bool { return strings.HasPrefix(n, module+"/") }
	merged := map[string]*Edge{}
	var order []string
	for _, r := range raw {
		file := r.File
		if rel, err := filepath.Rel(root, file); err == nil && !strings.HasPrefix(rel, "..") {
			file = filepath.ToSlash(rel)
		}
		e := Edge{From: r.From, To: r.To, File: file, Line: r.Line, Count: r.Count}
		switch {
		case r.From == "runtime.goexit":
			e.Outside = "go_stmt"
		case !in(r.From) || !in(r.To):
			e.Outside = "other_module"
		case isHarness(r.From) || isHarness(r.To):
			e.Outside = "harness"
		default:
			e.FromFID, e.ToFID = idx.Lookup(r.From), idx.Lookup(r.To)
		}
		k := fmt.Sprintf("%s|%s|%s|%d", e.From, e.To, e.File, e.Line)
		if m, ok := merged[k]; ok {
			m.Count += e.Count
			continue
		}
		merged[k] = &e
		order = append(order, k)
	}
	sort.Strings(order)
	o := Observed{Lang: "go"}
	for _, k := range order {
		o.Edges = append(o.Edges, *merged[k])
	}
	return o
}

// Classes of an observed edge.
const (
	ClassOK         = "ok"          // the graph has the edge
	ClassNoEdge     = "no_edge"     // the call site is there and has no edge to the function that was called: a missed edge
	ClassNoCallsite = "no_callsite" // the graph has no call in that function at that line
	ClassIDUnmapped = "id_unmapped" // the name does not stand for a function of the graph (a mistake of the observation, or of the rules of names)
	ClassOutside    = "outside"     // not compared (the reason is in Outside)
)

// Result is the comparison.
type Result struct {
	Edges []Edge
	Class []string
	Note  []string // outside: the reason; ok: how
	Over  []OverRow
}

// OverRow is a call of a function value or an interface method that has two candidates or more: how many of them
// were seen called. Information only (an edge that was not seen is not a wrong one: the cases run each once).
type OverRow struct {
	Func     string
	File     string
	Line     int
	Cands    int
	Observed int
}

// Compare classifies every observed edge against the graph.
func Compare(o Observed, g *Graph) Result {
	r := Result{Edges: o.Edges, Class: make([]string, len(o.Edges)), Note: make([]string, len(o.Edges))}
	seen := map[int]map[string]bool{}
	for i, e := range o.Edges {
		switch {
		case e.Outside != "":
			r.Class[i], r.Note[i] = ClassOutside, e.Outside
			continue
		case e.FromFID == "" || e.ToFID == "":
			r.Class[i] = ClassIDUnmapped
			continue
		}
		fn := g.fidID[e.FromFID]
		sites := g.byLine[lineKey(fn, e.File, e.Line)]
		if len(sites) == 0 {
			if g.positionless(fn) {
				r.Class[i], r.Note[i] = ClassOutside, "no_position" // a call that the graph has no line for (range over a function)
			} else {
				r.Class[i] = ClassNoCallsite
			}
			continue
		}
		r.Class[i] = ClassNoEdge
		for _, s := range sites {
			if g.reaches(s, e.ToFID) {
				r.Class[i] = ClassOK
				if seen[s.id] == nil {
					seen[s.id] = map[string]bool{}
				}
				seen[s.id][canon(e.ToFID)] = true
				break
			}
		}
	}
	for id, callees := range seen {
		s := g.sites[id]
		if s.ncand >= 2 {
			r.Over = append(r.Over, OverRow{g.fid[s.fn], s.file, s.line, s.ncand, len(callees)})
		}
	}
	sort.Slice(r.Over, func(i, j int) bool {
		a, b := r.Over[i], r.Over[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return r
}

// Print writes the tables.
func (r Result) Print(w io.Writer, verbose bool) {
	count := map[string]int{}
	outside := map[string]int{}
	for i, c := range r.Class {
		count[c]++
		if c == ClassOutside {
			outside[r.Note[i]]++
		}
	}
	fmt.Fprintf(w, "observed edges (distinct caller line + callee): %d\n", len(r.Edges))
	for _, c := range []string{ClassOK, ClassNoEdge, ClassNoCallsite, ClassIDUnmapped, ClassOutside} {
		fmt.Fprintf(w, "  %-12s %d\n", c, count[c])
	}
	for _, k := range sortedKeys(outside) {
		fmt.Fprintf(w, "    outside: %-14s %d\n", k, outside[k])
	}
	for _, c := range []string{ClassNoEdge, ClassNoCallsite, ClassIDUnmapped} {
		for i, e := range r.Edges {
			if r.Class[i] == c {
				if c == ClassIDUnmapped {
					fmt.Fprintf(w, "%s: %s -> %s (%s:%d) [from=%q to=%q]\n", c, e.From, e.To, e.File, e.Line, e.FromFID, e.ToFID)
				} else {
					fmt.Fprintf(w, "%s: %s -> %s (%s:%d)\n", c, e.FromFID, e.ToFID, e.File, e.Line)
				}
			}
		}
	}
	fmt.Fprintf(w, "over-approximation (information only; call sites with ncand >= 2 that were seen at all): %d\n", len(r.Over))
	if verbose {
		for _, o := range r.Over {
			fmt.Fprintf(w, "  %s:%d %s: %d candidates, %d seen called\n", o.File, o.Line, o.Func, o.Cands, o.Observed)
		}
	}
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
