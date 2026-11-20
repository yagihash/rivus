package graphq

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/yagihash/rivus/internal/graphdb"
)

// Target is what a specification names: functions, or one call site (or, for
// "file:line" with several calls on it, the calls to choose from).
type Target struct {
	Funcs []int64
	// Call is set when the specification names a call: the function that
	// holds it is the one callers asks about, and the functions it reaches
	// are the ones callees lists.
	Call *graphdb.CallRow
	// Same are the other calls at the very same place of the same function (a
	// desugared "a()?" is two): they are one call to a reader, and callees
	// gives what all of them reach.
	Same      []graphdb.CallRow
	Ambiguous []graphdb.CallRow
	// Desc says what the specification was taken for.
	Desc string
	// Builtin is the number of calls of a builtin that the specification
	// named and the answer leaves out.
	Builtin int
}

// SpecError is a specification that names nothing, with what is near it.
type SpecError struct {
	Spec       string
	Msg        string
	Candidates []string
}

func (e *SpecError) Error() string {
	s := fmt.Sprintf("%s: %s", e.Spec, e.Msg)
	if len(e.Candidates) > 0 {
		s += "\nnear it:\n  " + strings.Join(e.Candidates, "\n  ")
	}
	return s
}

var (
	posRe  = regexp.MustCompile(`^(.+?):(\d+)(?::(\d+))?$`)
	siteRe = regexp.MustCompile(`^s\d+:[0-9a-f]+$`)
)

// Resolve finds what a specification names. It is one of four: a symbol
// ("os/exec.Command", "(*pkg.T).Method"), file:line:col (the call that begins
// there), file:line (the calls on the line), or site:<ID> (a site of "rivus
// sites"; "--site ID" is the same). A path of a file is the one the graph
// has (relative to the analysed directory), or an absolute path, or the end of
// one.
func (e *Engine) Resolve(spec string, includeBuiltin bool) (*Target, error) {
	if spec == "" {
		return nil, &SpecError{Spec: spec, Msg: "the target is empty"}
	}
	if id, ok := strings.CutPrefix(spec, "site:"); ok {
		return e.resolveSite(spec, id, includeBuiltin)
	}
	if siteRe.MatchString(spec) {
		return e.resolveSite(spec, spec, includeBuiltin)
	}
	if m := posRe.FindStringSubmatch(spec); m != nil && !strings.HasSuffix(m[1], ":") {
		line, _ := strconv.Atoi(m[2])
		col := 0
		if m[3] != "" {
			col, _ = strconv.Atoi(m[3])
		}
		file, near, err := e.db.ResolveFile(m[1])
		if err != nil {
			return nil, err
		}
		if file != "" {
			return e.resolvePos(spec, file, line, col, includeBuiltin)
		}
		// not a file of the graph: perhaps a symbol that looks like one
		if t, err := e.resolveSymbol(spec); err == nil {
			return t, nil
		}
		return nil, &SpecError{Spec: spec, Msg: fmt.Sprintf("no file %q in the graph (the files are named as in the output of \"rivus sites\": relative to the analysed directory)", m[1]), Candidates: near}
	}
	return e.resolveSymbol(spec)
}

func (e *Engine) resolveSite(spec, id string, includeBuiltin bool) (*Target, error) {
	s, ok, err := e.db.SiteByID(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &SpecError{Spec: spec, Msg: fmt.Sprintf("no site %q in the graph (\"rivus sites\" lists them; the IDs change with the rules)", id)}
	}
	if s.CallSite > 0 {
		c, err := e.db.CallRowByID(s.CallSite)
		if err != nil {
			return nil, err
		}
		return e.callTarget(c, fmt.Sprintf("call site of site %s", id), includeBuiltin)
	}
	f, err := e.db.FuncRowByFID(s.Func)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, &SpecError{Spec: spec, Msg: "the site is not on a call and has no function"}
	}
	e.funcs[f.ID] = f
	return &Target{Funcs: []int64{f.ID}, Desc: fmt.Sprintf("function %s (the site %s is not a call: it is taken as the function that holds it)", f.FID, id)}, nil
}

