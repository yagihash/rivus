package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Stage 1 for Dart (docs/DESIGN-package-models.md, 6.11; bench/parity/README.md).
//
// The package is fetched with `dart pub get` in a project of its own that has nothing but the dependency, and its public API is read by
// the Dart analyzer (dartapi/bin/api.dart: types only; the symbols are written as the extractor writes them). Nothing of the package is
// run: no `dart run`, no test, no build hook. When a package in the project has `hook/build.dart` or `hook/link.dart` the command stops
// (exit 10) and says which. The rule table of layer 1 is the one of Go (rules.go: the same words, the same shapes); this file only reads
// the signature of Dart in the form that the table needs.

// dartName: `package:dio`, `dio` and `package:dio/dio.dart` all name the package dio.
func dartName(pkg string) string {
	s := strings.TrimPrefix(pkg, "package:")
	name, _, _ := strings.Cut(s, "/")
	return name
}

func isDart(pkg string) bool { return strings.HasPrefix(pkg, "package:") }

type dartType struct {
	C     string `json:"c"` // string, uri, bytes, strings, reader, func, object, map, own, other, void
	T     string `json:"t"`
	ID    string `json:"id"`
	Own   bool   `json:"own"`
	Iface bool   `json:"iface"`
}

type dartParam struct {
	Name     string   `json:"name"`
	Named    bool     `json:"named"`
	Index    int      `json:"index"`
	Required bool     `json:"required"`
	Type     dartType `json:"type"`
}

type dartSym struct {
	Symbol   string      `json:"symbol"`
	Name     string      `json:"name"`
	Kind     string      `json:"kind"` // type, constructor, method, getter, setter, field, function
	Class    string      `json:"class"`
	Static   bool        `json:"static"`
	Abstract bool        `json:"abstract"`
	Params   []dartParam `json:"params"`
	Ret      *dartType   `json:"ret"`
	Fields   []struct {
		Name string   `json:"name"`
		Type dartType `json:"type"`
	} `json:"fields"`
}

type dartAPIJSON struct {
	Package   string    `json:"package"`
	Libraries int       `json:"libraries"`
	Symbols   []dartSym `json:"symbols"`
}

// helperDir is the directory of the program that reads the API (a Dart project of its own).
func helperDir(repo string) string { return filepath.Join(repo, "bench", "model-author", "dartapi") }

// runDartTool runs a dart command in dir and returns what it printed to stdout.
func runDartTool(dir string, args ...string) ([]byte, []byte, error) {
	c := exec.Command("dart", args...)
	c.Dir = dir
	var so, se bytes.Buffer
	c.Stdout, c.Stderr = &so, &se
	err := c.Run()
	return so.Bytes(), se.Bytes(), err
}

// DartFetch fetches name@version (`dart pub get` only) into dir and returns the version that was resolved.
func DartFetch(dir, name, version string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", &exitError{ExitFetch, err}
	}
	v := version
	if v == "" {
		v = "any"
	}
	pub := fmt.Sprintf("name: fx\npublish_to: none\nenvironment:\n  sdk: ^3.8.0\ndependencies:\n  %s: %s\n", name, v)
	return dartPubGet(dir, name, pub)
}

func dartPubGet(dir, name, pubspec string) (string, error) {
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte(pubspec), 0o644); err != nil {
		return "", &exitError{ExitFetch, err}
	}
	_ = os.Remove(filepath.Join(dir, "pubspec.lock")) // the version is the one that pubspec.yaml says, not an old lock's
	if _, se, err := runDartTool(dir, "pub", "get"); err != nil {
		return "", &exitError{ExitFetch, fmt.Errorf("dart pub get (%s): %v\n%s", name, err, se)}
	}
	if hooks := DartHooks(dir); len(hooks) > 0 {
		return "", &exitError{ExitFetch, fmt.Errorf("stopped: a package has a build hook, which could run code when it is built (nothing was run): %s", strings.Join(hooks, ", "))}
	}
	return dartLockedVersion(dir, name)
}

type pkgConfig struct {
	Packages []struct {
		Name       string `json:"name"`
		RootURI    string `json:"rootUri"`
		PackageURI string `json:"packageUri"`
	} `json:"packages"`
}

