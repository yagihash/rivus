package taint

import (
	"github.com/yagihash/rivus/internal/ir"
	"github.com/yagihash/rivus/internal/sg"
)

// Terminal kinds recorded as uncertainty (ADR-008). Only the ones the
// analysis can currently observe are produced.
const (
	KindDynamicDispatch      = "dynamic_dispatch"
	KindUnmodeledLibraryCall = "unmodeled_library_call"
	// KindDependencyBodyUnread: a call of a method of an interface that has a model has a candidate whose
	// body is in the program but belongs to a dependency (ADR-075: the bodies of dependencies are not read; the
	// model says what they do). What the body does with the value, a call to an implementation of the
	// project's own (a wrapper that hands the value on), is not followed. It is rivus' limit, and it is
	// recorded beside dynamic_dispatch, which cannot tell this from a call with no unread candidate.
	KindDependencyBodyUnread = "dependency_body_unread"
	// KindProjectBodyUnread: a call of a function of the project whose body the extractor could not read (fn_skipped:
	// the compiler's interface stopped on it). Not a library call that has no model: the code is the project's, and what
	// it does with the value is not in the analysis (rivus' limit; the extractor says why in the detail).
	KindProjectBodyUnread    = "project_body_unread"
	KindUnresolvedCallee     = "unresolved_callee"
	KindContainerImprecision = "container_element_imprecision"
	KindDepthLimit           = "depth_limit"
	// KindAccessPathTruncated is on a hop where a value went through a path that was cut at the depth limit (-k, or
	// -dep-k in the functions of the dependencies): what is below the cut is taken to be tainted as well, so the
	// flow from there on may exist only because of the cut.
	KindAccessPathTruncated    = "access_path_truncated"
	KindRecursionApproximation = "recursion_approximation"
	// KindCharTransform is on a hop where a character read out of a text (a byte, a rune, a code unit), or a value made of
	// one, entered a number that taint stops at otherwise (-char-taint). The finding that goes through it may exist only
	// because the text was taken apart and put together again (more flows than there are, never fewer).
	KindCharTransform      = "char_transform"
	KindMissingSource      = "missing_source"
	KindReflection         = "reflection"
	KindConcurrency        = "concurrency"
	KindFFIBoundary        = "ffi_boundary"
	KindGlobalVar          = "global_var"
	KindConfigRead         = "config_read"
	KindExportedAPIParam   = "exported_api_param"
	KindStoredDataBoundary = "stored_data_boundary"
	// KindModelApproximation: a flow that a model says is an approximation (the flow's `approximate` text).
	KindModelApproximation = "model_approximation"
	// KindNumericStop marks the data of a source that became a number or a bool: they are
	// not followed (attributable to rivus: -numeric-taint follows them).
	KindNumericStop = "numeric_stop"
	// KindUnsupported marks a construct that the extractor could not express: what
	// it does is not in the analysis (attributable to rivus).
	KindUnsupported = "unsupported_construct"
	// KindAssumedCall marks a closure that is made here and called where it cannot be seen (given
	// to another function, stored, returned): what it captured is applied at the place it was made,
	// as if it were called (C-7, ADR-059).
	KindAssumedCall = "assumed_call"
	// KindFieldImprecision marks a sink that names a part of its value (a field below the argument) and was
	// reached by taint that is not known to be in that part: the value is tainted as a whole, the path was
	// cut, an element of a container, or the value went through a function that is not read. The detail
	// says why (whole_value, path_limit, element, opaque_callee). KindFieldUnresolved marks a sink whose
	// field names a name that the program does not have (a misspelling): the sink was taken whole.
	KindFieldImprecision = "field_imprecision"
	KindFieldUnresolved  = "field_unresolved"
	// KindNamedUnresolved marks a sink that names a named parameter (`named:`) that no call of the symbol in
	// the program has (a misspelling, or a parameter that was renamed): the sink was taken whole.
	KindNamedUnresolved = "named_unresolved"
	// KindPointerAlias marks a hop that goes through a write (or a read) via a pointer that is held in a cell, which
	// rivus approximates by aliasing (ptrAlias, ptrcells.go): the flow may not exist.
	KindPointerAlias   = "pointer_alias"
	AttributableTarget = "target"
	AttributableRivus  = "rivus"
	AttributableMixed  = "mixed"
	maxPathSteps       = 200
)

