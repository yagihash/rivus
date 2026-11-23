package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/fingerprint"
	"github.com/yagihash/rivus/internal/sites"
	"github.com/yagihash/rivus/internal/taint"
	"github.com/yagihash/rivus/internal/vcs"
)

// ResolveSchemaVersion identifies the format of ResolveResult.
const ResolveSchemaVersion = "rivus.resolve/1"

// Bookmark is what an agent keeps of a place it has looked at: a site, or a
// finding, exactly as rivus printed it. Nothing else is needed: the
// fingerprints are the memory.
type Bookmark struct {
	ID          string            `json:"id"`
	Role        string            `json:"role,omitempty"`
	Symbol      string            `json:"symbol,omitempty"`
	Func        string            `json:"func,omitempty"`
	Snippet     string            `json:"snippet,omitempty"`
	Fingerprint map[string]string `json:"fingerprint,omitempty"`
	// Revision is the code the bookmark was taken from: the "target" of the
	// output it was copied from. Optional; see ReadBookmarks.
	Revision *vcs.Target `json:"revision,omitempty"`

	// Source and Sink are set for a finding.
	Source *Bookmark `json:"source,omitempty"`
	Sink   *Bookmark `json:"sink,omitempty"`
}

// IsFinding reports whether the bookmark is a finding.
func (b *Bookmark) IsFinding() bool { return b.Source != nil && b.Sink != nil }