func readPackageConfig(dir string) (*pkgConfig, error) {
	b, err := os.ReadFile(filepath.Join(dir, ".dart_tool", "package_config.json"))
	if err != nil {
		return nil, err
	}
	var pc pkgConfig
	if err := json.Unmarshal(b, &pc); err != nil {
		return nil, err
	}
	return &pc, nil
}

func (pc *pkgConfig) root(dir, name string) string {
	for _, p := range pc.Packages {
		if p.Name != name {
			continue
		}
		if u, err := url.Parse(p.RootURI); err == nil && u.Scheme == "file" {
			return u.Path
		}
		return filepath.Clean(filepath.Join(dir, ".dart_tool", filepath.FromSlash(p.RootURI)))
	}
	return ""
}

// DartHooks lists the packages of the project that have a build hook (hook/build.dart, hook/link.dart).
func DartHooks(dir string) []string {
	pc, err := readPackageConfig(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, p := range pc.Packages {
		if p.Name == "fx" {
			continue
		}
		root := pc.root(dir, p.Name)
		for _, h := range []string{"build.dart", "link.dart"} {
			if root != "" {
				if _, err := os.Stat(filepath.Join(root, "hook", h)); err == nil {
					out = append(out, p.Name+"/hook/"+h)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

var lockVersionRe = regexp.MustCompile(`(?m)^  (\S+):\n(?:    .*\n)*?    version: "([^"]+)"`)

func dartLockedVersion(dir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "pubspec.lock"))
	if err != nil {
		return "", &exitError{ExitFetch, err}
	}
	for _, m := range lockVersionRe.FindAllStringSubmatch(string(b), -1) {
		if m[1] == name {
			return m[2], nil
		}
	}
	return "", &exitError{ExitFetch, fmt.Errorf("%s is not in pubspec.lock", name)}
}

// DartReadAPI runs the analyzer program over the project and reads what it prints.
func DartReadAPI(helper, dir, name string, libs []string, online bool) (*dartAPIJSON, error) {
	// the reader's own dependencies (the analyzer) come from the cache when they are there
	if _, se, err := runDartTool(helper, "pub", "get", "--offline"); err != nil {
		if !online {
			return nil, &exitError{ExitLoad, fmt.Errorf("dart pub get --offline (the API reader): %v\n%s", err, se)}
		}
		if _, se, err := runDartTool(helper, "pub", "get"); err != nil {
			return nil, &exitError{ExitLoad, fmt.Errorf("dart pub get (the API reader): %v\n%s", err, se)}
		}
	}
	args := append([]string{"run", "bin/api.dart", dir, name}, libs...)
	so, se, err := runDartTool(helper, args...)
	if err != nil {
		return nil, &exitError{ExitLoad, fmt.Errorf("reading the API of %s: %v\n%s", name, err, se)}
	}
	var a dartAPIJSON
	if err := json.Unmarshal(so, &a); err != nil {
		return nil, &exitError{ExitLoad, fmt.Errorf("the API reader printed no JSON: %v", err)}
	}
	if a.Libraries == 0 {
		return nil, &exitError{ExitLoad, fmt.Errorf("%s has no public library (lib/*.dart outside lib/src)", name)}
	}
	return &a, nil
}

// DartLoad fetches the package and reads its API; withCalls says whether `calls` candidates are listed.
func DartLoad(o *options, repo, slug string, facts *Facts) (*loaded, *API, error) {
	name := dartName(o.pkg)
	dir := filepath.Join(o.work, "fetch-"+slug)
	ver, err := DartFetch(dir, name, o.version)
	if err != nil {
		return nil, nil, err
	}
	var libs []string
	if o.dartLibs != "" {
		libs = strings.Split(o.dartLibs, ",")
	}
	a, err := DartReadAPI(helperDir(repo), dir, name, libs, true)
	if err != nil {
		return nil, nil, err
	}
	return &loaded{Module: name, Version: ver}, ScanDart(a, o.pkg, !facts.BodiesRead(o.pkg)), nil
}

// positional names a parameter the way the rows do: `args: [0]` is arg0, `named: [ref]` is named:ref.
func (p dartParam) pos() string {
	if p.Named {
		return "named:" + p.Name
	}
	return fmt.Sprintf("arg%d", p.Index)
}

func (p dartParam) strLike() bool {
	return p.Type.C == "string" || p.Type.C == "uri" || p.Type.C == "strings"
}

func dartSig(s dartSym) string {
	var ps []string
	var named []string
	for _, p := range s.Params {
		t := p.Type.T + " " + p.Name
		if p.Named {
			named = append(named, strings.TrimSpace(t))
		} else {
			ps = append(ps, strings.TrimSpace(t))
		}
	}
	all := strings.Join(ps, ", ")
	if len(named) > 0 {
		if all != "" {
			all += ", "
		}
		all += "{" + strings.Join(named, ", ") + "}"
	}
	ret := ""
	if s.Ret != nil && s.Ret.T != "" {
		ret = " -> " + s.Ret.T
	}
	switch s.Kind {
	case "field", "getter":
		return s.Name + ret
	}
	return s.Name + "(" + all + ")" + ret
}

type dartFieldList = []struct {
	Name string
	Type dartType
}

// walkFields calls f for each string field (and each one of the classes of the package that it holds, to three steps) whose name a rule of the table knows.
func walkFields(fields map[string]dartFieldList, id, prefix string, depth int, seen map[string]bool, f func(path, cat string)) {
	if depth >= 3 || seen[id] {
		return
	}
	seen[id] = true
	defer delete(seen, id)
	for _, fl := range fields[id] {
		switch {
		case fl.Type.C == "string" || fl.Type.C == "uri":
			if c := categoryOf(fl.Name); c != "" && c != "xss" {
				f(prefix+fl.Name, c)
			}
		case fl.Type.C == "own":
			walkFields(fields, fl.Type.ID, prefix+fl.Name+".", depth+1, seen, f)
		}
	}
}

// hasNoun: a name of the table's resource nouns, in the singular or the plural (`ProjectsTopicsResource` is about a topic).
func hasNoun(name string) bool {
	for _, w := range words(name) {
		for _, n := range resourceNouns {
			if w == n || w == n+"s" {
				return true
			}
		}
	}
	return false
}

func dartCarriesData(t *dartType) bool {
	// a Map (the headers, the metadata, the parameters of a call) carries what somebody wrote as a string does
	return t != nil && (t.C == "string" || t.C == "bytes" || t.C == "reader" || t.C == "map")
}

// ScanDart applies the rule table of layer 1 to the API of a Dart package: the rules of Go, read for the signature of Dart.
func ScanDart(a *dartAPIJSON, pkg string, withCalls bool) *API {
	api := &API{Pkg: pkg, Symbols: map[string]string{}}
	fields := map[string]dartFieldList{}
	for _, s := range a.Symbols {
		if s.Kind != "type" {
			continue
		}
		for _, f := range s.Fields {
			fields[s.Symbol] = append(fields[s.Symbol], struct {
				Name string
				Type dartType
			}{f.Name, f.Type})
		}
	}
	taken := map[string]bool{}
	for _, s := range a.Symbols {
		if s.Kind == "type" {
			continue
		}
		sig := dartSig(s)
		api.Symbols[s.Symbol] = sig
		for _, it := range dartItems(s, sig, fields, withCalls) {
			taken[s.Symbol] = true
			api.Items = append(api.Items, it)
		}
	}
	for _, s := range a.Symbols {
		if s.Kind == "type" || taken[s.Symbol] || s.Kind == "field" || s.Kind == "getter" {
			continue
		}
		for _, p := range s.Params {
			if p.strLike() || p.Type.C == "bytes" || p.Type.C == "reader" || p.Type.C == "object" {
				api.Layer2 = append(api.Layer2, s.Symbol+" "+dartSig(s))
				break
			}
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

func dartItems(s dartSym, sig string, fields map[string]dartFieldList, withCalls bool) []Item {
	var out []Item
	// a field of a class: a source when it carries a body or content
	if s.Kind == "field" {
		// a constant (`static const contentTypeHeader`) is what the package says, not what somebody wrote
		if !s.Static && s.Ret != nil && dartCarriesData(s.Ret) && hasWord(s.Name, sourceWords) {
			out = append(out, Item{Symbol: s.Symbol, Kind: KindSource, Positions: []string{"field"}, Rule: "source: field that carries a body or content", Sig: "field " + s.Ret.T})
		}
		return out
	}
	var sinkPos, cats []string
	seenCat := map[string]bool{}
	addSink := func(pos, cat string) {
		sinkPos = append(sinkPos, pos)
		if !seenCat[cat] {
			seenCat[cat] = true
			cats = append(cats, cat)
		}
	}
	// the noun of the member or of its class names the resource of a `name` / `id` argument (`Bucket.read(name)`, `Topic(name)`)
	resourceish := hasNoun(s.Name) || hasNoun(s.Class)
	fileish := hasWord(s.Name, fileVerbs)
	if s.Kind != "setter" {
		for _, p := range s.Params {
			pos := p.pos()
			switch {
			case p.strLike():
				if c := categoryOf(p.Name); c != "" {
					addSink(pos, c)
				} else if (p.Name == "name" || p.Name == "id") && resourceish {
					addSink(pos, "resource_name")
				} else if (p.Name == "name" || p.Name == "oldname" || p.Name == "newname") && fileish {
					addSink(pos, "path_traversal")
				}
			case p.Type.C == "object":
				// `dynamic` or `Object` says "one of several types" (dio's savePath is a path or a function that makes it): the name decides
				if c := categoryOf(p.Name); c != "" && c != "xss" {
					addSink(pos, c)
				}
			case p.Type.C == "bytes":
				if hasWord(p.Name, bytesSinkWords) {
					addSink(pos, "xss")
				}
			case p.Type.C == "own":
				// a class of the package with a field that names something (a request with a path, a destination with an object),
				// and the classes inside it (a request that holds a configuration that holds a query), up to three steps
				walkFields(fields, p.Type.ID, "", 0, map[string]bool{}, func(path, cat string) { addSink(pos+"."+path, cat) })
			}
		}
	} else {
		// a setter takes one value: the property is what is written (`options.baseUrl = x`)
		if len(s.Params) == 1 && s.Params[0].strLike() {
			if c := categoryOf(s.Name); c != "" {
				addSink(s.Params[0].pos(), c)
			}
		}
	}
	// a format string and the values after it (`format(String format, List<Object> args)`)
	for _, p := range s.Params {
		if p.strLike() && hasWord(p.Name, []string{"format"}) {
			for _, q := range s.Params {
				if q.Index > p.Index && !q.Named && (q.Type.C == "object" || q.Type.C == "strings") {
					addSink(q.pos(), "xss")
				}
			}
		}
	}
	if len(sinkPos) > 0 {
		out = append(out, Item{Symbol: s.Symbol, Kind: KindSink, Positions: sinkPos, Rule: "sink: argument that names a " + strings.Join(cats, "/"), Sig: sig})
	}
	// source: a result that carries a body or content (a function that is a sanitizer candidate is not one)
	if s.Kind != "setter" && s.Kind != "constructor" && dartCarriesData(s.Ret) && hasWord(s.Name, sourceWords) && !hasWord(s.Name, sanitizerWords) {
		out = append(out, Item{Symbol: s.Symbol, Kind: KindSource, Positions: []string{"result"}, Rule: "source: result of a function that carries a body or content", Sig: sig})
	}
	// sanitizer: string -> string
	if s.Kind != "setter" && s.Kind != "constructor" && s.Kind != "getter" && hasWord(s.Name, sanitizerWords) && s.Ret != nil && s.Ret.C == "string" {
		for _, p := range s.Params {
			if p.Type.C == "string" {
				out = append(out, Item{Symbol: s.Symbol, Kind: KindSanitizer, Positions: []string{"result"}, Rule: "sanitizer: string to string, named escape/quote/sanitize/clean", Sig: sig})
				break
			}
		}
	}
	// calls: a function or an abstract class of the package that the caller gives
	if withCalls && s.Kind != "setter" {
		var callPos []string
		for _, p := range s.Params {
			if p.Type.C == "func" || (p.Type.C == "own" && p.Type.Iface) {
				callPos = append(callPos, p.pos())
			}
		}
		if len(callPos) > 0 {
			out = append(out, Item{Symbol: s.Symbol, Kind: KindCalls, Positions: callPos, Rule: "calls: takes a function or an abstract class of the package", Sig: sig})
		}
	}
	return out
}