// attributable gives the attributable_to value of each kind
// (docs/ARCHITECTURE.md section 4.4).
func attributable(kind string) string {
	switch kind {
	case KindDynamicDispatch, KindUnresolvedCallee, KindConcurrency, KindAssumedCall:
		return AttributableMixed
	case KindMissingSource, KindReflection, KindFFIBoundary, KindGlobalVar, KindConfigRead, KindExportedAPIParam, KindStoredDataBoundary:
		return AttributableTarget
	default:
		return AttributableRivus
	}
}

// Uncertainty records a place where rivus could not know, and why.
type Uncertainty struct {
	Kind           string   `json:"kind"`
	AttributableTo string   `json:"attributable_to"`
	Detail         string   `json:"detail,omitempty"`
	Symbol         string   `json:"symbol,omitempty"`
	Candidates     []string `json:"candidates,omitempty"`
	// CoarsenedSCC is, on the scc_coarsened note of a group that is solved at depth 0 and on each hop that went through
	// a function of it, which group (ADR-124): so that a reader can tell the findings that went through one without
	// reading the wording.
	CoarsenedSCC *CoarsenedSCC `json:"coarsened_scc,omitempty"`
	// NarrowedFrom is, for a call through Fn::call (Rust) whose candidates were narrowed to the closures that
	// flow to its receiver, how many candidates there were before. Basis is the assumption the narrowing
	// rests on, when it rests on one ("closed_world_entry": the program was read as one that starts at its
	// entry point, so the callers that are not read are not counted).
	NarrowedFrom int         `json:"narrowed_from,omitempty"`
	Basis        string      `json:"basis,omitempty"`
	Func         string      `json:"func,omitempty"`
	Position     ir.Position `json:"position"`
	Snippet      string      `json:"snippet,omitempty"`
	// Recv and Operands are the type and the class of the receiver and of the arguments (receiver excluded) of a call that
	// has no model: what `rivus models coverage -yaml` needs to write a row that names the arguments a function may write
	// into (a pointer, a slice, a map, an interface) and leaves out the ones it cannot (a string, a number), and the
	// `context.Context`.
	Recv     *Operand  `json:"recv,omitempty"`
	Operands []Operand `json:"operands,omitempty"`

	pos ir.Pos
	// global and globalPath say which package variable (and which path inside it) a global_var
	// note is about: the evidence of a finding (Finding.SharedState) is made from them.
	global     *sg.Value
	globalPath string
}

// Operand is the type of an operand of a call, as the frontend wrote it, and its class.
type Operand struct {
	Type  ir.TypeID    `json:"type,omitempty"`
	Class ir.TypeClass `json:"class,omitempty"`
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
	// ViaSharedState is the mark that this source reached the sink through package variables (the places of
	// Finding.SharedState of the finding that this source would be, in the same order): the names of the variables. The
	// input of another entry point, carried through a shared state to this sink. Absent when the path goes through none.
	// Output only (ADR-069).
	ViaSharedState []string `json:"via_shared_state,omitempty"`
}

