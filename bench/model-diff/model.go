package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/yagihash/rivus/internal/model"
)

// A model file as the tool needs it: the rows of one package (flows and sinks), the notes in the comments next to the rows,
// and the text of the file without the rows of the package (the run that reads the bodies).

// The kinds of the rows that the test reads.
const (
	RowFlow      = "flow"      // a propagators row with flows: compared with the body
	RowPure      = "pure"      // a propagators row with `pure: true`: the body must have no flow that the row loses
	RowSink      = "sink"      // a sinks row: compared with its declaration
	RowSanitizer = "sanitizer" // a sanitizers row: the body must carry the value that the row says it cleans, and the row's flows must be the body's
	RowSource    = "source"    // a sources row: the symbol, the field or the type must be in the package, and the positions must exist
)

// Row is one row of a model file that is checked.
type Row struct {
	Kind   string
	Symbol string            // for a source of the kind field or param_type: the field or the type
	Sink   *model.Sink       // a sinks row
	Prop   *model.Propagator // a propagators row (flow, pure)
	San    *model.Sanitizer
	Src    *model.Source
}

// Note is a `# model-diff: <kind> <symbol> <text>` comment.
type Note struct {
	Kind, Symbol, Text string
	File               string
	Line               int
}

var startRe = regexp.MustCompile(`^\s*#\s*model-diff:`)

var noteRe = regexp.MustCompile(`^\s*#\s*model-diff:\s*(\S+)\s+(\S+)\s*(.*?)\s*$`)

// ParseNotes collects the notes of a model file, line by line (not with the parser of the model: a comment is not data of the model).
// `miss-reviewed <symbol>@<position> <reason>` (for example `encoding/json.Unmarshal@arg1`): a MISS at that one position that was read
// against the body and is not a flow of data (the reading follows only the type of the value, or goes through a call that it cannot resolve).
// It is shown as `MISS-reviewed` and does not fail; the note is what records the reading, so it is for one position, never a whole function.
// `extra` needs a reason; `value <Type> <expression>` (Dart) says how to make a value of a class (the receiver, an argument): `$S` is where the source goes.
func ParseNotes(file, text string) ([]Note, []string) {
	var notes []Note
	var errs []string
	for i, l := range strings.Split(text, "\n") {
		m := noteRe.FindStringSubmatch(l)
		if m == nil {
			if startRe.MatchString(l) {
				errs = append(errs, fmt.Sprintf("%s:%d: a note is `# model-diff: <extra|value|miss-reviewed> <symbol> <text>`", file, i+1))
			}
			continue
		}
		n := Note{Kind: m[1], Symbol: m[2], Text: m[3], File: file, Line: i + 1}
		switch n.Kind {
		case "extra", "value", "miss-reviewed":
			if n.Kind == "miss-reviewed" {
				if e := checkReviewed(n); e != "" {
					errs = append(errs, fmt.Sprintf("%s:%d: %s", file, n.Line, e))
					continue
				}
			}
			if n.Text == "" {
				errs = append(errs, fmt.Sprintf("%s:%d: the note `%s %s` has no %s", file, n.Line, n.Kind, n.Symbol, map[string]string{"extra": "reason", "value": "expression", "miss-reviewed": "reason"}[n.Kind]))
				continue
			}
		default:
			errs = append(errs, fmt.Sprintf("%s:%d: unknown kind of note %q (extra, value, miss-reviewed)", file, n.Line, n.Kind))
			continue
		}
		notes = append(notes, n)
	}
	return notes, errs
}

// ReviewKinds are the kinds that the text of a `miss-reviewed` note starts with (the reason is one of these, then a short explanation):
//
//	type-only     not the value but only information of its type reaches the output (the error of encoding/json names the type of `v`)
//	non-payload   the flow of the body is real, but what arrives is of a fixed narrow shape (a number, a bool, a name of a few capital
//	              letters): it needs what it is narrowed to, then `src=<file of the standard library>.go:<function>` (docs/MODELING.md,
//	              "Stopping at a number, and a value of a narrow shape")
//	body-default  an apparent flow of the reading of the bodies: an unresolved call that it takes as "arguments to results" (it needs the
//	              number of the issue, `#N`, that is to make the reading better; the kind goes away when that is fixed)
var ReviewKinds = []string{"type-only", "non-payload", "body-default"}

