package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Probe is one generated function `fixN`: a call of one function of the library with the source in one position.
type Probe struct {
	ID       int      `json:"id"`
	Symbol   string   `json:"symbol"`
	Kind     string   `json:"kind"`              // flow: compared with the body; sink: compared with the declaration
	Variant  string   `json:"variant"`           // the position that has the source (recv, arg0, ...) or "const"
	Outputs  []string `json:"outputs,omitempty"` // flow: the places that have a sink of their own (result0, arg1, recv, ...)
	Category string   `json:"category,omitempty"`
	Expect   *bool    `json:"expect,omitempty"` // sink: whether a finding is declared
}

// Skip is what the generator did not make: not checked, and said so.
type Skip struct {
	Symbol  string `json:"symbol"`
	Variant string `json:"variant,omitempty"`
	Reason  string `json:"reason"`
}

// Hits is the findings of one run: the probe and the sink that it reached ("" for the sink that a sinks row declares).
type Hits map[[2]string]bool

func key(id int, out string) [2]string { return [2]string{fmt.Sprint(id), out} }

var (
	fixRe  = regexp.MustCompile(`fix(\d+)$`)
	sinkRe = regexp.MustCompile(`sink(\d+)_(\w+)$`)
)

type finding struct {
	Category string `json:"category"`
	Source   struct {
		Func string `json:"func"`
	} `json:"source"`
	Sink struct {
		Symbol string `json:"symbol"`
	} `json:"sink"`
}

