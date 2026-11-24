package taint

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yagihash/rivus/internal/sg"
)

// WriteSummaryDump writes the final summary of every function as one line each: the function ID, the number of
// elements and steps, and the SHA-256 of the whole summary (every output with its elements, guards and steps, the
// sinks it reaches, and the marks on the parameters). The lines are sorted by function ID, and what has no order of
// its own (the outputs, the elements of an output, the sinks) is sorted inside, so the file does not depend on the
// order of the work, the threads or the machine. It is for the comparison of the representation before and after a
// change that is meant to change only memory or cost (bench/compare.sh --graph-check): it is read from the
// fields that a summary has, not from how they are stored, so a change of the storage that keeps the summaries
// keeps the file. Internal: no flag; cmd/rivus writes it when RIVUS_INTERNAL_SUMMARY_DUMP names a file.
func (e *Engine) WriteSummaryDump(w io.Writer) error {
	type row struct{ id, line string }
	rows := make([]row, 0, len(e.summaries))
	for fn, sum := range e.summaries {
		if sum == nil {
			continue
		}
		id := e.funcID(fn)
		text, elems, steps := summaryText(sum)
		h := sha256.Sum256([]byte(text))
		rows = append(rows, row{id, fmt.Sprintf("%s\telems=%d\tsteps=%d\t%s", id, elems, steps, hex.EncodeToString(h[:]))})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	bw := bufio.NewWriter(w)
	for _, r := range rows {
		bw.WriteString(r.line)
		bw.WriteByte('\n')
	}
	return bw.Flush()
}

// summaryText is the canonical text of a summary, and its number of elements and steps (the same count as the
// stats summary_elems and summary_steps).
func summaryText(sum *Summary) (string, int, int) {
	elems, steps := 0, 0
	var outs []string
	for _, o := range sum.Outputs {
		g := ""
		if o.Global != nil {
			g = o.Global.Name
		}
		var es []string
		for _, oe := range o.Elems {
			elems++
			steps += oe.Steps.len()
			es = append(es, oe.E.String()+" guards["+guardsKey(oe.Guards)+"] steps["+stepsText(oe.Steps)+"]")
		}
		sort.Strings(es)
		outs = append(outs, fmt.Sprintf("out %s %d %q %q widened=%t {%s}", o.Kind, o.Index, g, o.Path, o.Widened, strings.Join(es, " | ")))
	}
	sort.Strings(outs)
	var sinks []string
	for _, k := range sum.Sinks {
		acc := append([]string(nil), k.Accepts...)
		sort.Strings(acc)
		sinks = append(sinks, fmt.Sprintf("sink %q %s guards[%s] field=%q unsure=%t accepts[%s] steps[%s]",
			k.Site, k.Elem.String(), guardsKey(k.Guards), k.Field, k.Unsure, strings.Join(acc, ","), stepsText(k.Steps)))
	}
	sort.Strings(sinks)
	ints := func(name string, v []int) string {
		var b strings.Builder
		b.WriteString(name + "[")
		for i, n := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.Itoa(n))
		}
		return b.String() + "]"
	}
	var stops []string
	for _, c := range sum.CharStopped {
		stops = append(stops, strconv.Itoa(c.Param)+":"+c.Func)
	}
	parts := append(outs, sinks...)
	parts = append(parts, ints("opaque", sum.Opaque), ints("limited", sum.LimitedParams), ints("unresolved", sum.UnresolvedParams),
		"charstop["+strings.Join(stops, ",")+"]", "widenedoutputs="+strconv.Itoa(sum.WidenedOutputs))
	return strings.Join(parts, "\n"), elems, steps
}

// stepsText is the steps of a path in order, each with its place, the model that decided it and its notes.
func stepsText(r *rope) string {
	var b strings.Builder
	r.each(func(s Step) bool {
		p := s.pos
		fmt.Fprintf(&b, "(%s %q %s:%d:%d:%d:%d", s.Func, s.Desc, p.File, p.Offset, p.Length, p.Line, p.Col)
		if m := s.Model; m != nil {
			fmt.Fprintf(&b, " model=%s/%s/%s/%s/%s/%s", m.Symbol, m.Role, m.Origin, m.DecidedAt, m.ModelVersion, m.Rationale)
		}
		for _, n := range s.Notes {
			fmt.Fprintf(&b, " note=%s/%s/%s/%s", n.Kind, n.AttributableTo, n.Symbol, n.Detail)
		}
		b.WriteByte(')')
		return true
	})
	return b.String()
}

