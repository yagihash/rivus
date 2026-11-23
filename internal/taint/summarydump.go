package taint

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
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
