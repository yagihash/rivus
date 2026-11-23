package main

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

type site struct {
	id     int
	fn     int
	file   string
	line   int
	kind   string
	ncand  int
	callee string
}

// Graph is what the comparison reads of the SQLite file that `rivus index -graph` writes (read only).
type Graph struct {
	fids    []string
	fid     map[int]string
	fidID   map[string]int
	sites   map[int]*site
	byLine  map[string][]*site // function id, file, line
	byFunc  map[int][]*site
	targets map[int][]string // call site -> fids it has an edge to
}

func lineKey(fn int, file string, line int) string { return fmt.Sprintf("%d|%s|%d", fn, file, line) }

// OpenGraph reads the functions, call sites and edges.
func OpenGraph(path string) (*Graph, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	g := &Graph{fid: map[int]string{}, fidID: map[string]int{}, sites: map[int]*site{}, byLine: map[string][]*site{}, byFunc: map[int][]*site{}, targets: map[int][]string{}}
	rows, err := db.Query(`SELECT id, fid FROM funcs`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int
		var fid string
		if err := rows.Scan(&id, &fid); err != nil {
			rows.Close()
			return nil, err
		}
		g.fid[id], g.fidID[fid] = fid, id
		g.fids = append(g.fids, fid)
	}
	rows.Close()
	rows, err = db.Query(`SELECT c.id, c.func_id, coalesce(fl.path, ''), c.line, c.kind, c.ncand, coalesce(c.callee, '') FROM callsites c LEFT JOIN files fl ON fl.id = c.file_id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		s := &site{}
		if err := rows.Scan(&s.id, &s.fn, &s.file, &s.line, &s.kind, &s.ncand, &s.callee); err != nil {
			rows.Close()
			return nil, err
		}
		g.sites[s.id] = s
		k := lineKey(s.fn, s.file, s.line)
		g.byLine[k] = append(g.byLine[k], s)
		g.byFunc[s.fn] = append(g.byFunc[s.fn], s)
	}
	rows.Close()
	rows, err = db.Query(`SELECT callsite_id, to_func FROM edges`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var cs, to int
		if err := rows.Scan(&cs, &to); err != nil {
			rows.Close()
			return nil, err
		}
		g.targets[cs] = append(g.targets[cs], g.fid[to])
	}
	rows.Close()
	return g, rows.Err()
}

// reaches reports whether the call site has an edge to the function, or to a relay of the graph (m$bound, m$thunk: a
// synthetic function made for a method value) that has one.
func (g *Graph) reaches(s *site, to string) bool {
	want := canon(to)
	for _, t := range g.targets[s.id] {
		if canon(t) == want {
			return true
		}
		if strings.HasSuffix(t, "$bound") || strings.HasSuffix(t, "$thunk") {
			for _, w := range g.byFunc[g.fidID[t]] {
				for _, u := range g.targets[w.id] {
					if canon(u) == want {
						return true
					}
				}
			}
		}
	}
	return false
}

// positionless reports whether the function has a call (not a builtin) that the graph has no line for (the call of the
// iterator by a range over a function is one).
func (g *Graph) positionless(fn int) bool {
	for _, s := range g.byFunc[fn] {
		if s.line == 0 && s.kind != "builtin" {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
