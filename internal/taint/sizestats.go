package taint

import (
	"math"
	"sort"
	"strconv"

	"github.com/yagihash/rivus/internal/sg"
)

// SummarySizes is how the elements of the summaries are spread over the functions, and what they are made of
// (-pass-stats). The total (summary_elems) says how much there is; this says where: a few functions that hold most of
// it, or many functions that hold hundreds each, and which kind of element it is. It has numbers only (no function and
// no name of the code that is read), so it can be passed on.
type SummarySizes struct {
	// Functions are the functions that have a summary with at least one element; Empty are the ones that have none.
	Functions int `json:"functions"`
	Empty     int `json:"empty"`
	// The size of a summary is the number of its elements (over all its outputs). Median, P90 and P99 are of the
	// functions with a summary, and Max is the largest.
	Median int `json:"median"`
	P90    int `json:"p90"`
	P99    int `json:"p99"`
	Max    int `json:"max"`
	// Over100, Over1000 and Over10000 are how many functions have a summary larger than that.
	Over100   int `json:"over_100"`
	Over1000  int `json:"over_1000"`
	Over10000 int `json:"over_10000"`
	// Top1PctShare is the part of all the elements (in thousandths) that the largest 1% of the functions hold: near
	// 1000 is a few functions that hold nearly everything, near 10 is an even spread.
	Top1PctShare int `json:"top_1pct_share_permille"`
	// OwnElems and DepElems are the elements of the functions of the project and of its dependencies.
	OwnElems int `json:"own_elems"`
	DepElems int `json:"dep_elems"`
	// ByOutput is the elements by what they end up in: "result", "param" (a write into the object behind a parameter),
	// "global" and "throw".
	ByOutput map[string]int `json:"by_output"`
	// ByElement is the elements by what they are: "real" (a concrete label of a source), "param" (taint of a parameter,
	// symbolic: whatever the caller passes), "param_0" (of the parameter 0: the receiver, when there is one),
	// "with_path" (of a parameter, below a path inside it), "with_guards" (under a condition on a parameter) and
	// "with_mods" (after a sanitizer operation).
	ByElement map[string]int `json:"by_element"`
	// RealOrigins is how many different source sites the Real elements of the summaries come from, and OriginReach is, over
	// them, in how many functions each one is carried (Max, Median, P90; Over100 and Over1000 are how many source sites
	// are in more functions than that): a few sites in every function, or each site in a few functions.
	RealOrigins int          `json:"real_origins"`
	OriginReach *ReachSpread `json:"origin_reach,omitempty"`
	// GlobalsTop is the number of elements that the summaries put into each global variable (the 10 with the most, in
	// numbers only: no name), and GlobalsWithElems is how many globals have any.
	GlobalsTop       []int `json:"globals_top,omitempty"`
	GlobalsWithElems int   `json:"globals_with_elems"`
	// ByPathDepth is the elements of a parameter by the depth of their path inside it ("0" is the parameter itself, "3+"
	// is three or more: the depth that the analysis keeps is -k).
	ByPathDepth map[string]int `json:"by_path_depth"`
}

// ReachSpread is how a count (the functions that a source site is carried in) is spread over a set of source sites.
type ReachSpread struct {
	Max      int `json:"max"`
	Median   int `json:"median"`
	P90      int `json:"p90"`
	Over100  int `json:"over_100"`
	Over1000 int `json:"over_1000"`
}

func (e *Engine) summarySizes() *SummarySizes {
	s := &SummarySizes{ByOutput: map[string]int{}, ByElement: map[string]int{}, ByPathDepth: map[string]int{}}
	var sizes []int
	total := 0
	reach := map[strID]int{} // source site -> the functions that carry it
	globals := map[*sg.Value]int{}
	for fn, sum := range e.summaries {
		n := 0
		seen := map[strID]struct{}{}
		for _, o := range sum.Outputs {
			n += len(o.Elems)
			s.ByOutput[o.Kind] += len(o.Elems)
			if o.Kind == OutGlobal && o.Global != nil {
				globals[o.Global] += len(o.Elems)
			}
			for _, oe := range o.Elems {
				if oe.E.Kind == Real {
					s.ByElement["real"]++
					if _, ok := seen[oe.E.origin]; !ok {
						seen[oe.E.origin] = struct{}{}
						reach[oe.E.origin]++
					}
					continue
				}
				s.ByElement["param"]++
				if oe.E.Param == 0 {
					s.ByElement["param_0"]++
				}
				d := pathDepth(oe.E.Path())
				if d >= 3 {
					s.ByPathDepth["3+"]++
				} else {
					s.ByPathDepth[strconv.Itoa(d)]++
				}
				if oe.E.Path() != "" {
					s.ByElement["with_path"]++
				}
				if oe.E.Mods() != "" {
					s.ByElement["with_mods"]++
				}
				if len(oe.Guards) > 0 {
					s.ByElement["with_guards"]++
				}
			}
		}
		if n == 0 {
			s.Empty++
			continue
		}
		sizes = append(sizes, n)
		total += n
		if fn.Dep {
			s.DepElems += n
		} else {
			s.OwnElems += n
		}
	}
	s.RealOrigins = len(reach)
	if len(reach) > 0 {
		counts := make([]int, 0, len(reach))
		for _, c := range reach {
			counts = append(counts, c)
		}
		sort.Ints(counts)
		q := func(f float64) int {
			return counts[int(math.Min(float64(len(counts)-1), math.Floor(f*float64(len(counts)))))]
		}
		r := &ReachSpread{Max: counts[len(counts)-1], Median: q(0.5), P90: q(0.9)}
		for _, c := range counts {
			if c > 100 {
				r.Over100++
			}
			if c > 1000 {
				r.Over1000++
			}
		}
		s.OriginReach = r
	}
	for _, n := range globals {
		if n > 0 {
			s.GlobalsWithElems++
			s.GlobalsTop = append(s.GlobalsTop, n)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(s.GlobalsTop)))
	if len(s.GlobalsTop) > 10 {
		s.GlobalsTop = s.GlobalsTop[:10]
	}
	s.Functions = len(sizes)
	if len(sizes) == 0 {
		return s
	}
	sort.Ints(sizes)
	at := func(q float64) int {
		return sizes[int(math.Min(float64(len(sizes)-1), math.Floor(q*float64(len(sizes)))))]
	}
	s.Median, s.P90, s.P99, s.Max = at(0.5), at(0.9), at(0.99), sizes[len(sizes)-1]
	top := (len(sizes) + 99) / 100
	held := 0
	for _, n := range sizes[len(sizes)-top:] {
		held += n
	}
	s.Top1PctShare = held * 1000 / total
	for _, n := range sizes {
		if n > 100 {
			s.Over100++
		}
		if n > 1000 {
			s.Over1000++
		}
		if n > 10000 {
			s.Over10000++
		}
	}
	return s
}
