package main

import (
	"fmt"
	"sort"
	"strings"
)

var langs = []string{"go", "dart", "rust"}
var langName = map[string]string{"go": "Go", "dart": "Dart", "rust": "Rust"}

// tailOf is the spelling of a symbol without the path that varies (a re-export, a library URI, a generic
// instance): what two spellings of one API have in common.
func tailOf(lang, sym string) string {
	switch lang {
	case "go":
		return strings.ReplaceAll(sym, "*", "") // a pointer or a value receiver is the one difference
	case "dart":
		if i := strings.LastIndex(sym, "#"); i >= 0 {
			return sym[i+1:]
		}
		return sym
	default:
		parts := strings.Split(sym, "::")
		if len(parts) > 2 {
			parts = parts[len(parts)-2:]
		}
		return strings.Join(parts, "::")
	}
}

// DeadStatus of a row no IR hits.
const (
	DeadMissing = "api_missing" // the symbol is not in the types of the language (Go: go/types knows)
	DeadSuspect = "spelling"    // another spelling of the same API is in an IR
	DeadMember  = "member"      // the same member name on another type of the same library (an inherited method, a Deref, a trait)
	DeadUnused  = "unused"      // nothing like it in any IR
	DeadNoCall  = "not_a_call"  // the key is a type, a field, a variable, a marker: no call to hit
)

type RowResult struct {
	Row
	HitAll    bool
	HitReal   bool
	Sites     int
	Dead      string
	Spellings []string
}

type DeadReport struct {
	Lang    string
	Results []RowResult
}

// CheckRows matches the rows of one language with the corpora of that language.
func CheckRows(lang string, rows []Row, corpora []*Corpus, gu *GoUniverse) []RowResult {
	var kept []*Corpus
	for _, c := range corpora {
		if !c.Supp {
			kept = append(kept, c)
		}
	}
	corpora = kept
	tails := map[string][]string{}
	for _, c := range corpora {
		for s := range c.Calls {
			t := tailOf(lang, s)
			tails[t] = appendUniq(tails[t], s)
		}
		for s := range c.FuncSyms {
			t := tailOf(lang, s)
			tails[t] = appendUniq(tails[t], s)
		}
	}
	members := map[string][]string{}
	for _, c := range corpora {
		for s := range c.Calls {
			if k := memberKey(lang, s); k != "" {
				members[k] = appendUniq(members[k], s)
			}
		}
	}
	var out []RowResult
	for _, r := range rows {
		if r.Lang != lang {
			continue
		}
		rr := RowResult{Row: r}
		if !r.Call {
			rr.Dead = DeadNoCall
			out = append(out, rr)
			continue
		}
		for _, c := range corpora {
			hit := c.FuncSyms[r.Symbol]
			if cc := c.Calls[r.Symbol]; cc != nil {
				hit = true
				rr.Sites += cc.Lib + cc.Own
			}
			if hit {
				rr.HitAll = true
				if c.Real {
					rr.HitReal = true
				}
			}
		}
		if !rr.HitAll {
			switch {
			case lang == "go" && gu != nil && isMissing(gu, r.Symbol):
				rr.Dead = DeadMissing
			default:
				for _, s := range tails[tailOf(lang, r.Symbol)] {
					if s != r.Symbol {
						rr.Spellings = append(rr.Spellings, s)
					}
				}
				sort.Strings(rr.Spellings)
				if len(rr.Spellings) > 0 {
					rr.Dead = DeadSuspect
				} else if k := memberKey(lang, r.Symbol); k != "" && len(members[k]) > 0 {
					rr.Dead = DeadMember
					rr.Spellings = append(rr.Spellings, members[k]...)
					sort.Strings(rr.Spellings)
				} else {
					rr.Dead = DeadUnused
				}
			}
		}
		out = append(out, rr)
	}
	return out
}

var commonMembers = map[string]bool{"new": true, "from": true, "into": true, "get": true, "len": true, "as_ref": true,
	"clone": true, "default": true, "fmt": true, "eq": true, "next": true, "drop": true, "deref": true, "toString": true,
	"hashCode": true, "map": true, "iterator": true, "length": true, "add": true, "contains": true, "remove": true}

// memberKey is the library (Dart: before "#"; Rust: the crate) and the member name of a symbol of a language that
// has inheritance or Deref: "" when the language is Go or the name is too common to say anything.
func memberKey(lang, sym string) string {
	switch lang {
	case "dart":
		i := strings.LastIndex(sym, "#")
		if i < 0 {
			return ""
		}
		rest := sym[i+1:]
		j := strings.LastIndex(rest, ".")
		if j < 0 || commonMembers[rest[j+1:]] {
			return ""
		}
		return sym[:i] + "#" + rest[j+1:]
	case "rust":
		parts := strings.Split(sym, "::")
		if len(parts) < 3 || commonMembers[parts[len(parts)-1]] {
			return ""
		}
		return parts[0] + "::" + parts[len(parts)-1]
	}
	return ""
}