// ReviewKind returns the kind that the note starts with ("" if it is none of the ReviewKinds) and the explanation after it.
func ReviewKind(n Note) (kind, text string) {
	f := strings.Fields(n.Text)
	if len(f) == 0 {
		return "", ""
	}
	for _, k := range ReviewKinds {
		if f[0] == k {
			return k, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(n.Text), k))
		}
	}
	return "", ""
}

var issueRe = regexp.MustCompile(`#[0-9]+`)

// srcRe is the place that shows a non-payload: `src=<path>.go` (then `:<function>`).
var srcRe = regexp.MustCompile(`\bsrc=\S+\.go\S*`)

func checkReviewed(n Note) string {
	if !strings.Contains(n.Symbol, "@") {
		return fmt.Sprintf("the note `miss-reviewed %s` needs a position: <symbol>@<position>", n.Symbol)
	}
	k, text := ReviewKind(n)
	switch {
	case k == "":
		return fmt.Sprintf("the note `miss-reviewed %s` starts with a kind (%s), then a short explanation", n.Symbol, strings.Join(ReviewKinds, ", "))
	case text == "":
		return fmt.Sprintf("the note `miss-reviewed %s` has a kind and no explanation", n.Symbol)
	case k == "non-payload" && (!srcRe.MatchString(text) || strings.TrimSpace(srcRe.ReplaceAllString(text, "")) == ""):
		return fmt.Sprintf("the note `miss-reviewed %s` of the kind non-payload needs what the value is narrowed to and where the source shows it (`src=<file>.go:<function>`)", n.Symbol)
	case k == "body-default" && !issueRe.MatchString(text):
		return fmt.Sprintf("the note `miss-reviewed %s` of the kind body-default needs the number of the issue (#N)", n.Symbol)
	}
	return ""
}

// LoadRows reads a model file and returns its rows of the package: flows rows, pure rows, sinks, sanitizers and sources (Dart: the
// flows rows and the sinks only: the other kinds are not checked there, and the caller says so).
func LoadRows(path, lang, pkg string) ([]Row, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var rs model.RuleSet
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, "", fmt.Errorf("%s: %v", path, err)
	}
	var rows []Row
	for i := range rs.Propagators {
		p := &rs.Propagators[i]
		if !model.MatchMajor(SymbolPackage(lang, p.Symbol), pkg) {
			continue
		}
		switch {
		case len(p.Flows) > 0:
			p.Symbol = concreteIn(p.Symbol, pkg)
			rows = append(rows, Row{Kind: RowFlow, Symbol: p.Symbol, Prop: p})
		case p.Pure:
			p.Symbol = concreteIn(p.Symbol, pkg)
			rows = append(rows, Row{Kind: RowPure, Symbol: p.Symbol, Prop: p})
		}
	}
	for i := range rs.Sinks {
		s := &rs.Sinks[i]
		if model.MatchMajor(SymbolPackage(lang, s.Symbol), pkg) {
			s.Symbol = concreteIn(s.Symbol, pkg)
			rows = append(rows, Row{Kind: RowSink, Symbol: s.Symbol, Sink: s})
		}
	}
	if lang == "go" {
		for i := range rs.Sanitizers {
			s := &rs.Sanitizers[i]
			if model.MatchMajor(SymbolPackage(lang, s.Symbol), pkg) {
				s.Symbol = concreteIn(s.Symbol, pkg)
				rows = append(rows, Row{Kind: RowSanitizer, Symbol: s.Symbol, San: s})
			}
		}
		for i := range rs.Sources {
			s := &rs.Sources[i]
			name := s.Symbol
			switch s.Kind {
			case model.KindField:
				name = s.Field
			case model.KindParamType:
				name = strings.TrimPrefix(s.Type, "*")
			}
			if name == "" || !model.MatchMajor(SymbolPackage(lang, name), pkg) {
				continue
			}
			name = concreteIn(name, pkg)
			s.Symbol, s.Field, s.Type = concreteIn(s.Symbol, pkg), concreteIn(s.Field, pkg), concreteIn(s.Type, pkg)
			rows = append(rows, Row{Kind: RowSource, Symbol: name, Src: s})
		}
	}
	return rows, string(data), nil
}

