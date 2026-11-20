// Package sites enumerates the places where a rule matches: source sites,
// where untrusted data enters, and sink sites, where it must not arrive.
//
// Enumeration is level L0 (rule matching on resolved symbols) plus the L1
// literal mark. It needs neither a call graph nor any data flow analysis, so
// it is fast. Every site has a stable ID and a fingerprint (ADR-028).
package sites

import (
	"sort"

	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/model"
)

// Roles of a site.
const (
	RoleSource = "source"
	RoleSink   = "sink"
)

// Site is one place where a rule matches.
type Site struct {
	ID          string            `json:"id"`
	Role        string            `json:"role"`
	Kind        string            `json:"kind"` // call_result, param_type, field, call
	Func        string            `json:"func"`
	Symbol      string            `json:"symbol"`
	Category    string            `json:"category,omitempty"`
	Labels      []string          `json:"labels,omitempty"` // source labels, or the labels a sink accepts
	Position    ir.Position       `json:"position"`
	Snippet     string            `json:"snippet"`
	LiteralOnly *bool             `json:"literal_only,omitempty"` // L1, sinks only
	Fingerprint map[string]string `json:"fingerprint"`

	// IR is where the site is in the neutral IR.
	IR *IRRef `json:"-"`

	// Callee is the function a site on a call of a function value is for (the
	// call may reach several, and each is a site of its own).
	Callee string `json:"-"`

	Sink  []model.Sink   `json:"-"`
	Src   []model.Source `json:"-"`
	irpos ir.Pos
	order int
}

// Result is the enumeration of a program.
type Result struct {
	Sites []*Site
	// ByIRInstr maps an instruction of the IR to the sites it carries, and
	// ByIRParam a parameter.
	ByIRInstr map[IRInstr][]*Site
	ByIRParam map[IRParam][]*Site
	// ByID finds a site by ID.
	ByID map[string]*Site
}

// SinkArgs returns the dangerous argument positions of a call with n
// arguments, according to the sink rules (all arguments when none is given).
func SinkArgs(sinks []model.Sink, n int) []int {
	seen := map[int]bool{}
	var out []int
	for _, sk := range sinks {
		if len(sk.Args) == 0 {
			for i := 0; i < n; i++ {
				if !seen[i] {
					seen[i] = true
					out = append(out, i)
				}
			}
			continue
		}
		for _, i := range sk.Args {
			if i >= 0 && i < n && !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	sort.Ints(out)
	return out
}

func labelsOf(srcs []model.Source) []string {
	set := map[string]bool{}
	for _, s := range srcs {
		for _, l := range s.Labels {
			set[l] = true
		}
	}
	return sortedKeys(set)
}

// acceptsOf merges the labels the sink rules react to. A rule without a
// label list reacts to every label, so the merged answer is "any" (nil).
func acceptsOf(sinks []model.Sink) []string {
	set := map[string]bool{}
	for _, s := range sinks {
		if len(s.Accepts) == 0 {
			return nil
		}
		for _, l := range s.Accepts {
			set[l] = true
		}
	}
	return sortedKeys(set)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