// ReadHits reads the JSON of `rivus analyze`. A finding of a sink of the generated code is a hit of the place (`fixN`, `sinkN_<out>`),
// any other finding is a hit of the sink of the model that `fixN` calls, with its category.
func ReadHits(data []byte) (Hits, error) {
	var res struct {
		Findings []finding `json:"findings"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	h := Hits{}
	for _, f := range res.Findings {
		m := fixRe.FindStringSubmatch(f.Source.Func)
		if m == nil {
			continue
		}
		if s := sinkRe.FindStringSubmatch(f.Sink.Symbol); s != nil && s[1] == m[1] {
			h[[2]string{m[1], s[2]}] = true
		} else if !sinkRe.MatchString(f.Sink.Symbol) {
			h[[2]string{m[1], ""}] = true
			h[[2]string{m[1], "cat:" + f.Category}] = true
		}
	}
	return h, nil
}

// Result is the verdict of one probe.
type Result struct {
	Probe
	Status string   `json:"status"` // agree, MISS, EXTRA, EXTRA-intended
	Miss   []string `json:"miss,omitempty"`
	Extra  []string `json:"extra,omitempty"`
	Reason string   `json:"reason,omitempty"` // the reason of the note that allows the EXTRA or the reviewed MISS
}

type Report struct {
	Language   string         `json:"language"`
	Package    string         `json:"package"`
	Models     []string       `json:"models"`
	Counts     map[string]int `json:"counts"`
	Results    []Result       `json:"results"`
	Unverified []Skip         `json:"unverified"`
	Stale      []string       `json:"stale_notes,omitempty"`
	Reviewed   []string       `json:"miss_reviewed,omitempty"` // every miss-reviewed note used: <symbol>@<position> <kind> <explanation>
	Errors     []string       `json:"errors,omitempty"`
	Runs       map[string]Run `json:"runs,omitempty"`
}

// Run is the cost of one analysis.
type Run struct {
	Seconds float64 `json:"seconds"`
	MaxRSS  int64   `json:"max_rss_mb"`
}

// Compare judges the probes. a is the run that reads the bodies (nil when only declarations are checked), b the run with the models.
func Compare(probes []Probe, skips []Skip, a, b Hits, notes []Note) *Report {
	extra := map[string]Note{}
	for _, n := range notes {
		if n.Kind == "extra" {
			extra[n.Symbol] = n
		}
	}
	reviewed := map[string]Note{}
	for _, n := range notes {
		if n.Kind == "miss-reviewed" {
			reviewed[n.Symbol] = n
		}
	}
	usedMiss := map[string]bool{}
	used := map[string]bool{}
	rep := &Report{Counts: map[string]int{}, Unverified: skips}
	if rep.Unverified == nil {
		rep.Unverified = []Skip{}
	}
	for _, p := range probes {
		r := Result{Probe: p, Status: "agree"}
		if p.Kind == "flow" {
			for _, o := range p.Outputs {
				inA, inB := a[key(p.ID, o)], b[key(p.ID, o)]
				if inA && !inB {
					r.Miss = append(r.Miss, o)
				} else if inB && !inA {
					r.Extra = append(r.Extra, o)
				}
			}
		} else {
			got := b[key(p.ID, "cat:"+p.Category)]
			if p.Expect != nil && *p.Expect && !got {
				r.Miss = []string{"no finding at " + p.Variant}
			} else if (p.Expect == nil || !*p.Expect) && got {
				r.Extra = []string{"finding at " + p.Variant}
			}
		}
		switch {
		case len(r.Miss) > 0:
			pos := p.Variant
			if i := strings.IndexByte(pos, ' '); i > 0 {
				pos = pos[:i]
			}
			if n, ok := reviewed[p.Symbol+"@"+pos]; ok {
				r.Status, r.Reason = "MISS-reviewed", n.Text
				if !usedMiss[n.Symbol] {
					rep.Reviewed = append(rep.Reviewed, n.Symbol+" "+n.Text)
				}
				usedMiss[n.Symbol] = true
			} else {
				r.Status = "MISS"
			}
		case len(r.Extra) > 0:
			if n, ok := extra[p.Symbol]; ok {
				r.Status, r.Reason = "EXTRA-intended", n.Text
				used[p.Symbol] = true
			} else {
				r.Status = "EXTRA"
			}
		}
		rep.Counts[r.Status]++
		if p.Kind == "flow" && r.Status == "agree" && !anyHit(a, b, p) {
			rep.Counts["agree_no_flow"]++ // neither run has a flow here: the probe shows nothing about this position
		}
		rep.Results = append(rep.Results, r)
	}
	rep.Counts["unverified"] = len(skips)
	for _, n := range notes {
		if n.Kind == "miss-reviewed" && !usedMiss[n.Symbol] {
			rep.Stale = append(rep.Stale, fmt.Sprintf("%s:%d: no MISS for %s (the note is not needed)", n.File, n.Line, n.Symbol))
		}
		if n.Kind == "extra" && !used[n.Symbol] {
			rep.Stale = append(rep.Stale, fmt.Sprintf("%s:%d: no EXTRA for %s (the note is not needed)", n.File, n.Line, n.Symbol))
		}
	}
	return rep
}

func anyHit(a, b Hits, p Probe) bool {
	for _, o := range p.Outputs {
		if a[key(p.ID, o)] || b[key(p.ID, o)] {
			return true
		}
	}
	return false
}

// Failed is the exit status: a MISS, an EXTRA that no note allows, or an error.
func (r *Report) Failed() bool {
	return r.Counts["MISS"] > 0 || r.Counts["EXTRA"] > 0 || len(r.Errors) > 0
}

// Print writes the table.
func (r *Report) Print(w io.Writer) {
	fmt.Fprintf(w, "model-diff %s %s\n", r.Language, r.Package)
	for _, x := range r.Results {
		if x.Status == "agree" {
			continue
		}
		detail := strings.Join(append(append([]string{}, x.Miss...), x.Extra...), ",")
		if x.Reason != "" {
			detail += "  (" + x.Reason + ")"
		}
		fmt.Fprintf(w, "  %-15s %-8s %-52s %-10s %s\n", x.Status, x.Kind, x.Symbol, x.Variant, detail)
	}
	for _, u := range r.Unverified {
		fmt.Fprintf(w, "  %-15s %-8s %-52s %-10s %s\n", "unverified", "", u.Symbol, u.Variant, u.Reason)
	}
	for _, s := range r.Stale {
		fmt.Fprintf(w, "  stale note: %s\n", s)
	}
	fmt.Fprintf(w, "  miss-reviewed: %d\n", len(r.Reviewed))
	for _, s := range r.Reviewed {
		fmt.Fprintf(w, "    miss-reviewed %s\n", s)
	}
	for _, e := range r.Errors {
		fmt.Fprintf(w, "  ERROR: %s\n", e)
	}
	var ks []string
	for k := range r.Counts {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var parts []string
	for _, k := range ks {
		parts = append(parts, fmt.Sprintf("%s=%d", k, r.Counts[k]))
	}
	fmt.Fprintf(w, "  total: %d probes: %s\n", len(r.Results), strings.Join(parts, " "))
	for _, k := range []string{"A", "B"} {
		if x, ok := r.Runs[k]; ok {
			fmt.Fprintf(w, "  run %s: %.1f s, %d MB\n", k, x.Seconds, x.MaxRSS)
		}
	}
}