// concreteIn writes the symbol of a row for the major of the package that is checked: a row with /v* is for every major, and
// the generated code and the probes name the one function of this major.
func concreteIn(sym, pkg string) string {
	if !model.HasMajorWildcard(sym) {
		return sym
	}
	i := strings.Index(sym, model.MajorWildcard)
	// the package of the symbol up to the end of /v*, in the package that is checked, is /v and the digits
	start := strings.LastIndexAny(sym[:i], "()*[] ") + 1
	mod := sym[start:i]
	if !strings.HasPrefix(pkg, mod+"/v") {
		return sym
	}
	rest := pkg[len(mod)+2:]
	if k := strings.IndexByte(rest, '/'); k >= 0 {
		rest = rest[:k]
	}
	return model.ConcreteMajor(sym, rest)
}

// majorOfVersion is the major of a version (v93.0.0, 93.1.2): the digits before the first dot.
func majorOfVersion(v string) (string, bool) {
	v = strings.TrimPrefix(v, "v")
	n, _, _ := strings.Cut(v, ".")
	if n == "" || strings.Trim(n, "0123456789") != "" {
		return "", false
	}
	return n, true
}

var goPkgRe = regexp.MustCompile(`^(.*?)(\.v[0-9]+)?\.`)

// SymbolPackage is the package that a symbol of a model names ("" when it names none).
func SymbolPackage(lang, sym string) string {
	if lang == "dart" {
		if i := strings.Index(sym, "#"); strings.HasPrefix(sym, "package:") && i > 0 {
			name := sym[len("package:"):i]
			if j := strings.Index(name, "/"); j > 0 {
				name = name[:j]
			}
			return "package:" + name
		}
		return ""
	}
	s := strings.TrimPrefix(sym, "(")
	s = strings.TrimPrefix(s, "*")
	if i := strings.Index(s, ")"); i >= 0 {
		s = s[:i]
	}
	slash := strings.LastIndex(s, "/") + 1
	m := goPkgRe.FindStringSubmatchIndex(s[slash:])
	if m == nil {
		return ""
	}
	end := m[3] // the end of the name before ".vN" or before the dot
	if m[4] >= 0 {
		end = m[5]
	}
	return s[:slash+end]
}

var rowStartRe = regexp.MustCompile(`^  - `)

// DropPackage returns the text of a model file without the rows (of any section) whose symbol is of the package: the models
// of the others stay as they are. A row is the lines from its `  - ` to the next one or to the next key.
func DropPackage(text, lang, pkg string) string {
	lines := strings.Split(text, "\n")
	var out []string
	for i := 0; i < len(lines); {
		if !rowStartRe.MatchString(lines[i]) {
			out = append(out, lines[i])
			i++
			continue
		}
		j := i + 1
		for j < len(lines) && !rowStartRe.MatchString(lines[j]) && (lines[j] == "" || lines[j][0] == ' ' || lines[j][0] == '#') {
			j++
		}
		// the comment lines at the end of the block belong to the next row
		k := j
		for k > i+1 && (strings.TrimSpace(lines[k-1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[k-1]), "#") && !strings.HasPrefix(lines[k-1], "    ")) {
			k--
		}
		drop := false
		for _, l := range lines[i:k] {
			if m := symbolLineRe.FindStringSubmatch(l); m != nil {
				drop = model.MatchMajor(SymbolPackage(lang, strings.Trim(m[1], `"'`)), pkg)
				break
			}
		}
		if !drop {
			out = append(out, lines[i:k]...)
		}
		out = append(out, lines[k:j]...)
		i = j
	}
	return strings.Join(out, "\n")
}

var symbolLineRe = regexp.MustCompile(`^(?:  - |    )symbol:\s*(\S+)`)

// BuiltinFiles lists the built-in model files of a checkout.
func BuiltinFiles(repo string) ([]string, error) {
	fs, err := filepath.Glob(filepath.Join(repo, "internal", "model", "builtin", "*.yaml"))
	sort.Strings(fs)
	return fs, err
}
