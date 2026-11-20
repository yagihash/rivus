package main

import (
	"github.com/yagihash/rivus/internal/report"
	"github.com/yagihash/rivus/internal/taint"
)

// offSourcePath returns the findings whose first path has no step in the function of the
// source of the finding (BENCH-2). A finding must show where its taint comes from: the
// function that holds the source site is on the path. When it is not, the source was reached
// through an object that another function wrote back (CORE-A / CORE-B, see
// TestParamSourceSharedObject): the report of a source that is not the one that flows.
//
// The tests that call this (TestDogfood, the three TestCurated*, TestParamSourceSharedObject)
// hold the count as a ceiling that is today's value: it must go down, never up, and it goes
// to 0 when CORE-B is fixed.
func offSourcePath(rep *report.Report) []*taint.Finding {
	var out []*taint.Finding
	for _, f := range rep.Findings {
		if !hasStepIn(f, f.Source.Func) {
			out = append(out, f)
		}
	}
	return out
}

// hasStepIn reports whether the first path of f has a step in the function fn.
func hasStepIn(f *taint.Finding, fn string) bool {
	if len(f.Paths) == 0 {
		return false
	}
	for _, st := range f.Paths[0].Steps {
		if st.Func == fn {
			return true
		}
	}
	return false
}
