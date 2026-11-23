package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/model"
)

// RejectedSummary is what a review of the pull request looks at: how many `rejected:` entries of the package this change adds (against "+base+"),
// and the reasons by group (the same `why` is one group), with the count of each; and the `deferred:` entries with their refs.
func RejectedSummary(repo, base string, f *Facts) string {
	old := map[string]bool{}
	for _, file := range f.Files {
		rel, err := filepath.Rel(repo, file)
		if err != nil {
			continue
		}
		out, err := exec.Command("git", "-C", repo, "show", base+":"+filepath.ToSlash(rel)).Output()
		if err != nil {
			continue
		}
		rs, err := model.Parse(strings.NewReader(string(out)), base+":"+rel)
		if err != nil {
			continue
		}
		for _, r := range rs.Rejected {
			old[r.Symbol+"\x00"+r.Kind+"\x00"+r.Position] = true
		}
	}
	type group struct {
		why        string
		total, add int
		kinds      map[string]int
	}
	groups := map[string]*group{}
	added := 0
	for _, r := range f.Rejected {
		g := groups[r.Why]
		if g == nil {
			g = &group{why: r.Why, kinds: map[string]int{}}
			groups[r.Why] = g
		}
		g.total++
		g.kinds[r.Kind]++
		if !old[r.Symbol+"\x00"+r.Kind+"\x00"+r.Position] {
			g.add++
			added++
		}
	}
	var gs []*group
	for _, g := range groups {
		gs = append(gs, g)
	}
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].total != gs[j].total {
			return gs[i].total > gs[j].total
		}
		return gs[i].why < gs[j].why
	})
	var b strings.Builder
	fmt.Fprintf(&b, "rejected: %d entries of the package, %d added by this change (against %s), in %d group(s) of the same reason\n", len(f.Rejected), added, base, len(gs))
	for _, g := range gs {
		var ks []string
		for k, n := range g.kinds {
			ks = append(ks, fmt.Sprintf("%s %d", k, n))
		}
		sort.Strings(ks)
		fmt.Fprintf(&b, "  %3d (added %d; %s): %s\n", g.total, g.add, strings.Join(ks, ", "), g.why)
	}
	fmt.Fprintf(&b, "deferred: %d entries\n", len(f.Deferred))
	for _, d := range f.Deferred {
		fmt.Fprintf(&b, "  %s %s %s ref %s: %s\n", d.Kind, d.Symbol, d.Position, d.Ref, d.Why)
	}
	return b.String()
}

// checkDeferredRefs: the file of every `ref` of `deferred:` exists in docs/issues (the loader checks the form only; the files are the checkout's).
func checkDeferredRefs(repo string, f *Facts) error {
	for _, d := range f.Deferred {
		pat := filepath.Join(repo, "docs", "issues", model.DeferredRefFile(d.Ref))
		if m, _ := filepath.Glob(pat); len(m) == 0 {
			return fmt.Errorf("%s: deferred %s: ref %s has no file in docs/issues", d.File, d.Symbol, d.Ref)
		}
	}
	return nil
}

// Breakdown counts the candidates by kind, the rule (category), the word that fired, and the shape of the type (Dart; Go has no detail yet):
// how many there are, how many have a row, are rejected or deferred, and are undecided. Text for a work file.
func Breakdown(api *API, m *Matched) string {
	status := map[string]string{}
	mark := func(items []Item, s string) {
		for _, it := range items {
			status[it.Kind+"\x00"+it.Symbol] = s
		}
	}
	mark(m.ByRow, "row")
	mark(m.ByRejected, "decided")
	mark(m.ByDeferred, "decided")
	mark(m.Undecided, "undecided")
	type cnt struct{ total, row, decided, undecided int }
	rows := map[string]*cnt{}
	byKind := map[string]*cnt{}
	add := func(c *cnt, st string) {
		c.total++
		switch st {
		case "row":
			c.row++
		case "decided":
			c.decided++
		default:
			c.undecided++
		}
	}
	for _, it := range api.Items {
		st := status[it.Kind+"\x00"+it.Symbol]
		k := byKind[it.Kind]
		if k == nil {
			k = &cnt{}
			byKind[it.Kind] = k
		}
		add(k, st)
		seen := map[string]bool{}
		details := it.Detail
		if len(details) == 0 {
			details = []string{"-"}
		}
		for _, d := range details {
			key := it.Kind + " " + d
			if seen[key] {
				continue
			}
			seen[key] = true
			c := rows[key]
			if c == nil {
				c = &cnt{}
				rows[key] = c
			}
			add(c, st)
		}
	}
	var keys []string
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := rows[keys[i]], rows[keys[j]]
		if a.undecided != b.undecided {
			return a.undecided > b.undecided
		}
		return keys[i] < keys[j]
	})
	var b strings.Builder
	fmt.Fprintf(&b, "breakdown of the layer-1 candidates of %s (a candidate with several places is counted once in each line)\n\n", api.Pkg)
	b.WriteString("by kind:        total  row  decided  undecided\n")
	var kinds []string
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		c := byKind[k]
		fmt.Fprintf(&b, "  %-12s %6d %4d %8d %10d\n", k, c.total, c.row, c.decided, c.undecided)
	}
	b.WriteString("\nby kind, rule:word/shape (undecided first):\n   total  row  decided  undecided\n")
	for _, k := range keys {
		c := rows[k]
		fmt.Fprintf(&b, "  %6d %4d %8d %10d  %s\n", c.total, c.row, c.decided, c.undecided, k)
	}
	return b.String()
}
