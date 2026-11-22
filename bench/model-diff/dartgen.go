package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yagihash/rivus/internal/model"
)

// The generator for Dart: the signatures are read from the source of the package (a regular expression: the positional and named
// parameters, the return type), not run. What a regular expression cannot know is said in the model file:
// `# model-diff: value <Type> <expression>` makes a value of a class (a receiver, an argument), `$S` is where the source goes
// (a class that has no `$S` is made without it, and the source cannot be put into it).

type DartOptions struct {
	Pkg, Out, Version, Repo string
	Rows                    []Row
	Notes                   []Note
}

type dartParam struct {
	Type, Name string
	Named      bool
	Required   bool
}

type dartSig struct {
	Params   []dartParam
	Ret      string
	Getter   bool
	Ctor     bool
	Class    string
	Name     string
	Abstract bool
}

type dartGen struct {
	pkg, lib string // the package name and the directory of its lib
	hints    map[string]string
	probes   []Probe
	skips    []Skip
	errs     []string
	funcs    []string
	sinks    []string
	files    map[string]string
}

var depVersionRe = regexp.MustCompile(`(?m)^  (\S+):\n(?:    .*\n)*?    version: "([^"]+)"`)

// repoDartVersion: the version that a pubspec.lock of the checkout has of the package ("" if none).
func repoDartVersion(repo, name string) string {
	var locks []string
	for _, pat := range []string{"bench/*/pubspec.lock", "bench/*/*/pubspec.lock", "extractor/dart/pubspec.lock", "pubspec.lock"} {
		m, _ := filepath.Glob(filepath.Join(repo, pat))
		locks = append(locks, m...)
	}
	sort.Strings(locks)
	for _, l := range locks {
		b, err := os.ReadFile(l)
		if err != nil {
			continue
		}
		for _, m := range depVersionRe.FindAllStringSubmatch(string(b), -1) {
			if m[1] == name {
				return m[2]
			}
		}
	}
	return ""
}

func GenDart(o DartOptions) (*Gen, error) {
	name := strings.TrimPrefix(o.Pkg, "package:")
	dir := filepath.Join(o.Out, "fx")
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		return nil, err
	}
	ver := o.Version
	if ver == "" {
		ver = repoDartVersion(o.Repo, name)
	}
	if ver == "" {
		ver = "any"
	}
	pub := fmt.Sprintf("name: fx\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\ndependencies:\n  %s: %s\n", name, ver)
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte(pub), 0o644); err != nil {
		return nil, err
	}
	c := exec.Command("dart", "pub", "get")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("dart pub get: %v\n%s", err, out)
	}
	lib, err := dartLibDir(dir, name)
	if err != nil {
		return nil, err
	}
	g := &dartGen{pkg: name, lib: lib, hints: map[string]string{}, files: map[string]string{}}
	for _, n := range o.Notes {
		if n.Kind == "value" {
			g.hints[n.Symbol] = n.Text
		}
	}
	seen := map[string]bool{}
	for _, r := range o.Rows {
		k := fmt.Sprint(r.Symbol, r.Sink != nil)
		if seen[k] {
			continue
		}
		seen[k] = true
		g.row(r)
	}
	g.write(dir, name)
	gen := &Gen{Probes: g.probes, Skips: g.skips, Dir: dir, DirA: dir, Errors: g.errs}
	for _, p := range g.probes {
		if p.Kind == "flow" {
			gen.HasFlows = true
		}
	}
	return gen, nil
}

// dartLibDir: the lib directory of the package, from .dart_tool/package_config.json.
func dartLibDir(dir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, ".dart_tool", "package_config.json"))
	if err != nil {
		return "", err
	}
	var pc struct {
		Packages []struct{ Name, RootURI, PackageURI string }
	}
	if err := json.Unmarshal(b, &pc); err != nil {
		return "", err
	}
	for _, p := range pc.Packages {
		if p.Name == name {
			root := p.RootURI
			if u, err := url.Parse(root); err == nil && u.Scheme == "file" {
				root = u.Path
			} else {
				root = filepath.Join(dir, ".dart_tool", root)
			}
			return filepath.Join(root, p.PackageURI), nil
		}
	}
	return "", fmt.Errorf("%s is not in package_config.json", name)
}