func isMissing(gu *GoUniverse, sym string) bool {
	_, st := gu.Resolve(sym)
	return st == "missing"
}

func appendUniq(xs []string, s string) []string {
	if len(xs) >= 6 {
		return xs
	}
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}

type DeadCounts struct {
	Rows, Calls, NoCall                    int
	Hit, HitReal                           int
	Dead, Missing, Suspect, Member, Unused int
	ByShape                                map[string][4]int // shape -> rows, hit, suspect+missing, unused
}

func CountDead(rs []RowResult) DeadCounts {
	d := DeadCounts{ByShape: map[string][4]int{}}
	for _, r := range rs {
		d.Rows++
		if !r.Call {
			d.NoCall++
			continue
		}
		d.Calls++
		a := d.ByShape[r.Shape]
		a[0]++
		if r.HitAll {
			d.Hit++
			a[1]++
			if r.HitReal {
				d.HitReal++
			}
		} else {
			d.Dead++
			switch r.Dead {
			case DeadMissing:
				d.Missing++
				a[2]++
			case DeadSuspect:
				d.Suspect++
				a[2]++
			case DeadMember:
				d.Member++
				a[2]++
			default:
				d.Unused++
				a[3]++
			}
		}
		d.ByShape[r.Shape] = a
	}
	return d
}

// ---- demand ----

// Demand counts the sites that call a function without a body in the program, by what the model says.
type Demand struct {
	Corpus                string
	LibSites, ModeledSite int
	LibTestSites          int // lib sites in test directories
	ModeledTestSites      int
	Syms, ModeledSyms     int
	StdSites, StdModeled  int
	// DepBody are the sites of Go that call a function of a dependency outside the standard library with no model:
	// the analysis reads its body (gossa Deps), so it is not a call to a function without a body.
	DepBody int
	// by shape: [sites, modeled sites]
	ByShape map[string][2]int
	Top     []DemandSym
}

type DemandSym struct {
	Symbol  string
	Sites   int
	Shape   string
	Modeled bool
}

// shapeOfUnmodeled guesses the shape of a function that has no row: by its types for Go (the standard library
// and what the module loads), by its name for a callback, and "?" otherwise.
func shapeOfUnmodeled(lang, sym string, gu *GoUniverse) string {
	if lang == "go" && gu != nil {
		if fn, st := gu.Resolve(sym); st == "found" {
			sh, weak := FuncShape(fn)
			switch {
			case sh == "":
				return ShapeNone
			case sh == ShapeWrite && weak:
				return "ii?"
			}
			return sh
		}
	}
	if IsCallbackName(lang, sym) {
		return ShapeCall
	}
	if IsWriterName(lang, sym) {
		return "ii?"
	}
	return ShapeUnknown
}

func CountDemand(c *Corpus, mi *ModelIndex, gu *GoUniverse, skipTests bool) Demand {
	d := Demand{Corpus: c.Name, ByShape: map[string][2]int{}}
	for sym, cc := range c.Calls {
		lib := cc.Lib
		if skipTests {
			lib -= cc.LibTest
		}
		if lib <= 0 {
			continue
		}
		modeled := mi.HasModel(c.Lang, sym)
		if c.Lang == "go" && !modeled && !IsStd(c.Lang, sym) {
			d.DepBody += lib
			continue
		}
		sh := mi.ShapeOf(c.Lang, sym)
		if sh == "" {
			sh = shapeOfUnmodeled(c.Lang, sym, gu)
		}
		d.LibSites += lib
		if IsStd(c.Lang, sym) {
			d.StdSites += lib
			if modeled {
				d.StdModeled += lib
			}
		}
		d.Syms++
		a := d.ByShape[sh]
		a[0] += lib
		if modeled {
			d.ModeledSite += lib
			d.ModeledSyms++
			a[1] += lib
		}
		d.ByShape[sh] = a
		d.Top = append(d.Top, DemandSym{sym, lib, sh, modeled})
	}
	sort.Slice(d.Top, func(i, j int) bool {
		if d.Top[i].Sites != d.Top[j].Sites {
			return d.Top[i].Sites > d.Top[j].Sites
		}
		return d.Top[i].Symbol < d.Top[j].Symbol
	})
	return d
}

func pct(a, b int) string {
	if b == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(a)/float64(b))
}
