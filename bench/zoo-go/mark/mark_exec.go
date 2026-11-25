//go:build zooexec

package mark

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// The marker and the judgement of "reached" are in bench/zoo/spec.json; the constants below are
// its copy, and spec_test.go holds the two to be the same.

// Class is the class of a sink.
type Class string

const (
	Any  Class = "Any"
	Cmd  Class = "Cmd"
	SQL  Class = "SQL"
	HTML Class = "HTML"
	Path Class = "Path"
)

var order = []Class{Cmd, SQL, HTML, Path}

type classSpec struct {
	segment string
	danger  [2]string
	set     string
}

var specs = map[Class]classSpec{
	Cmd:  {".SH", [2]string{";|$", "|$;"}, ";|$\n"},
	SQL:  {".SQL", [2]string{"'\"", "'`"}, "'\"\\`"},
	HTML: {".HTML", [2]string{"<>&", "<&>"}, "<>&"},
	Path: {".PATH", [2]string{"//../..", "/../../"}, "./"},
}

// Variant is the variant that this process runs (0 or 1): ZOO_VARIANT.
var Variant = func() int {
	if os.Getenv("ZOO_VARIANT") == "1" {
		return 1
	}
	return 0
}()

// Danger is the danger of the class in the variant that runs.
func Danger(c Class) string { return specs[c].danger[Variant] }

// Hit is one call of a sink.
type Hit struct {
	Case      string   `json:"case"`
	File      string   `json:"file"`
	Line      int      `json:"line"`
	Class     Class    `json:"class"`
	Seen      []string `json:"seen"`
	Reached   []string `json:"reached"`
	Srcs      []string `json:"srcs"`
	Recv      string   `json:"recv"`
	Masked    string   `json:"masked"`
	Ambiguous bool     `json:"ambiguous,omitempty"`
}

var (
	mu       sync.Mutex
	current  string
	caseSrcs = map[string][]string{}
	allTags  = map[string]bool{}
	hits     []Hit
	cases    []caseEntry
)

type caseEntry struct {
	id string
	fn func()
}

// Case registers a case.
func Case(id string, fn func()) {
	mu.Lock()
	defer mu.Unlock()
	cases = append(cases, caseEntry{id, fn})
}

// Run runs every registered case once, in the order of the registration (or the reverse), and
// returns the hits.
func Run(reverse bool) []Hit {
	mu.Lock()
	list := append([]caseEntry{}, cases...)
	mu.Unlock()
	if reverse {
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
	}
	for _, c := range list {
		mu.Lock()
		current = c.id
		mu.Unlock()
		c.fn()
	}
	mu.Lock()
	defer mu.Unlock()
	return append([]Hit{}, hits...)
}

// CaseIDs lists the registered cases.
func CaseIDs() []string {
	mu.Lock()
	defer mu.Unlock()
	var ids []string
	for _, c := range cases {
		ids = append(ids, c.id)
	}
	return ids
}

// Src returns the marker of the tag.
func Src(tag string) string {
	mu.Lock()
	allTags[tag] = true
	caseSrcs[current] = appendOnce(caseSrcs[current], tag)
	mu.Unlock()
	var b strings.Builder
	b.WriteString("Z" + tag)
	for _, c := range order {
		b.WriteString(specs[c].segment + specs[c].danger[Variant])
	}
	return b.String()
}

func appendOnce(l []string, s string) []string {
	for _, x := range l {
		if x == s {
			return l
		}
	}
	return append(l, s)
}

// SrcBytes is Src as bytes.
func SrcBytes(tag string) []byte { return []byte(Src(tag)) }

// NewIn makes the In of a tag.
func NewIn(tag string) In { return In{v: Src(tag)} }

func SinkCmd(s string)        { record(Cmd, "", []string{s}) }
func SinkCmdB(s string) bool  { record(Cmd, "", []string{s}); return false }
func SinkCmdBytes(b []byte)   { record(Cmd, "", []string{string(b)}) }
func SinkSQL(s string)        { record(SQL, "", []string{s}) }
func SinkHTML(s string)       { record(HTML, "", []string{s}) }
func SinkPath(base, s string) { record(Path, base, []string{s}) }

// SinkAny records every string that is in the value (fields, also the unexported ones, pointers,
// slices, arrays, maps with their keys, to a depth of 8).
func SinkAny(v any) {
	var strs []string
	walk(reflect.ValueOf(v), 8, map[uintptr]bool{}, &strs)
	record(Any, "", strs)
}

func walk(v reflect.Value, depth int, seen map[uintptr]bool, out *[]string) {
	if !v.IsValid() || depth < 0 {
		return
	}
	switch v.Kind() {
	case reflect.String:
		*out = append(*out, v.String())
	case reflect.Interface:
		walk(v.Elem(), depth-1, seen, out)
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		walk(v.Elem(), depth-1, seen, out)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			walk(v.Field(i), depth-1, seen, out)
		}
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			*out = append(*out, string(v.Bytes()))
			return
		}
		for i := 0; i < v.Len(); i++ {
			walk(v.Index(i), depth-1, seen, out)
		}
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, k := range keys {
			walk(k, depth-1, seen, out)
			walk(v.MapIndex(k), depth-1, seen, out)
		}
	}
}