func (g *dartGen) skip(sym, variant, why string) {
	g.skips = append(g.skips, Skip{Symbol: sym, Variant: variant, Reason: why})
}

// findBody returns the text of the class (to the first line that is only `}`).
func classBody(src, cls string) (string, bool, bool) {
	m := regexp.MustCompile(`(?m)^((?:abstract |final |base |sealed |interface |mixin )*)(?:mixin )?class ` + cls + `\b[^{;]*\{`).FindStringSubmatchIndex(src)
	if m == nil {
		return "", false, false
	}
	end := strings.Index(src[m[1]:], "\n}\n")
	if end < 0 {
		end = len(src) - m[1]
	}
	return src[m[1] : m[1]+end], strings.Contains(src[m[2]:m[3]], "abstract"), true
}

func splitParams(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '<', '(', '[', '{':
			depth++
		case '>', ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if t := strings.TrimSpace(s[start:]); t != "" {
		out = append(out, t)
	}
	return out
}

func parseParams(params, body string) []dartParam {
	var out []dartParam
	named := false
	var pos, nm string
	if i := strings.IndexAny(params, "{["); i >= 0 {
		pos, nm = params[:i], params[i+1:]
		named = params[i] == '{'
		nm = strings.TrimRight(strings.TrimSpace(nm), "}]")
	} else {
		pos = params
	}
	for _, p := range splitParams(pos) {
		out = append(out, parseParam(p, false, body))
	}
	if named || nm != "" {
		for _, p := range splitParams(nm) {
			out = append(out, parseParam(p, named, body))
		}
	}
	return out
}

