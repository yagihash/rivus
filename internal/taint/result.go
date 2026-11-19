package taint

import (
	"github.com/yagihash/rivus/internal/ir"
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
	KindReflection             = "reflection"
	KindConcurrency            = "concurrency"
	KindFFIBoundary            = "ffi_boundary"
	KindGlobalVar              = "global_var"
	KindConfigRead             = "config_read"
	KindExportedAPIParam       = "exported_api_param"
	KindStoredDataBoundary     = "stored_data_boundary"
	// KindNumericStop marks the data of a source that became a number or a bool: they are
	// not followed (attributable to rivus: -numeric-taint follows them).
	KindNumericStop = "numeric_stop"
	// KindUnsupported marks a construct that the extractor could not express: what
	// it does is not in the analysis (attributable to rivus).
	KindUnsupported    = "unsupported_construct"
	AttributableTarget = "target"
	AttributableRivus  = "rivus"
	AttributableMixed  = "mixed"
	maxPathSteps       = 200
)

// attributable gives the attributable_to value of each kind
// (docs/ARCHITECTURE.md section 4.4).
func attributable(kind string) string {
	switch kind {
	case KindDynamicDispatch, KindUnresolvedCallee, KindConcurrency:
		return AttributableMixed
	case KindMissingSource, KindReflection, KindFFIBoundary, KindGlobalVar, KindConfigRead, KindExportedAPIParam, KindStoredDataBoundary:
		return AttributableTarget
	default:
		return AttributableRivus
	}
}

// Uncertainty records a place where rivus could not know, and why.
type Uncertainty struct {
	Kind           string      `json:"kind"`
	AttributableTo string      `json:"attributable_to"`
	Detail         string      `json:"detail,omitempty"`
	Symbol         string      `json:"symbol,omitempty"`
	Candidates     []string    `json:"candidates,omitempty"`
	Func           string      `json:"func,omitempty"`
	Position       ir.Position `json:"position"`
	Snippet        string      `json:"snippet,omitempty"`

	pos ir.Pos
}

// NewUncertainty creates an uncertainty with the attributable_to of its kind.
func NewUncertainty(kind, detail, symbol string) *Uncertainty {
	return newUncertainty(kind, detail, symbol)
}

func newUncertainty(kind, detail, symbol string) *Uncertainty {
	return &Uncertainty{Kind: kind, AttributableTo: attributable(kind), Detail: detail, Symbol: symbol}
}

// ModelRef says which model decided a hop (or an end) of a path, and who
// decided the model. It is how a reader sees that an LLM influenced a result
// (ADR-010).
type ModelRef struct {
	Symbol       string `json:"symbol"`
	Role         string `json:"role"` // source, sink, sanitizer or propagator
	Origin       string `json:"origin"`
	DecidedAt    string `json:"decided_at,omitempty"`
	ModelVersion string `json:"model_version,omitempty"`
	Rationale    string `json:"rationale,omitempty"`
}

// Suppression records a place where a model decided by an LLM removed taint:
// without the model, the data would have gone on. A finding that does not
// exist cannot say that, so the place is listed on its own (ADR-010).
type Suppression struct {
	Model    ModelRef    `json:"model"`
	Func     string      `json:"func"`
	Position ir.Position `json:"position"`
	Snippet  string      `json:"snippet,omitempty"`
	// Labels were removed; Origins are the source sites they came from.
	Labels  []string `json:"labels"`
	Origins []string `json:"origins"`

	pos ir.Pos
}

// Step is one hop of a taint path.
type Step struct {
	Func     string         `json:"func"`
	Position ir.Position    `json:"position"`
	Snippet  string         `json:"snippet,omitempty"`
	Desc     string         `json:"desc"`
	Notes    []*Uncertainty `json:"notes,omitempty"`
	// Model is set when a model decided this hop.
	Model *ModelRef `json:"model,omitempty"`

	guards []Guard

	pos ir.Pos
}

// SiteRef identifies a site in a finding.
type SiteRef struct {
	ID       string      `json:"id"`
	Symbol   string      `json:"symbol"`
	Func     string      `json:"func"`
	Position ir.Position `json:"position"`
	Snippet  string      `json:"snippet"`
	// Fingerprint lets a later run find the same site again (rivus resolve).
	Fingerprint map[string]string `json:"fingerprint,omitempty"`
}

