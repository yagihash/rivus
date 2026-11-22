package main

import (
	"fmt"
	"go/types"
	"sort"
	"strings"
	"unicode"
)

// The rule table of layer 1 (docs/DESIGN-package-models.md, 6.11): which candidates need a decision (a row, or an entry of `rejected:`).
// It reads the types of the package, and nothing else; the same package gives the same list. A rule is generic: it names a shape
// of a signature, never a package. A rule is added here, in a pull request, when a row of a model is found that no rule finds
// (`-recall`).

// Candidate kinds (the kinds of `rejected:`).
const (
	KindSink      = "sink"
	KindSource    = "source"
	KindSanitizer = "sanitizer"
	KindCalls     = "calls"
)

// Item is one candidate: a function, a method or a field, and what it may be.
type Item struct {
	Symbol string `json:"symbol"`
	Kind   string `json:"kind"`
	// Positions say where: arg0, arg1 (the receiver is not counted, as in `args:` of a row), arg1.Field, result, field.
	Positions []string `json:"positions"`
	// Rule is the name of the rule(s) that found it.
	Rule string `json:"rule"`
	// Sig is the signature, for the person who decides.
	Sig string `json:"sig"`
}

// word lists. A name is split into words at an upper-case letter, a digit and `_` (`bucketName` is bucket, name), and a rule fires when a
// word is in the list. The lists are the whole of the vocabulary of the table.
var (
	sinkWords = map[string][]string{
		"sql_injection":     {"query", "sql", "stmt", "statement"},
		"command_injection": {"cmd", "command", "script", "shell"},
		"path_traversal":    {"path", "oldpath", "newpath", "file", "filename", "filepath", "dir", "dirname", "directory", "folder", "dst", "fname", "glob"},
		"ssrf":              {"url", "uri", "href", "endpoint", "host", "hostname", "addr", "address", "domain", "baseurl"},
		"open_redirect":     {"location", "redirect"},
		"xss":               {"html", "markup", "template", "tmpl", "format"},
		"resource_name": {"bucket", "object", "owner", "repo", "repository", "project", "topic", "subscription", "collection",
			"table", "dataset", "queue", "prefix", "container", "blob", "organization", "org", "namespace"},
	}
	// bytesSinkWords are the words that make an argument of bytes (not of a string) a candidate: what is written as it is.
	bytesSinkWords = []string{"html", "markup", "data", "content", "body"}
	// resourceNouns are the names of a function whose `name` or `id` argument names a resource (`Client.Bucket(name)`).
	resourceNouns = []string{"bucket", "object", "topic", "subscription", "collection", "doc", "document", "table", "dataset", "queue", "container", "blob", "project", "repo", "repository", "org", "organization"}
	// fileVerbs are the words of a function that works on a file whose name is given as `name` (`os.Open(name)`, `os.Rename(oldname, newname)`).
	fileVerbs = []string{"open", "remove", "mkdir", "rename", "chmod", "chown", "lchown", "chtimes", "truncate", "symlink", "link", "readlink", "stat", "lstat", "chdir", "file", "dir"}
	// sourceWords are the words of a function or a field that carries what somebody else wrote: a body, a content, a header.
	sourceWords    = []string{"body", "content", "contents", "payload", "raw", "text", "data", "reader", "file", "files", "message", "header", "headers", "cookie", "cookies", "query", "param", "params", "form", "input", "request", "response"}
	sanitizerWords = []string{"escape", "quote", "sanitize", "sanitise", "clean"}
)

// words splits an identifier into lower-case words.
func words(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r == '_' || unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r):
			// a new word at an upper-case letter that follows a lower-case one, or that starts a word after an acronym (URLPath -> url, path)
			if i > 0 && (unicode.IsLower(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1]))) {
				flush()
			}
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return out
}

func hasWord(name string, list []string) bool {
	for _, w := range words(name) {
		for _, x := range list {
			if w == x {
				return true
			}
		}
	}
	return false
}

// categoryOf returns the first category (in the order of categoryOrder) one word of name belongs to.
func categoryOf(name string) string {
	for _, c := range categoryOrder {
		if hasWord(name, sinkWords[c]) {
			return c
		}
	}
	return ""
}

var categoryOrder = []string{"sql_injection", "command_injection", "path_traversal", "ssrf", "open_redirect", "xss", "resource_name"}