// dumpMinSCC is the size from which a component is written by the dump of each round.
var dumpMinSCC = 100

// iterDump is one large component whose summaries are written after each round: its number N in the order of the
// components (level by level), and the number of rounds done so far, over all passes. Only the goroutine of the
// component changes round.
type iterDump struct {
	n, round int
	warned   bool
}

// SetIterationDump makes the engine write, after each round of every large component (dumpMinSCC functions or
// more), the summaries of its functions to dir/scc<N>-iter<K>.tsv. Internal: no flag; cmd/rivus calls it when
// RIVUS_INTERNAL_SUMMARY_DUMP_EACH_ITER names a directory. Nothing changes in the analysis: the files are only written.
func (e *Engine) SetIterationDump(dir string) { e.iterDumpDir = dir }

// planIterDumps chooses the components to write, before the passes.
func (e *Engine) planIterDumps(levels [][][]*sg.Func) map[*sg.Func]*iterDump {
	if e.iterDumpDir == "" {
		return nil
	}
	if err := os.MkdirAll(e.iterDumpDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "rivus: RIVUS_INTERNAL_SUMMARY_DUMP_EACH_ITER: %v\n", err)
		return nil
	}
	out := map[*sg.Func]*iterDump{}
	n := 0
	for _, lv := range levels {
		for _, scc := range lv {
			if len(scc) >= dumpMinSCC && len(scc) >= parallelSCC {
				out[scc[0]] = &iterDump{n: n}
				n++
			}
		}
	}
	return out
}

