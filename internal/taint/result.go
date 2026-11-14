package taint

import (
	"go/token"

	"github.com/yagihash/rivus/internal/frontend/gossa"
)

// Terminal kinds recorded as uncertainty (ADR-008). Only the ones the
// analysis can currently observe are produced.
const (
	KindDynamicDispatch        = "dynamic_dispatch"
	KindUnmodeledLibraryCall   = "unmodeled_library_call"
	KindUnresolvedCallee       = "unresolved_callee"
	KindContainerImprecision   = "container_element_imprecision"
	KindDepthLimit             = "depth_limit"
	KindRecursionApproximation = "recursion_approximation"
	KindMissingSource          = "missing_source"
	AttributableTarget         = "target"
	AttributableRivus          = "rivus"
	AttributableMixed          = "mixed"
	maxPathSteps               = 200
)

// attributable gives the attributable_to value of each kind
// (docs/ARCHITECTURE.md section 4.4).
func attributable(kind string) string {
	switch kind {
	case KindDynamicDispatch, KindUnresolvedCallee:
		return AttributableMixed
	case KindMissingSource:
		return AttributableTarget
	default:
		return AttributableRivus
	}
}

// Uncertainty records a place where rivus could not know, and why.
type Uncertainty struct {
	Kind           string         `json:"kind"`
	AttributableTo string         `json:"attributable_to"`
	Detail         string         `json:"detail,omitempty"`
	Symbol         string         `json:"symbol,omitempty"`
	Candidates     []string       `json:"candidates,omitempty"`
	Func           string         `json:"func,omitempty"`
	Position       gossa.Position `json:"position"`
	Snippet        string         `json:"snippet,omitempty"`

	pos token.Pos
}

func newUncertainty(kind, detail, symbol string) *Uncertainty {
	return &Uncertainty{Kind: kind, AttributableTo: attributable(kind), Detail: detail, Symbol: symbol}
}

// Step is one hop of a taint path.
type Step struct {
	Func     string         `json:"func"`
	Position gossa.Position `json:"position"`
	Snippet  string         `json:"snippet,omitempty"`
	Desc     string         `json:"desc"`
	Notes    []*Uncertainty `json:"notes,omitempty"`

	pos token.Pos
}

// SiteRef identifies a site in a finding.
type SiteRef struct {
	ID       string         `json:"id"`
	Symbol   string         `json:"symbol"`
	Func     string         `json:"func"`
	Position gossa.Position `json:"position"`
	Snippet  string         `json:"snippet"`
}

// Finding is the fact that data from a source site can reach a sink site.
type Finding struct {
	ID          string            `json:"id"`
	Fingerprint map[string]string `json:"fingerprint"`
	Source      SiteRef           `json:"source"`
	Sink        SiteRef           `json:"sink"`
	Category    string            `json:"category,omitempty"`
	Labels      []string          `json:"labels"`
	Paths       []Path            `json:"paths"`
	// Uncertainties lists everything on the path that rivus could not
	// resolve; it is the part a reviewer should look at first.
	Uncertainties []*Uncertainty `json:"uncertainties,omitempty"`
	Confidence    string         `json:"confidence"`
}

// Path is one way the data gets from the source to the sink.
type Path struct {
	Steps []Step `json:"steps"`
}