func isString(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

func isBytes(t types.Type) bool {
	s, ok := t.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	b, ok := s.Elem().Underlying().(*types.Basic)
	return ok && b.Kind() == types.Byte
}

func isStrings(t types.Type) bool {
	s, ok := t.Underlying().(*types.Slice)
	return ok && isString(s.Elem())
}

// isReader: io.Reader or something that has its Read method with the same shape and is named a reader of data.
func isReader(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "io" {
		switch n.Obj().Name() {
		case "Reader", "ReadCloser", "ReadSeeker", "ReadSeekCloser", "ReaderAt":
			return true
		}
	}
	return false
}

func isFuncOrOwnInterface(t types.Type, self *types.Package) bool {
	switch u := t.Underlying().(type) {
	case *types.Signature:
		return true
	case *types.Slice:
		return isFuncOrOwnInterface(u.Elem(), self)
	case *types.Interface:
		n, ok := t.(*types.Named)
		return ok && n.Obj().Pkg() == self && u.NumMethods() > 0 && n.Obj().Exported()
	}
	return false
}

// API is what the loader reads: every exported function, method (declared on a type of the package) and field.
type API struct {
	Pkg     string
	Symbols map[string]string // symbol -> signature (all of them: the stale check reads it)
	Items   []Item
	// Layer2 are the functions that take a string, bytes or `any` and are in no item.
	Layer2 []string
}

type funcInfo struct {
	symbol string
	name   string
	fn     *types.Func
	sig    *types.Signature
}

// Scan reads the exported API of the package and applies the rule table.
func Scan(pkg *types.Package) *API {
	api := &API{Pkg: pkg.Path(), Symbols: map[string]string{}}
	var fns []funcInfo
	var fields []fieldInfo
	scope := pkg.Scope()
	for _, n := range scope.Names() {
		o := scope.Lookup(n)
		if !o.Exported() {
			continue
		}
		switch o := o.(type) {
		case *types.Func:
			sig := o.Type().(*types.Signature)
			fns = append(fns, funcInfo{symbol: pkg.Path() + "." + n, name: n, fn: o, sig: sig})
		case *types.TypeName:
			if o.IsAlias() {
				continue
			}
			named, ok := o.Type().(*types.Named)
			if !ok {
				continue
			}
			for i := 0; i < named.NumMethods(); i++ {
				m := named.Method(i)
				if !m.Exported() {
					continue
				}
				sig := m.Type().(*types.Signature)
				recv := pkg.Path() + "." + n
				if _, ptr := sig.Recv().Type().(*types.Pointer); ptr {
					recv = "*" + recv
				}
				fns = append(fns, funcInfo{symbol: "(" + recv + ")." + m.Name(), name: m.Name(), fn: m, sig: sig})
			}
			if st, ok := named.Underlying().(*types.Struct); ok {
				for i := 0; i < st.NumFields(); i++ {
					f := st.Field(i)
					if f.Exported() && !f.Embedded() {
						fields = append(fields, fieldInfo{symbol: pkg.Path() + "." + n + "." + f.Name(), name: f.Name(), typ: f.Type()})
					}
				}
			}
		}
	}
	structs := ownStructs(pkg)
	taken := map[string]bool{}
	for _, f := range fns {
		api.Symbols[f.symbol] = f.sig.String()
		items := funcItems(pkg, f, structs)
		for _, it := range items {
			taken[f.symbol] = true
			api.Items = append(api.Items, it)
		}
	}
	for _, f := range fields {
		api.Symbols[f.symbol] = f.typ.String()
		if (isString(f.typ) || isBytes(f.typ) || isReader(f.typ)) && hasWord(f.name, sourceWords) {
			api.Items = append(api.Items, Item{Symbol: f.symbol, Kind: KindSource, Positions: []string{"field"}, Rule: "source: field that carries a body or content", Sig: "field " + f.typ.String()})
		}
	}
	for _, f := range fns {
		if taken[f.symbol] {
			continue
		}
		if takesData(f.sig) {
			api.Layer2 = append(api.Layer2, f.symbol+f.sig.String()[len("func"):])
		}
	}
	sort.SliceStable(api.Items, func(i, j int) bool {
		if api.Items[i].Symbol != api.Items[j].Symbol {
			return api.Items[i].Symbol < api.Items[j].Symbol
		}
		return api.Items[i].Kind < api.Items[j].Kind
	})
	sort.Strings(api.Layer2)
	return api
}

type fieldInfo struct {
	symbol, name string
	typ          types.Type
}

// ownStructs: the structs of the package by name, for the rule of an argument that is a struct with a field that names something.
func ownStructs(pkg *types.Package) map[*types.TypeName]*types.Struct {
	out := map[*types.TypeName]*types.Struct{}
	for _, n := range pkg.Scope().Names() {
		if tn, ok := pkg.Scope().Lookup(n).(*types.TypeName); ok && !tn.IsAlias() {
			if st, ok := tn.Type().Underlying().(*types.Struct); ok {
				out[tn] = st
			}
		}
	}
	return out
}

func takesData(sig *types.Signature) bool {
	for i := 0; i < sig.Params().Len(); i++ {
		t := sig.Params().At(i).Type()
		if sig.Variadic() && i == sig.Params().Len()-1 {
			if s, ok := t.(*types.Slice); ok {
				t = s.Elem()
			}
		}
		if isString(t) || isBytes(t) || isStrings(t) || isReader(t) {
			return true
		}
		if i, ok := t.Underlying().(*types.Interface); ok && i.Empty() {
			return true
		}
	}
	return false
}

// funcItems applies the rules to one function or method.
func funcItems(pkg *types.Package, f funcInfo, structs map[*types.TypeName]*types.Struct) []Item {
	var out []Item
	sig := f.sig
	sigText := f.name + sig.String()[len("func"):]
	var sinkPos []string
	var cats []string
	seenCat := map[string]bool{}
	addSink := func(pos, cat string) {
		sinkPos = append(sinkPos, pos)
		if !seenCat[cat] {
			seenCat[cat] = true
			cats = append(cats, cat)
		}
	}
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i)
		t := p.Type()
		if sig.Variadic() && i == sig.Params().Len()-1 {
			if s, ok := t.(*types.Slice); ok {
				t = s.Elem()
			}
		}
		pos := fmt.Sprintf("arg%d", i)
		name := p.Name()
		switch {
		case isString(t) || isStrings(t):
			if c := categoryOf(name); c != "" {
				addSink(pos, c)
			} else if (name == "name" || name == "id") && hasWord(f.name, resourceNouns) {
				addSink(pos, "resource_name")
			} else if (name == "name" || name == "oldname" || name == "newname") && hasWord(f.name, fileVerbs) {
				addSink(pos, "path_traversal")
			}
		case isBytes(t):
			if hasWord(name, bytesSinkWords) {
				addSink(pos, "xss")
			}
		default:
			// a struct of the package with a field that names something (a destination with an Object, a query with a Prefix)
			pt := t
			if ptr, ok := pt.(*types.Pointer); ok {
				pt = ptr.Elem()
			}
			if n, ok := pt.(*types.Named); ok {
				if st := structs[n.Obj()]; st != nil {
					for j := 0; j < st.NumFields(); j++ {
						fl := st.Field(j)
						if fl.Exported() && isString(fl.Type()) {
							if c := categoryOf(fl.Name()); c != "" && c != "xss" {
								addSink(pos+"."+fl.Name(), c)
							}
						}
					}
				}
			}
		}
	}
	if len(sinkPos) > 0 {
		out = append(out, Item{Symbol: f.symbol, Kind: KindSink, Positions: sinkPos, Rule: "sink: argument that names a " + strings.Join(cats, "/"), Sig: sigText})
	}
	// source: a result that carries a body or content (a function that is a sanitizer candidate is not one: `EscapeText`)
	res := sig.Results()
	for i := 0; i < res.Len(); i++ {
		t := res.At(i).Type()
		if (isString(t) || isBytes(t) || isReader(t)) && hasWord(f.name, sourceWords) && !hasWord(f.name, sanitizerWords) {
			out = append(out, Item{Symbol: f.symbol, Kind: KindSource, Positions: []string{"result"}, Rule: "source: result of a function that carries a body or content", Sig: sigText})
			break
		}
	}
	// sanitizer: string -> string
	if sig.Params().Len() >= 1 && hasWord(f.name, sanitizerWords) {
		okParam := false
		for i := 0; i < sig.Params().Len(); i++ {
			if isString(sig.Params().At(i).Type()) {
				okParam = true
			}
		}
		okRes := false
		for i := 0; i < res.Len(); i++ {
			if isString(res.At(i).Type()) {
				okRes = true
			}
		}
		if okParam && okRes {
			out = append(out, Item{Symbol: f.symbol, Kind: KindSanitizer, Positions: []string{"result"}, Rule: "sanitizer: string to string, named escape/quote/sanitize/clean", Sig: sigText})
		}
	}
	// calls: a function or an interface of the package that the caller gives
	var callPos []string
	for i := 0; i < sig.Params().Len(); i++ {
		t := sig.Params().At(i).Type()
		if isFuncOrOwnInterface(t, pkg) {
			callPos = append(callPos, fmt.Sprintf("arg%d", i))
		}
	}
	if len(callPos) > 0 {
		out = append(out, Item{Symbol: f.symbol, Kind: KindCalls, Positions: callPos, Rule: "calls: takes a function or an interface of the package", Sig: sigText})
	}
	return out
}