// dumpRound writes the summaries of the functions of a component after a round: one line each, sorted by function ID,
// with the fields of WriteSummaryDump and then the breakdown of the elements (summaryBreakdown), whether the function
// changed in this round, and whether its access paths were cut or its cells merged (coarsened). The first line is a
// comment with the pass, the number of functions analysed in the round and the number that changed.
func (e *Engine) dumpRound(scc []*sg.Func, pass, analysed int, changed map[*sg.Func]bool) {
	d := e.iterDumps[scc[0]]
	d.round++
	type snap struct {
		fn     *sg.Func
		sum    *Summary
		coarse bool
	}
	snaps := make([]snap, 0, len(scc))
	e.mu.Lock()
	for _, fn := range scc {
		if sum := e.summaries[fn]; sum != nil {
			_, c := e.coarsened[fn]
			snaps = append(snaps, snap{fn, sum, c})
		}
	}
	e.mu.Unlock()
	rows := make([][2]string, 0, len(snaps))
	for _, s := range snaps {
		id := e.funcID(s.fn)
		text, elems, steps := summaryText(s.sum)
		h := sha256.Sum256([]byte(text))
		chg, coarse := 0, 0
		if changed[s.fn] {
			chg = 1
		}
		if s.coarse {
			coarse = 1
		}
		rows = append(rows, [2]string{id, fmt.Sprintf("%s\telems=%d\tsteps=%d\t%s\t%s\tchg=%d\tcoarse=%d",
			id, elems, steps, hex.EncodeToString(h[:]), summaryBreakdown(s.sum), chg, coarse)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	var b strings.Builder
	fmt.Fprintf(&b, "# pass=%d analysed=%d changed=%d funcs=%d\n", pass, analysed, len(changed), len(scc))
	sums := make([]*Summary, len(snaps))
	for i, s := range snaps {
		sums[i] = s.sum
	}
	fmt.Fprintf(&b, "# outkeys %s\n", topOutputTails(sums, 20))
	paths := map[string]int{}
	owners := map[string]map[string]int{}
	for _, s := range snaps {
		for _, o := range s.sum.Outputs {
			if o.Kind != "param" {
				continue
			}
			k := "param" + strconv.Itoa(o.Index) + o.Path
			paths[k]++
			if owners[k] == nil {
				owners[k] = map[string]int{}
			}
			owners[k][e.funcID(s.fn)]++
		}
	}
	pk := make([]string, 0, len(paths))
	for k := range paths {
		pk = append(pk, k)
	}
	sort.Slice(pk, func(i, j int) bool {
		if paths[pk[i]] != paths[pk[j]] {
			return paths[pk[i]] > paths[pk[j]]
		}
		return pk[i] < pk[j]
	})
	if len(pk) > 40 {
		pk = pk[:40]
	}
	for _, k := range pk {
		ids := make([]string, 0, len(owners[k]))
		for id := range owners[k] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) > 3 {
			ids = ids[:3]
		}
		fmt.Fprintf(&b, "# outpath %q *%d funcs=%d e.g. %s\n", k, paths[k], len(owners[k]), strings.Join(ids, " ; "))
	}
	for _, r := range rows {
		b.WriteString(r[1])
		b.WriteByte('\n')
	}
	name := filepath.Join(e.iterDumpDir, fmt.Sprintf("scc%d-iter%d.tsv", d.n, d.round))
	if err := os.WriteFile(name, []byte(b.String()), 0o644); err != nil && !d.warned {
		d.warned = true
		fmt.Fprintf(os.Stderr, "rivus: RIVUS_INTERNAL_SUMMARY_DUMP_EACH_ITER: %v\n", err)
	}
}

// topOutputTails counts the outputs of the summaries by their kind and the last step of their path (the field the
// output points into, see the access paths in elem.go; no step: "-"), and returns the n most frequent as
// "kind:tail*count", the most frequent first (ties by name).
func topOutputTails(sums []*Summary, n int) string {
	counts := map[string]int{}
	for _, sum := range sums {
		for _, o := range sum.Outputs {
			tail := "-"
			if i := strings.LastIndex(o.Path, "/"); i >= 0 {
				tail = o.Path[i+1:]
			}
			counts[o.Kind+":"+tail]++
		}
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > n {
		keys = keys[:n]
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "*" + strconv.Itoa(counts[k])
	}
	return strings.Join(parts, " ")
}

// summaryBreakdown is what the elements of a summary are made of, as fields of one line, each sorted by its key so
// that the order of the work does not matter: outs and sinks (their numbers), real and param (elements by kind),
// guarded (elements with a condition), widened (outputs that were merged, then the summary's own count), kinds
// (elements by the kind of the output: result, param, global, throw), depth (param elements by the depth of their
// access path) and byparam (param elements by the parameter index).
func summaryBreakdown(sum *Summary) string {
	real, param, guarded, widened := 0, 0, 0, 0
	kinds := map[string]int{}
	depth := map[int]int{}
	byParam := map[int]int{}
	outDepth := map[int]int{}
	outIdx := map[string]int{}
	maxOut := 0
	for _, o := range sum.Outputs {
		if o.Widened {
			widened++
		}
		outDepth[pathDepth(o.Path)]++
		outIdx[o.Kind+"@"+strconv.Itoa(o.Index)]++
		if len(o.Elems) > maxOut {
			maxOut = len(o.Elems)
		}
		kinds[o.Kind] += len(o.Elems)
		for _, oe := range o.Elems {
			if len(oe.Guards) > 0 {
				guarded++
			}
			if oe.E.Kind == Real {
				real++
				continue
			}
			param++
			depth[pathDepth(oe.E.Path())]++
			byParam[int(oe.E.Param)]++
		}
	}
	names := make([]string, 0, len(kinds))
	for k := range kinds {
		names = append(names, k)
	}
	sort.Strings(names)
	ks := make([]string, len(names))
	for i, k := range names {
		ks[i] = k + ":" + strconv.Itoa(kinds[k])
	}
	ints := func(m map[int]int) string {
		keys := make([]int, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = strconv.Itoa(k) + ":" + strconv.Itoa(m[k])
		}
		return strings.Join(parts, ",")
	}
	idxNames := make([]string, 0, len(outIdx))
	for k := range outIdx {
		idxNames = append(idxNames, k)
	}
	sort.Strings(idxNames)
	is := make([]string, len(idxNames))
	for i, k := range idxNames {
		is[i] = k + ":" + strconv.Itoa(outIdx[k])
	}
	return fmt.Sprintf("outs=%d\tsinks=%d\treal=%d\tparam=%d\tguarded=%d\twidened=%d/%d\tkinds=%s\tdepth=%s\tbyparam=%s\toutdepth=%s\toutidx=%s\tmaxout=%d",
		len(sum.Outputs), len(sum.Sinks), real, param, guarded, widened, sum.WidenedOutputs, strings.Join(ks, ","), ints(depth), ints(byParam), ints(outDepth), strings.Join(is, ","), maxOut)
}