// fieldType is the type of the field `name` of the class (for `this.name` in a constructor).
func fieldType(body, name string) string {
	m := regexp.MustCompile(`(?m)^\s*(?:final |late )*([\w<>?,. ]+?)\s+` + name + `;`).FindStringSubmatch(body)
	if m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

var (
	formalRe = regexp.MustCompile(`^(?:this|super)\.(\w+)$`)
	paramRe  = regexp.MustCompile(`^(.+?)\s+(\w+)$`)
)

func parseParam(p string, isNamed bool, body string) dartParam {
	req := strings.HasPrefix(p, "required ")
	p = strings.TrimPrefix(strings.TrimPrefix(p, "required "), "covariant ")
	if i := strings.Index(p, "="); i > 0 {
		p = strings.TrimSpace(p[:i])
	}
	if m := formalRe.FindStringSubmatch(p); m != nil {
		return dartParam{Type: fieldType(body, m[1]), Name: m[1], Named: isNamed, Required: req || !isNamed}
	}
	m := paramRe.FindStringSubmatch(p)
	if m == nil {
		return dartParam{Name: p, Named: isNamed, Required: req || !isNamed}
	}
	return dartParam{Type: strings.TrimSpace(m[1]), Name: m[2], Named: isNamed, Required: req || !isNamed}
}

// signature finds a method, a getter, a constructor (`new`) or a top-level function in the source of the package.
func (g *dartGen) signature(sym string, propertyRow bool) (*dartSig, string) {
	path, member, _ := strings.Cut(strings.TrimPrefix(sym, "package:"+g.pkg+"/"), "#")
	b, err := os.ReadFile(filepath.Join(g.lib, filepath.FromSlash(path)))
	if err != nil {
		return nil, "the file of the symbol is not in the package"
	}
	src := string(b)
	sig := &dartSig{}
	var body string
	cls, name, isMember := strings.Cut(member, ".")
	if !isMember {
		name, cls = member, ""
		body = src
	} else {
		var ok bool
		body, sig.Abstract, ok = classBody(src, cls)
		if !ok {
			return nil, "class not found"
		}
		sig.Class = cls
	}
	sig.Name = name
	var re *regexp.Regexp
	switch {
	case isMember && name == "new":
		re = regexp.MustCompile(`(?m)^  (?:const )?(?:factory )?` + cls + `\(`)
		sig.Ctor = true
	case isMember:
		re = regexp.MustCompile(`(?m)^  (?:static )?([\w<>?,.\[\] ]*?)\b` + name + `\(`)
		if g := regexp.MustCompile(`(?m)^  ([\w<>?,.\[\] ]*?)\bget ` + name + `\b`).FindStringSubmatch(body); g != nil {
			sig.Getter, sig.Ret = true, strings.TrimSpace(g[1])
			return sig, ""
		}
		if regexp.MustCompile(`(?m)^  (?:final |late )?([\w<>?,.\[\] ]+?) ` + name + `;`).MatchString(body) {
			sig.Getter = true
			return sig, ""
		}
	default:
		re = regexp.MustCompile(`(?m)^([\w<>?,.\[\] ]*?)\b` + name + `\(`)
	}
	m := re.FindStringSubmatchIndex(body)
	if m == nil && isMember && propertyRow {
		// not declared in this class (inherited, or a field of a superclass): a read of a property, as the row says (recv to result)
		sig.Getter = true
		return sig, ""
	}
	if m == nil {
		return nil, "signature not found (regular expression)"
	}
	if len(m) >= 4 && m[2] >= 0 {
		sig.Ret = strings.TrimSpace(body[m[2]:m[3]])
		sig.Ret = strings.TrimPrefix(sig.Ret, "static ")
	}
	depth, j := 1, m[1]
	for depth > 0 && j < len(body) {
		switch body[j] {
		case '(':
			depth++
		case ')':
			depth--
		}
		j++
	}
	sig.Params = parseParams(body[m[1]:j-1], body)
	return sig, ""
}

var (
	reString = regexp.MustCompile(`^String$`)
)

// val is the expression of a value: the source (taint) or a constant.
func (g *dartGen) val(ty string, taint bool) (string, bool) {
	return g.valFor(ty, "", taint)
}

// valFor: member is "Class.member" of the call that the value is the receiver of (a note for that member goes first).
func (g *dartGen) valFor(ty, member string, taint bool) (string, bool) {
	ty = strings.TrimSpace(ty)
	h, ok := g.hints[member]
	if !ok {
		h, ok = g.hints[strings.TrimSuffix(ty, "?")]
	}
	if ok {
		if taint && !strings.Contains(h, "$S") {
			return "", false
		}
		return strings.ReplaceAll(h, "$S", map[bool]string{true: "s", false: "'c'"}[taint]), true
	}
	nullable := strings.HasSuffix(ty, "?")
	base := strings.TrimSuffix(ty, "?")
	switch base {
	case "String":
		return map[bool]string{true: "s", false: "'c'"}[taint], true
	case "Uri":
		return map[bool]string{true: "Uri.parse(s)", false: "Uri.parse('http://c.example/')"}[taint], true
	case "List<int>", "Iterable<int>":
		return map[bool]string{true: "s.codeUnits", false: "<int>[]"}[taint], true
	case "Map<String, String>", "Map<String,String>":
		return map[bool]string{true: "{'k': s}", false: "<String, String>{}"}[taint], true
	case "Object", "dynamic":
		return map[bool]string{true: "s", false: "'c'"}[taint], true
	}
	if taint {
		return "", false
	}
	if nullable {
		return "null", true
	}
	switch base {
	case "int":
		return "0", true
	case "double", "num":
		return "0", true
	case "bool":
		return "false", true
	case "Duration":
		return "Duration.zero", true
	}
	return "", false
}

func (g *dartGen) row(r Row) {
	sig, why := g.signature(r.Symbol, r.Prop != nil && onlyFromRecv(r.Prop))
	if sig == nil {
		g.skip(r.Symbol, "", why)
		return
	}
	if r.Sink != nil {
		if len(r.Sink.Named) > 0 || r.Sink.WhenArgType != nil || len(r.Sink.Parts) > 0 || r.Sink.Field != "" {
			g.skip(r.Symbol, "", "sink row with named / parts / field / when_arg_type (not checked)")
			return
		}
	}
	for _, p := range sig.Params {
		if p.Type == "" {
			g.skip(r.Symbol, "", "parameter type unknown (the regular expression could not read it): "+p.Name)
			return
		}
		if strings.Contains(p.Type, "Function") && (p.Required || !p.Named) {
			g.skip(r.Symbol, "", "takes a callback (not checked)")
			return
		}
		if p.Required {
			if _, ok := g.val(p.Type, false); !ok {
				g.skip(r.Symbol, "", "argument type that the code cannot build: "+p.Type+" (add `# model-diff: value "+strings.TrimSuffix(p.Type, "?")+" <expression>` to the model file)")
				return
			}
		}
	}
	// the receiver
	var recvT bool
	recvCls := sig.Class
	instance := sig.Class != "" && !sig.Ctor && !g.static(r.Symbol)
	if instance {
		if _, ok := g.valFor(recvCls, recvCls+"."+sig.Name, false); !ok {
			if sig.Abstract || !g.hasNoArgCtor(r.Symbol, recvCls) {
				g.skip(r.Symbol, "", "no receiver: add `# model-diff: value "+recvCls+" <expression>` to the model file")
				return
			}
		}
		_, recvT = g.valFor(recvCls, recvCls+"."+sig.Name, true)
	}
	isSink := r.Sink != nil
	var positions []int
	for i, p := range sig.Params {
		if p.Named && !p.Required && !isSink {
			if _, ok := g.val(p.Type, true); !ok {
				continue
			}
		} else if p.Named && isSink {
			continue
		}
		if _, ok := g.val(p.Type, true); ok {
			positions = append(positions, i)
		}
	}
	emit := func(variant string, taintPos int, taintRecv bool, expect *bool) {
		g.emit(r, sig, variant, taintPos, taintRecv, expect)
	}
	if isSink {
		yes, no := true, false
		declared := func(k int) bool {
			if len(r.Sink.Args) == 0 && !r.Sink.Recv {
				return true
			}
			for _, a := range r.Sink.Args {
				if a == k {
					return true
				}
			}
			return false
		}
		npos := 0
		for _, p := range sig.Params {
			if !p.Named {
				npos++
			}
		}
		for _, a := range r.Sink.Args {
			if a >= npos {
				g.errs = append(g.errs, fmt.Sprintf("%s: the row declares arg%d, which the function does not have (%d positional parameters)", r.Symbol, a, npos))
			}
		}
		for _, i := range positions {
			e := &no
			if declared(i) {
				e = &yes
			}
			emit(fmt.Sprintf("arg%d", i), i, false, e)
		}
		if instance && recvT {
			e := &no
			if r.Sink.Recv {
				e = &yes
			}
			emit("recv", -1, true, e)
		}
		emit("const", -1, false, &no)
		return
	}
	n := 0
	if instance && recvT {
		emit("recv", -1, true, nil)
		n++
	}
	for _, i := range positions {
		emit(fmt.Sprintf("arg%d", i), i, false, nil)
		n++
	}
	if n == 0 {
		g.skip(r.Symbol, "", "no position that the source can be put into")
	}
}

// static: a `static` member (the receiver is the class).
func (g *dartGen) static(sym string) bool {
	path, member, _ := strings.Cut(strings.TrimPrefix(sym, "package:"+g.pkg+"/"), "#")
	b, _ := os.ReadFile(filepath.Join(g.lib, filepath.FromSlash(path)))
	_, name, _ := strings.Cut(member, ".")
	return regexp.MustCompile(`(?m)^  static [^\n(]*\b` + name + `\(`).Match(b)
}

func (g *dartGen) hasNoArgCtor(sym, cls string) bool {
	path, _, _ := strings.Cut(strings.TrimPrefix(sym, "package:"+g.pkg+"/"), "#")
	b, _ := os.ReadFile(filepath.Join(g.lib, filepath.FromSlash(path)))
	body, _, _ := classBody(string(b), cls)
	return regexp.MustCompile(`(?m)^  (?:const )?(?:factory )?` + cls + `\(\)`).MatchString(body)
}

func (g *dartGen) emit(r Row, sig *dartSig, variant string, taintPos int, taintRecv bool, expect *bool) {
	id := len(g.probes)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Future<void> fix%d() async { // %s %s\n  final s = src();\n  // ignore: unused_local_variable\n  final u = s;\n", id, r.Symbol, variant)
	var args []string
	for i, p := range sig.Params {
		if p.Named && !p.Required && i != taintPos {
			continue
		}
		e, _ := g.val(p.Type, i == taintPos)
		if p.Named {
			e = p.Name + ": " + e
		}
		args = append(args, e)
	}
	async := strings.HasPrefix(sig.Ret, "Future")
	await := map[bool]string{true: "await ", false: ""}[async]
	var call string
	switch {
	case sig.Ctor:
		call = sig.Class + "(" + strings.Join(args, ", ") + ")"
		await = ""
	case sig.Class != "":
		rc, _ := g.valFor(sig.Class, sig.Class+"."+sig.Name, taintRecv)
		if rc == "" {
			rc = sig.Class + "()"
		}
		if g.static(r.Symbol) {
			rc = sig.Class
		}
		fmt.Fprintf(&sb, "  final rc = %s;\n", rc)
		call = "rc." + sig.Name
		if !sig.Getter {
			call += "(" + strings.Join(args, ", ") + ")"
		}
	default:
		call = sig.Name + "(" + strings.Join(args, ", ") + ")"
	}
	var places []string
	if expect == nil && sig.Ret != "void" && sig.Ret != "Future<void>" {
		fmt.Fprintf(&sb, "  final r = %s%s;\n  sink%d_result(r);\n", await, call, id)
		g.sinks = append(g.sinks, fmt.Sprintf("sink%d_result", id))
		places = []string{"result"}
	} else {
		fmt.Fprintf(&sb, "  %s%s;\n", await, call)
	}
	sb.WriteString("}\n")
	g.funcs = append(g.funcs, sb.String())
	kind := "flow"
	cat := ""
	if r.Sink != nil {
		kind, cat = "sink", r.Sink.Category
	}
	g.probes = append(g.probes, Probe{ID: id, Symbol: r.Symbol, Kind: kind, Variant: variant, Outputs: places, Expect: expect, Category: cat})
}

func (g *dartGen) write(dir, name string) {
	var imports []string
	libs, _ := filepath.Glob(filepath.Join(g.lib, "*.dart"))
	sort.Strings(libs)
	for _, l := range libs {
		imports = append(imports, fmt.Sprintf("import 'package:%s/%s';", name, filepath.Base(l)))
	}
	var sb strings.Builder
	sb.WriteString(strings.Join(imports, "\n") + "\nimport 'mark.dart';\n\n")
	sb.WriteString(strings.Join(g.funcs, "\n"))
	for _, s := range g.sinks {
		fmt.Fprintf(&sb, "\nvoid %s(Object? v) {}\n", s)
	}
	var r strings.Builder
	r.WriteString("ruleset: diff/fx@1\nsources:\n  - symbol: package:fx/mark.dart#src\n    labels: [user_input]\nsinks:\n")
	for _, s := range g.sinks {
		fmt.Fprintf(&r, "  - symbol: package:fx/fix.dart#%s\n    category: model_diff\n", s)
	}
	_ = os.WriteFile(filepath.Join(dir, "lib", "fix.dart"), []byte(sb.String()), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "lib", "mark.dart"), []byte("String src() => '';\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "rules.yaml"), []byte(r.String()), 0o644)
}

// onlyFromRecv: every flow of the row starts at the receiver (the row of a property).
func onlyFromRecv(p *model.Propagator) bool {
	for _, f := range p.Flows {
		if f.From != "recv" {
			return false
		}
	}
	return len(p.Flows) > 0
}