// OtherSource is a source that reaches the same sink as the finding it is listed
// in, with what is known of its own path: so that a reviewer can tell a path that is
// as clear as the one shown from one that is not, without asking for it.
type OtherSource struct {
	SiteRef
	PathSteps          int    `json:"path_steps"`
	Uncertainties      int    `json:"uncertainties"`
	DispatchWidened    int    `json:"dispatch_widened,omitempty"`
	DispatchCandidates int    `json:"dispatch_candidates,omitempty"`
	Reachability       string `json:"reachability,omitempty"`
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
	// OtherSources are more sources of the same kind (a parameter of the same
	// type, in other functions) that reach this sink with the same labels. The
	// finding shows the shortest path; the rest are not repeated as findings,
	// since a request type is a source in every function that takes it, and
	// the same flow would be reported once per function. The finding shows the
	// path that is the best to look at first (reachable from where the program
	// starts, with the fewest things left unresolved, the fewest calls taken
	// to reach several implementations, then the shortest): the shortest alone
	// favours a path that goes through a function value taken to reach every
	// candidate. At most maxOtherSources are listed (without snippets, with what is
	// known of each path); the count has them all.
	OtherSources      []OtherSource `json:"other_sources,omitempty"`
	OtherSourcesCount int           `json:"other_sources_count,omitempty"`
	// Uncertainties lists everything on the path that rivus could not
	// resolve; it is the part a reviewer should look at first.
	Uncertainties []*Uncertainty `json:"uncertainties,omitempty"`
	// PathSteps is the number of steps of the (shortest) path shown. The
	// longer a path, the more of its hops can be a guess; this is a fact for
	// ranking, not a verdict.
	PathSteps int `json:"path_steps"`
	// Reachability says whether every function on the path can be reached from
	// where the program starts (its entry points: main and the initialisation
	// of the packages; for a library, its exported functions). A path that
	// cannot is in code that does not run in the program: a helper that only
	// the tests use, a mock, or a handler that is registered by reflection
	// (which rivus does not follow). Absent when nothing says where the
	// program starts.
	Reachability string `json:"reachability,omitempty"`
	// DispatchWidened is the number of calls on the path that went through an
	// interface or a function value and were taken to reach every one of
	// several implementations; DispatchCandidates is the most implementations
	// one of them had. The flow is real only if the value on this path is of
	// the type whose method holds the rest of the path: a reviewer can start
	// with the paths that have none of these. A fact for ranking, not a verdict.
	DispatchWidened    int `json:"dispatch_widened,omitempty"`
	DispatchCandidates int `json:"dispatch_candidates,omitempty"`
	// Confidence classifies what is known about the path, and does not judge
	// it: ConfidenceClear when nothing on it was left unresolved,
	// ConfidenceReview otherwise.
	Confidence string `json:"confidence"`
	// Models are the models that decided something on the path: the source
	// and sink rules, and the sanitizers and propagators passed.
	Models []ModelRef `json:"models,omitempty"`
	// LLMInfluenced is true when one of them was decided by an LLM.
	LLMInfluenced bool `json:"llm_influenced,omitempty"`
	// ConditionalNodes are the places on the path where the outcome
	// depends on a value the caller passes.
	ConditionalNodes []ConditionalNode `json:"conditional_nodes,omitempty"`
}

// Values of Finding.Confidence.
const (
	ConfidenceClear  = "no_uncertainty_on_path"
	ConfidenceReview = "requires_review"
)

// Values of Finding.Reachability.
const (
	ReachFromEntry    = "from_entry"
	ReachNotFromEntry = "not_from_entry"
)

// Path is one way the data gets from the source to the sink.
type Path struct {
	Steps []Step `json:"steps"`
}

// Kinds of conditional node.
const (
	// CondSanitization: depending on the condition, taint is removed or kept.
	CondSanitization = "value_dependent_sanitization"
	// CondFlow: the flow differs, in some other way, with the condition.
	CondFlow = "value_dependent_flow"
)

// ConditionalNode says: here the outcome depends on a value the caller
// passes. It lists which outcome goes with which value and shows what the
// caller passes. It does not say which outcome happens (ADR-001, ADR-006).
type ConditionalNode struct {
	Function        string      `json:"function"`
	Location        ir.Position `json:"location"`
	Kind            string      `json:"kind"`
	BranchCondition string      `json:"branch_condition"`
	DerivedFrom     string      `json:"derived_from"`
	// Outcomes maps "<condition> == true" and "<condition> == false" to
	// what happens on that side.
	Outcomes map[string]string `json:"outcomes"`
	// CallSite is where the condition's parameter gets its value on the
	// path of the finding; CallSiteArgument is the call as written and
	// Argument the expression passed.
	CallSite         *ir.Position `json:"call_site,omitempty"`
	CallSiteArgument string       `json:"call_site_argument,omitempty"`
	Argument         string       `json:"argument,omitempty"`
	// Resolved is true when the argument is a constant, which is then in
	// Provenance.
	Resolved       bool        `json:"resolved"`
	FunctionSource string      `json:"function_source,omitempty"`
	Provenance     *Provenance `json:"provenance,omitempty"`
}

// Provenance traces where the value of an argument comes from, and says why
// the tracing stopped where it did (ADR-008).
type Provenance struct {
	Argument   string            `json:"argument"`
	Resolved   []ResolvedValue   `json:"resolved"`
	Unresolved []UnresolvedValue `json:"unresolved"`
}

// ResolvedValue is a value the tracing found for certain.
type ResolvedValue struct {
	Value     string      `json:"value"`
	CallSite  ir.Position `json:"call_site"`
	Certainty string      `json:"certainty"`
}

// UnresolvedValue is a place where the tracing stopped.
type UnresolvedValue struct {
	TerminalKind   string      `json:"terminal_kind"`
	AttributableTo string      `json:"attributable_to"`
	Chain          []string    `json:"chain"`
	Detail         string      `json:"detail,omitempty"`
	Candidates     []string    `json:"candidates,omitempty"`
	Position       ir.Position `json:"position"`
	Snippet        string      `json:"snippet,omitempty"`
}