// Finding is the fact that data from a source site can reach a sink site.
type Finding struct {
	ID          string            `json:"id"`
	Fingerprint map[string]string `json:"fingerprint"`
	Source      SiteRef           `json:"source"`
	Sink        SiteRef           `json:"sink"`
	Category    string            `json:"category,omitempty"`
	// Categories are all the categories of the rows that name the sink with the same positions (Category is the first);
	// only when there are two or more.
	Categories []string `json:"categories,omitempty"`
	Labels     []string `json:"labels"`
	Paths      []Path   `json:"paths"`
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
	// ReachabilityNote explains not_from_entry (what the entry points leave out, ADR-052) and from_framework_callback (ADR-073).
	ReachabilityNote string `json:"reachability_note,omitempty"`
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
	// SharedState is the evidence for a flow that goes through a package variable: the flow
	// puts together what every writer of the variable wrote, and a reader of the finding cannot
	// tell a variable one function sets from one that a hundred do. Output only: it changes
	// neither the finding nor its path.
	SharedState []SharedState `json:"shared_state,omitempty"`
	// ThroughCoarsenedSCC are the IDs (CoarsenedSCC.ID, sorted) of the groups solved at depth 0 that a step of the
	// path went through (ADR-124). Absent when none: the findings without it come first in the output.
	ThroughCoarsenedSCC []string `json:"through_coarsened_scc,omitempty"`
}

// SharedState says who wrote the element of the finding to a shared place, and who reads it.
type SharedState struct {
	Place   SharedPlace   `json:"place"`
	Path    string        `json:"path,omitempty"`
	Writers SharedWriters `json:"writers"`
	// Readers is the number of functions that read or write the place (the ones the analysis
	// marks to be analysed again when it grows).
	Readers int `json:"readers"`
}

// SharedPlace is the place: a package variable ("global"; the name is the one the path's note has).
type SharedPlace struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// SharedWriters counts the functions that write to the place. Flow is the number that wrote an
// element of this finding's source (Flow <= All is what is expected: a function that passes on
// what a callee wrote is counted in Flow too, and the callee in All only when it stores to the
// variable itself); All is the number of functions that store to the variable directly; Test is
// how many of the Flow writers are in a file of a test (by its name: only evidence, nothing is left out).
type SharedWriters struct {
	Flow int `json:"flow"`
	All  int `json:"all"`
	Test int `json:"test,omitempty"`
}

// Values of Finding.Confidence.
const (
	ConfidenceClear  = "no_uncertainty_on_path"
	ConfidenceReview = "requires_review"
)

// Values of Finding.Reachability.
const (
	ReachFromEntry = "from_entry"
	// ReachFromFrameworkCallback: some function of the path is not reached from the entry points,
	// and every one that is not is reached from a method that a framework calls (an override of a
	// method that a dependency declares: State.initState, StatelessWidget.build), which the code
	// that was read does not call. The framework calls it where the program runs (ADR-073).
	ReachFromFrameworkCallback = "from_framework_callback"
	ReachNotFromEntry          = "not_from_entry"
)

// NotFromEntryNote is the explanation that goes with not_from_entry. The entry points are the
// start of the program as the code that was read says it, and the methods that a framework
// calls are those that override a method of a dependency's type (ReachFromFrameworkCallback).
// What is left out is what a framework is given as a value (a handler registered with a router,
// a function stored in a table) and that the code read does not pass on, and what the code
// of a dependency that was not read would call.
const NotFromEntryNote = "not_from_entry does not mean the code is dead: a function that a framework calls (a handler registered with it, a method that a dependency's code calls through a table) is not an entry point here, and the code of a dependency that was not read does not call it"

// FrameworkCallbackNote is the explanation that goes with from_framework_callback.
const FrameworkCallbackNote = "from_framework_callback: a function on the path is not reached from the entry points of the program, but it overrides a method that a dependency declares (State.initState, StatelessWidget.build), which the framework calls: the path starts where the framework calls, as the code of the dependency that was not read would"

// Path is one way the data gets from the source to the sink.
type Path struct {
	Steps []Step `json:"steps"`
	// OmittedSteps is how many steps were left out of the middle of the path (it kept the first and the last
	// maxPathSteps/2). The finding has a depth_limit note ("path_steps_omitted:") for it.
	OmittedSteps int `json:"omitted_steps,omitempty"`
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