// CleanX replaces the dangerous characters of the class by '_'.
func clean(c Class, s string) string {
	set := specs[c].set
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(set, r) {
			return '_'
		}
		return r
	}, s)
}

func CleanCmd(s string) string  { return clean(Cmd, s) }
func CleanSQL(s string) string  { return clean(SQL, s) }
func CleanHTML(s string) string { return clean(HTML, s) }
func CleanPath(s string) string { return clean(Path, s) }

// CleanResource: see mark.go.
func CleanResource(s string) string { return clean(Cmd, s) }

func record(class Class, base string, strs []string) {
	_, file, line, _ := runtime.Caller(2)
	// a skeleton of an external package (bench/zoo-go-pkg/<package>/skel) calls the sink for its function: the place is the
	// call of that function in the cell, the first frame of the stack that is in cells/
	for skip := 2; skip < 16; skip++ {
		_, f, l, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		if strings.Contains(f, "/cells/") {
			file, line = f, l
			break
		}
	}
	if i := strings.LastIndex(file, "/cells/"); i >= 0 {
		file = file[i+1:]
	} else {
		file = filepath.Base(file)
	}
	text := strings.Join(strs, "\x00")
	lower := strings.ToLower(text)
	mu.Lock()
	defer mu.Unlock()
	var seen []string
	for tag := range allTags {
		if strings.Contains(lower, "z"+tag+".") {
			seen = append(seen, tag)
		}
	}
	sort.Strings(seen)
	reached := false
	for _, s := range strs {
		if class == Any {
			for _, c := range order {
				reached = reached || Judge(c, "/zoo/base", s)
			}
		} else {
			reached = reached || Judge(class, base, s)
		}
	}
	// for the carry: the text without the dangers of the other classes (they differ between the
	// variants and are no part of what this sink is about); an Any sink has no carry
	masked := text
	for _, c := range order {
		if c != class {
			masked = strings.ReplaceAll(strings.ReplaceAll(masked, specs[c].danger[0], "#"), specs[c].danger[1], "#")
		}
	}
	if class == Any {
		masked = ""
	}
	h := Hit{Case: current, Masked: masked, File: file, Line: line, Class: class, Seen: seen, Srcs: append([]string{}, caseSrcs[current]...), Recv: text}
	if reached {
		switch {
		case len(seen) > 0:
			h.Reached = seen
		case len(caseSrcs[current]) == 1:
			h.Reached = append([]string{}, caseSrcs[current]...)
		default:
			h.Ambiguous = true
		}
	}
	hits = append(hits, h)
}

// Judge says whether the danger of the class, in the variant that runs, has reached a sink of
// the class in the text (the judgement of bench/zoo/spec.json).
func Judge(c Class, base, s string) bool {
	d := Danger(c)
	switch c {
	case Cmd:
		return judgeCmd(s, d)
	case SQL:
		for i := 0; ; {
			j := strings.Index(s[i:], d)
			if j < 0 {
				return false
			}
			j += i
			prev := byte(0)
			if j > 0 {
				prev = s[j-1]
			}
			next := byte(0)
			if j+1 < len(s) {
				next = s[j+1]
			}
			if prev != '\\' && prev != '\'' && next != '\'' {
				return true
			}
			i = j + 1
		}
	case HTML:
		return strings.Contains(s, d)
	case Path:
		if !strings.Contains(s, "..") {
			return false
		}
		rel, err := filepath.Rel(base, filepath.Join(base, s))
		return err != nil || rel == ".." || strings.HasPrefix(rel, "../")
	}
	return false
}

func judgeCmd(s, d string) bool {
	quote := byte(0) // 0, '\'' or '"'
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote == 0 && strings.HasPrefix(s[i:], d):
			return true
		case c == '\\' && quote != '\'':
			i++
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		case quote != 0 && c == quote:
			quote = 0
		}
	}
	return false
}

// SinkStmt runs the statement: the SQL of it is the sink, the Params are not.
func SinkStmt(s *Stmt) { record(SQL, "", []string{s.SQL}) }

// SinkStmtAny is SinkStmt for a statement that is held by an interface.
func SinkStmtAny(v any) { record(SQL, "", []string{v.(*Stmt).SQL}) }

// SinkTask sends the request of the task: the URL of the HTTP request is the sink.
func SinkTask(r *CreateTask) {
	u := ""
	if r.Task != nil {
		if w, ok := r.Task.MessageType.(*TaskHTTPRequest); ok && w.HTTPRequest != nil {
			u = w.HTTPRequest.URL
		}
	}
	record(Cmd, "", []string{u})
}

// Opaque moves the "body" parameter of a statement to its SQL: what a function that is not read may do
// to a structure that it is given.
func Opaque(s *Stmt) {
	if b, ok := s.Params["body"]; ok {
		s.SQL = b
	}
}