// ReadBookmarks reads a JSON array of bookmarks, or an object with a
// "bookmarks" array, or a single bookmark.
func ReadBookmarks(path string) ([]*Bookmark, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	var list []*Bookmark
	if err := json.Unmarshal(data, &list); err == nil {
		return list, validate(list)
	}
	var wrapped struct {
		// Target is the "target" of the output the bookmarks were copied
		// from; it is the revision of every bookmark that has none of its own.
		Target    *vcs.Target `json:"target"`
		Bookmarks []*Bookmark `json:"bookmarks"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Bookmarks != nil {
		for _, b := range wrapped.Bookmarks {
			if b != nil && b.Revision == nil {
				b.Revision = wrapped.Target
			}
		}
		return wrapped.Bookmarks, validate(wrapped.Bookmarks)
	}
	var one Bookmark
	if err := json.Unmarshal(data, &one); err != nil {
		return nil, fmt.Errorf("bookmarks: %w", err)
	}
	return []*Bookmark{&one}, validate([]*Bookmark{&one})
}

func validate(list []*Bookmark) error {
	for i, b := range list {
		if b == nil || b.ID == "" {
			return fmt.Errorf("bookmark %d has no id", i)
		}
		if len(b.Fingerprint) == 0 && !b.IsFinding() {
			return fmt.Errorf("bookmark %s has no fingerprint: pass the site or finding exactly as rivus printed it", b.ID)
		}
	}
	return nil
}

func revisionChange(from, to *vcs.Target) string {
	switch {
	case from == nil || to == nil:
		return "unknown"
	case from.Revision == to.Revision:
		return "same"
	}
	return "different"
}

// Statuses of a resolved bookmark.
const (
	StatusUnchanged  = "unchanged"
	StatusChanged    = "changed"
	StatusRematched  = "rematched"
	StatusAmbiguous  = "ambiguous"
	StatusUnresolved = "unresolved"
)

// Reasons a bookmark is unresolved.
const (
	ReasonNotFound    = "not_found"    // the code was analysed and the place is not there
	ReasonNotAnalysed = "not_analyzed" // the package was not analysed (not loaded, or failed to load)
)

// Flow states of a finding bookmark.
const (
	FlowPresent      = "present"
	FlowAbsentNow    = "absent_now"
	FlowNotEvaluated = "not_evaluated"
)

// SiteResult is the resolution of one site.
type SiteResult struct {
	Bookmark string `json:"bookmark"`
	Status   string `json:"status"`
	// Current is the site as it is now, with its new ID.
	Current *taint.SiteRef `json:"current,omitempty"`
	// MatchBasis is the fingerprint key that found the site.
	MatchBasis string `json:"match_basis,omitempty"`
	// ChangedKeys are the change detection keys that differ.
	ChangedKeys []string `json:"changed_keys,omitempty"`
	// PreviousSnippet is what the bookmark remembers, to put next to the
	// current snippet when the match is not certain.
	PreviousSnippet string          `json:"previous_snippet,omitempty"`
	Candidates      []taint.SiteRef `json:"candidates,omitempty"`
	Reason          string          `json:"reason,omitempty"`
	Evidence        []string        `json:"evidence,omitempty"`
}

// Flow says whether the data still gets from the source to the sink.
type Flow struct {
	Status string `json:"status"`
	// PathChanged is true when the finding exists but its path is another.
	PathChanged    bool   `json:"path_changed,omitempty"`
	CurrentFinding string `json:"current_finding,omitempty"`
	// Findings that share the current source or the current sink, to look at
	// when the path is gone: the flow may have moved.
	RelatedFindings []string `json:"related_findings,omitempty"`
	// Uncertainties are why rivus may have missed the flow (absent_now):
	// those in the two functions and those that concern the whole run. This
	// is not a verdict that the code is safe.
	Uncertainties []*taint.Uncertainty `json:"uncertainties,omitempty"`
}

// Resolution is the answer for one bookmark.
type Resolution struct {
	Bookmark string `json:"bookmark"`
	Kind     string `json:"kind"` // site or finding
	Status   string `json:"status"`
	// FromRevision is the code the bookmark was taken from, when known.
	FromRevision *vcs.Target `json:"from_revision,omitempty"`
	// RevisionChange says how that code relates to the code now: "same"
	// (the same commit; the working trees are not compared, ADR-038 addendum), "different", or "unknown" (one
	// of the two is not known).
	RevisionChange string `json:"revision_change"`
	SiteResult
	Source *SiteResult `json:"source,omitempty"`
	Sink   *SiteResult `json:"sink,omitempty"`
	Flow   *Flow       `json:"flow,omitempty"`
}

// ResolveResult answers a set of bookmarks.
type ResolveResult struct {
	SchemaVersion string `json:"schema_version"`
	// Target is the code the bookmarks were resolved against.
	Target  *vcs.Target   `json:"target,omitempty"`
	Results []*Resolution `json:"results"`
	// Incomplete and IncompleteReasons are those of the analysis of the current code (ADR-120): an "unresolved" or an
	// "absent_now" of an analysis that did not finish may be "not looked for". Always in the output.
	Incomplete        bool        `json:"incomplete"`
	IncompleteReasons []string    `json:"incomplete_reasons"`
	Stats             taint.Stats `json:"stats"`
}

// Resolve finds where the bookmarks are in the code this report describes.
//
// The identity keys are tried in order of trust: the content key first, then
// the structural key (ADR-028). The result states how a place was found and
// does not judge whether it is the same place; when the match rests on the
// structural key alone, both snippets are returned so that a reader can see.
func (r *Report) Resolve(bms []*Bookmark) *ResolveResult {
	idx := newSiteIndex(r)
	inc, reasons := r.IncompleteState()
	out := &ResolveResult{SchemaVersion: ResolveSchemaVersion, Target: r.Target, Stats: r.Stats, Incomplete: inc, IncompleteReasons: reasons, Results: []*Resolution{}}
	for _, b := range bms {
		res := r.resolveOne(idx, b)
		res.FromRevision = b.Revision
		res.RevisionChange = revisionChange(b.Revision, r.Target)
		out.Results = append(out.Results, res)
	}
	return out
}

type siteIndex struct {
	byContent    map[string][]*sites.Site
	byStructural map[string][]*sites.Site
}

func newSiteIndex(r *Report) *siteIndex {
	idx := &siteIndex{byContent: map[string][]*sites.Site{}, byStructural: map[string][]*sites.Site{}}
	for _, s := range r.Sites {
		if c := s.Fingerprint[fingerprint.Content]; c != "" {
			idx.byContent[c] = append(idx.byContent[c], s)
		}
		if c := s.Fingerprint[fingerprint.Structural]; c != "" {
			idx.byStructural[c] = append(idx.byStructural[c], s)
		}
	}
	return idx
}

func (r *Report) resolveOne(idx *siteIndex, b *Bookmark) *Resolution {
	if !b.IsFinding() {
		sr := r.resolveSite(idx, b)
		return &Resolution{Bookmark: b.ID, Kind: "site", Status: sr.Status, SiteResult: *sr}
	}
	src := r.resolveSite(idx, b.Source)
	snk := r.resolveSite(idx, b.Sink)
	res := &Resolution{Bookmark: b.ID, Kind: "finding", Source: src, Sink: snk}
	res.SiteResult.Bookmark = b.ID
	res.Flow = r.flow(b, src, snk)

	switch {
	case src.Status == StatusUnresolved || snk.Status == StatusUnresolved:
		res.Status = StatusUnresolved
		res.Reason = src.Reason
		if res.Reason == "" || snk.Reason == ReasonNotAnalysed {
			res.Reason = snk.Reason
		}
	case src.Status == StatusAmbiguous || snk.Status == StatusAmbiguous:
		res.Status = StatusAmbiguous
	case src.Status == StatusRematched || snk.Status == StatusRematched:
		res.Status = StatusRematched
	case src.Status == StatusChanged || snk.Status == StatusChanged || res.Flow.PathChanged:
		res.Status = StatusChanged
	default:
		res.Status = StatusUnchanged
	}
	res.SiteResult.Status = res.Status
	return res
}

func (r *Report) resolveSite(idx *siteIndex, b *Bookmark) *SiteResult {
	res := &SiteResult{Bookmark: b.ID, PreviousSnippet: b.Snippet}
	content := idx.byContent[b.Fingerprint[fingerprint.Content]]
	structural := idx.byStructural[b.Fingerprint[fingerprint.Structural]]
	if b.Fingerprint[fingerprint.Content] == "" {
		content = nil
	}
	if b.Fingerprint[fingerprint.Structural] == "" {
		structural = nil
	}

	var found *sites.Site
	switch {
	case len(content) == 1:
		found, res.MatchBasis = content[0], fingerprint.Content
	case len(content) > 1:
		res.Status = StatusAmbiguous
		res.MatchBasis = fingerprint.Content
		res.Candidates = refs(content)
		return res
	case len(structural) == 1:
		found, res.MatchBasis = structural[0], fingerprint.Structural
	case len(structural) > 1:
		res.Status = StatusAmbiguous
		res.MatchBasis = fingerprint.Structural
		res.Candidates = refs(structural)
		return res
	default:
		res.Status = StatusUnresolved
		res.Reason = r.whyMissing(b)
		return res
	}

	cur := ref(found)
	res.Current = &cur
	res.ChangedKeys = changedKeys(b.Fingerprint, found.Fingerprint)
	sameID := found.ID == b.ID
	switch {
	case res.MatchBasis == fingerprint.Structural:
		// The expression is not what it was (or a similar call came
		// before it): the position among the calls to the symbol is all
		// that connects the two.
		res.Status = StatusRematched
		res.Evidence = append(res.Evidence,
			"the content fingerprint did not match: the call expression changed, or a similar call was added or removed before it; compare the snippets")
	case !sameID:
		res.Status = StatusRematched
		res.Evidence = append(res.Evidence,
			"the same call expression is now at another position among the calls to "+b.Symbol+" in the function; the ID changed")
	case len(res.ChangedKeys) > 0:
		res.Status = StatusChanged
	default:
		res.Status = StatusUnchanged
	}
	return res
}

// changedKeys compares the change detection keys present on both sides.
func changedKeys(old, cur map[string]string) []string {
	var out []string
	for _, k := range []string{fingerprint.Expr, fingerprint.Body} {
		if ov, ok := old[k]; ok {
			if cv, ok := cur[k]; ok && ov != cv {
				out = append(out, k)
			}
		}
	}
	return out
}

// whyMissing tells a place that is gone from a package that was not looked at.
func (r *Report) whyMissing(b *Bookmark) string {
	pkg := UnitOf(b.Func)
	for _, le := range r.LoadErrors {
		if le.Package == pkg {
			return ReasonNotAnalysed
		}
	}
	if pkg != "" {
		analysed := false
		for _, p := range r.Packages {
			if p == pkg {
				analysed = true
			}
		}
		if !analysed {
			return ReasonNotAnalysed
		}
	}
	return ReasonNotFound
}

// UnitOf is what a function ID says about the unit of code it belongs to: the
// package of a Go function, the package of a Dart library ("package:name"; the
// files of the project itself are one unit, "file:"), the crate of a Rust
// function. A report lists the units that were analysed (packages), so that a
// place that is gone can be told from one that was not looked at.
func UnitOf(funcID string) string { return taint.UnitOf(funcID) }

// packageOf extracts the import path from a function ID such as
// "example.com/m/pkg.F", "(*example.com/m/pkg.T).M" or "example.com/m/pkg.F$1".
func packageOf(funcID string) string {
	s := funcID
	if strings.HasPrefix(s, "(") {
		if end := strings.Index(s, ")"); end > 0 {
			s = s[1:end]
		}
	}
	s = strings.TrimPrefix(s, "*")
	if i := strings.IndexAny(s, "$["); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "."); i > 0 {
		return s[:i]
	}
	return ""
}

func (r *Report) flow(b *Bookmark, src, snk *SiteResult) *Flow {
	f := &Flow{Status: FlowNotEvaluated}
	if src.Current == nil || snk.Current == nil {
		return f
	}
	var related []string
	for _, fi := range r.Findings {
		sameSrc := fi.Source.ID == src.Current.ID
		sameSnk := fi.Sink.ID == snk.Current.ID
		if sameSrc && sameSnk {
			f.Status = FlowPresent
			f.CurrentFinding = fi.ID
			if old := b.Fingerprint[fingerprint.Path]; old != "" && fi.Fingerprint[fingerprint.Path] != "" && old != fi.Fingerprint[fingerprint.Path] {
				f.PathChanged = true
			}
			continue
		}
		if sameSrc || sameSnk {
			related = append(related, fi.ID)
		}
	}
	sort.Strings(related)
	f.RelatedFindings = related
	if f.Status == FlowPresent {
		return f
	}
	// Not found: say what could have hidden it. This is a fact about what
	// rivus could not follow, not a statement that the flow cannot happen.
	f.Status = FlowAbsentNow
	for _, u := range r.Uncertainties {
		if u.Func == src.Current.Func || u.Func == snk.Current.Func || u.Func == "" {
			f.Uncertainties = append(f.Uncertainties, u)
			if len(f.Uncertainties) >= 20 {
				break
			}
		}
	}
	return f
}

func ref(s *sites.Site) taint.SiteRef {
	return taint.SiteRef{ID: s.ID, Symbol: s.Symbol, Func: s.Func, Position: s.Position,
		Snippet: s.Snippet, Fingerprint: s.Fingerprint}
}

func refs(ss []*sites.Site) []taint.SiteRef {
	out := make([]taint.SiteRef, 0, len(ss))
	for _, s := range ss {
		out = append(out, ref(s))
	}
	return out
}

// ForeignID reports whether a function ID is that of a language other than Go.
func ForeignID(funcID string) bool {
	return strings.HasPrefix(funcID, "package:") || strings.HasPrefix(funcID, "file:") || strings.Contains(funcID, "::")
}