func (e *Engine) callTarget(c *graphdb.CallRow, what string, includeBuiltin bool) (*Target, error) {
	t := &Target{Call: c, Desc: what}
	if c.Kind == graphdb.KindBuiltin && !includeBuiltin {
		return nil, &SpecError{Spec: pos(c.File, c.Line, c.Col), Msg: fmt.Sprintf("the call is a builtin (%s): builtins are left out; --include-builtin keeps them", c.Callee)}
	}
	f, err := e.fn(c.Func)
	if err != nil {
		return nil, err
	}
	t.Funcs = []int64{f.ID}
	return t, nil
}

func (e *Engine) resolvePos(spec, file string, line, col int, includeBuiltin bool) (*Target, error) {
	rows, err := e.db.CallRowsAt(file, line, col)
	if err != nil {
		return nil, err
	}
	note := ""
	if col > 0 && len(rows) == 0 {
		if rows, err = e.db.CallRowsBefore(file, line, col); err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			note = fmt.Sprintf("; no call begins at column %d, the call that begins at column %d is taken", col, rows[0].Col)
		}
	}
	var keep []graphdb.CallRow
	builtin := 0
	for _, r := range rows {
		if r.Kind == graphdb.KindBuiltin && !includeBuiltin {
			builtin++
			continue
		}
		keep = append(keep, r)
	}
	// calls at one place of one function are one choice
	var groups [][]graphdb.CallRow
	for _, r := range keep {
		if n := len(groups); n > 0 && groups[n-1][0].Func == r.Func && groups[n-1][0].Line == r.Line && groups[n-1][0].Col == r.Col {
			groups[n-1] = append(groups[n-1], r)
			continue
		}
		groups = append(groups, []graphdb.CallRow{r})
	}
	switch len(groups) {
	case 0:
		if builtin > 0 {
			return nil, &SpecError{Spec: spec, Msg: fmt.Sprintf("the only calls there are builtins (%d): they are left out; --include-builtin keeps them", builtin)}
		}
		se := &SpecError{Spec: spec, Msg: fmt.Sprintf("no call at %s:%d (the graph has the calls the program makes; a line with none, a declaration for one, is not one)", file, line)}
		if f, err := e.db.EnclosingFunc(file, line); err == nil && f != nil {
			se.Candidates = []string{fmt.Sprintf("the function that begins last before the line: %s (%s:%d)", f.FID, file, f.Line)}
		}
		return nil, se
	case 1:
		g := groups[0]
		t, err := e.callTarget(&g[0], fmt.Sprintf("call site %s%s", pos(g[0].File, g[0].Line, g[0].Col), note), includeBuiltin)
		if t != nil {
			t.Builtin = builtin
			t.Same = g[1:]
		}
		return t, err
	}
	var firsts []graphdb.CallRow
	for _, g := range groups {
		firsts = append(firsts, g[0])
	}
	return &Target{Ambiguous: firsts, Builtin: builtin, Desc: fmt.Sprintf("%d calls on %s:%d", len(groups), file, line)}, nil
}

func (e *Engine) resolveSymbol(spec string) (*Target, error) {
	fs, err := e.db.FuncsNamed(spec)
	if err != nil {
		return nil, err
	}
	desc := "function " + spec
	if len(fs) == 0 {
		// the package path left out in front: taken when only one function has it
		cands, err := e.db.FuncsEndingWith(spec, 6)
		if err != nil {
			return nil, err
		}
		if len(cands) == 1 {
			fs = cands
			desc = fmt.Sprintf("function %s (the name %q is the end of it)", cands[0].FID, spec)
		} else if len(cands) > 1 {
			se := &SpecError{Spec: spec, Msg: "no function with that name; several end with it"}
			for _, c := range cands {
				se.Candidates = append(se.Candidates, c.FID)
			}
			return nil, se
		}
	}
	if len(fs) == 0 {
		se := &SpecError{Spec: spec, Msg: "no function with that symbol in the graph"}
		if sim, err := e.db.SimilarSymbols(spec, 8); err == nil {
			se.Candidates = sim
		}
		return nil, se
	}
	t := &Target{Desc: desc}
	for i := range fs {
		e.funcs[fs[i].ID] = &fs[i]
		t.Funcs = append(t.Funcs, fs[i].ID)
	}
	if len(fs) > 1 {
		t.Desc += fmt.Sprintf(" (%d functions have this symbol: a method and its wrappers)", len(fs))
	}
	return t, nil
}

func (e *Engine) ambiguous(rows []graphdb.CallRow) ([]CallSiteOut, error) {
	var out []CallSiteOut
	for i := range rows {
		c, err := e.callSiteOut(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
