package main

import (
	"fmt"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/yagihash/rivus/internal/model"
)

// The generator for Go: the functions of the rows of the model, found with go/types in the package, are called one by one with the
// source in one position; every place that the call writes into has a sink of its own (`sinkN_<place>`).
// Not made, and said so ("unverified"): generics, callbacks, a type that the code cannot build or name, an interface that
// has no known implementation, a receiver that the source cannot be put into.

// Gen is what a generator makes.
type Gen struct {
	Probes   []Probe
	Skips    []Skip
	Dir      string // the code that the models are applied to (B)
	DirA     string // the code that has the bodies (A; the same as Dir unless the package is of the standard library)
	HasFlows bool
	Errors   []string
}

type goGen struct {
	pkg     *types.Package
	rows    []Row
	imports map[string]string // import path -> alias
	funcs   []string
	sinks   []string
	probes  []Probe
	skips   []Skip
	errs    []string
}

type goTarget struct {
	fn      *types.Func
	sig     *types.Signature
	recv    *types.Named // nil for a function
	ptrRecv bool
}

// GoOptions: where the fixture goes and what is read.
type GoOptions struct {
	Pkg, Out, Local, Version string
	Rows                     []Row
}

func goTool(dir string, args ...string) error {
	c := exec.Command("go", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

var modRe = regexp.MustCompile(`(?m)^module\s+(\S+)`)

// GenGo writes the fixture of Go and returns the probes.
func GenGo(o GoOptions) (*Gen, error) {
	dir := filepath.Join(o.Out, "fx")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	gomod := "module fx\n\ngo " + goVersion() + "\n"
	if o.Local != "" {
		abs, _ := filepath.Abs(o.Local)
		b, err := os.ReadFile(filepath.Join(abs, "go.mod"))
		if err != nil {
			return nil, err
		}
		m := modRe.FindSubmatch(b)
		gomod += fmt.Sprintf("\nrequire %s v0.0.0\nreplace %s => %s\n", m[1], m[1], abs)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		return nil, err
	}
	stdlib := !strings.Contains(strings.SplitN(o.Pkg, "/", 2)[0], ".")
	if o.Local == "" && !stdlib {
		at := o.Pkg
		if o.Version != "" {
			at += "@" + o.Version
		}
		if err := goTool(dir, "get", at); err != nil {
			return nil, err
		}
	}
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps, Dir: dir}
	pkgs, err := packages.Load(cfg, o.Pkg)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 || pkgs[0].Types == nil || len(pkgs[0].Errors) > 0 {
		return nil, fmt.Errorf("cannot load %s: %v", o.Pkg, pkgs[0].Errors)
	}
	g := &goGen{pkg: pkgs[0].Types, rows: o.Rows, imports: map[string]string{}}
	g.imports[o.Pkg] = "tp"
	g.build()
	gen := &Gen{Probes: g.probes, Skips: g.skips, Dir: dir, DirA: dir, Errors: g.errs}
	for _, p := range g.probes {
		if p.Kind == "flow" {
			gen.HasFlows = true
		}
	}
	if err := g.write(dir, o.Pkg, ""); err != nil {
		return nil, err
	}
	if stdlib && gen.HasFlows {
		a, err := mirrorStd(o, g, dir)
		if err != nil {
			// the bodies cannot be read: the flows are not checked (said), the sinks are
			var keep []Probe
			seen := map[string]bool{}
			for _, p := range gen.Probes {
				if p.Kind == "flow" {
					if !seen[p.Symbol] {
						seen[p.Symbol] = true
						gen.Skips = append(gen.Skips, Skip{Symbol: p.Symbol, Reason: "the body of the standard library cannot be copied here: " + err.Error()})
					}
					continue
				}
				keep = append(keep, p)
			}
			gen.Probes, gen.HasFlows = keep, false
		} else {
			gen.DirA = a
		}
	}
	if err := goTool(dir, "mod", "tidy"); err != nil {
		return nil, err
	}
	return gen, nil
}

// qual names the packages of the types: the package under test is `tp`, the others get an alias.
func (g *goGen) qual(p *types.Package) string {
	if a, ok := g.imports[p.Path()]; ok {
		return a
	}
	a := fmt.Sprintf("q%d", len(g.imports))
	g.imports[p.Path()] = a
	return a
}

func (g *goGen) ts(t types.Type) string { return types.TypeString(t, g.qual) }

// nameable: the code can write the type (no unexported name of another package, no internal package).
func nameable(t types.Type) bool {
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		o := t.Obj()
		if o.Pkg() != nil && (!o.Exported() || strings.Contains(o.Pkg().Path()+"/", "/internal/") || strings.HasPrefix(o.Pkg().Path(), "internal/")) {
			return false
		}
		if t.TypeArgs().Len() > 0 || t.TypeParams().Len() > 0 {
			return false
		}
		return true
	case *types.Basic:
		return true
	case *types.Pointer:
		return nameable(t.Elem())
	case *types.Slice:
		return nameable(t.Elem())
	case *types.Array:
		return nameable(t.Elem())
	case *types.Map:
		return nameable(t.Key()) && nameable(t.Elem())
	case *types.Interface:
		return t.NumMethods() == 0
	}
	return false
}

func isBasic(t types.Type, info types.BasicInfo) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&info != 0
}

// val is the expression of a value of the type: the source (taint) or a constant. ok is false when the code cannot make one.
func (g *goGen) val(t types.Type, taint bool) (string, bool) {
	t = types.Unalias(t)
	name := t.String()
	switch name {
	case "error":
		return map[bool]string{true: "terr{s}", false: `terr{"c"}`}[taint], true
	case "io.Reader":
		return map[bool]string{true: "trd{s}", false: `trd{"c"}`}[taint], true
	case "io.Writer":
		return "&twr{}", !taint
	case "any", "interface{}":
		return map[bool]string{true: "s", false: `"c"`}[taint], true
	}
	if !nameable(t) {
		if i, ok := t.Underlying().(*types.Interface); ok && i.NumMethods() > 0 {
			return "", false
		}
		return "", false
	}
	ty := g.ts(t)
	named := name != t.Underlying().String()
	conv := func(e string) string {
		if named {
			return ty + "(" + e + ")"
		}
		return e
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsString != 0:
			return conv(map[bool]string{true: "s", false: `"c"`}[taint]), true
		case taint:
			return "", false
		case u.Info()&types.IsBoolean != 0:
			return ty + "(false)", true
		case u.Info()&(types.IsInteger|types.IsFloat) != 0:
			return ty + "(0)", true
		}
	case *types.Slice:
		if isBasic(u.Elem(), types.IsInteger) && u.Elem().Underlying().(*types.Basic).Kind() == types.Byte {
			return conv(`[]byte(` + map[bool]string{true: "s", false: `"c"`}[taint] + `)`), true
		}
		if isBasic(u.Elem(), types.IsString) && nameable(u.Elem()) {
			e, _ := g.val(u.Elem(), taint)
			return ty + "{" + e + "}", true
		}
		if !taint {
			return "nil", true
		}
	case *types.Map:
		if !taint {
			return "nil", true
		}
		if isBasic(u.Key(), types.IsString) && isBasic(u.Elem(), types.IsString) {
			k, _ := g.val(u.Key(), false)
			e, _ := g.val(u.Elem(), true)
			return ty + "{" + k + ": " + e + "}", true
		}
	case *types.Pointer:
		if !taint && nameable(u.Elem()) {
			return "new(" + g.ts(u.Elem()) + ")", true
		}
	case *types.Struct:
		if !taint {
			return ty + "{}", true
		}
	case *types.Interface:
		if !taint {
			return "nil", true
		}
	}
	return "", false
}

func (g *goGen) build() {
	seen := map[string]bool{}
	var rows []Row
	for _, r := range g.rows {
		if !seen[r.Symbol+fmt.Sprint(r.Sink != nil)] {
			seen[r.Symbol+fmt.Sprint(r.Sink != nil)] = true
			rows = append(rows, r)
		}
	}
	for _, r := range rows {
		t, why := g.resolve(r.Symbol)
		if t == nil {
			g.skip(r.Symbol, "", why)
			continue
		}
		if r.Sink != nil {
			g.sinkProbes(r, t)
		} else {
			g.flowProbes(r, t)
		}
	}
}

func (g *goGen) skip(sym, variant, why string) {
	g.skips = append(g.skips, Skip{Symbol: sym, Variant: variant, Reason: why})
}

var methodRe = regexp.MustCompile(`^\((\*?)(.+)\)\.(\w+)$`)

// resolve finds the function of a symbol of the model in the package.
func (g *goGen) resolve(sym string) (*goTarget, string) {
	pkgPath := g.pkg.Path()
	if m := methodRe.FindStringSubmatch(sym); m != nil {
		tn := strings.TrimPrefix(m[2], pkgPath+".")
		obj, _ := g.pkg.Scope().Lookup(tn).(*types.TypeName)
		if obj == nil {
			return nil, "type not found in the package"
		}
		named, ok := types.Unalias(obj.Type()).(*types.Named)
		if !ok || named.TypeParams().Len() > 0 {
			return nil, "generic type"
		}
		if _, isIface := named.Underlying().(*types.Interface); isIface {
			return nil, "method of an interface: no implementation to call (unverified)"
		}
		sel, _, _ := types.LookupFieldOrMethod(types.NewPointer(named), true, g.pkg, m[3])
		fn, _ := sel.(*types.Func)
		if fn == nil {
			return nil, "method not found"
		}
		return g.target(fn, named, m[1] == "*")
	}
	fn, _ := g.pkg.Scope().Lookup(strings.TrimPrefix(sym, pkgPath+".")).(*types.Func)
	if fn == nil {
		return nil, "function not found in the package"
	}
	return g.target(fn, nil, false)
}

func (g *goGen) target(fn *types.Func, recv *types.Named, ptr bool) (*goTarget, string) {
	sig := fn.Type().(*types.Signature)
	if sig.TypeParams().Len() > 0 {
		return nil, "generic function (not checked)"
	}
	if recv != nil && recv.Obj().Pkg() != g.pkg {
		return nil, "method of a type of another package"
	}
	if recv != nil && (!nameable(recv) || !fn.Exported()) {
		return nil, "type or method that the code cannot name"
	}
	for i := 0; i < sig.Params().Len(); i++ {
		t := sig.Params().At(i).Type()
		if sig.Variadic() && i == sig.Params().Len()-1 {
			t = t.(*types.Slice).Elem()
		}
		if _, ok := t.Underlying().(*types.Signature); ok {
			return nil, "takes a callback (not checked)"
		}
		if _, ok := g.val(t, false); !ok {
			return nil, "argument type that the code cannot build: " + t.String()
		}
	}
	return &goTarget{fn: fn, sig: sig, recv: recv, ptrRecv: ptr}, ""
}

func paramType(sig *types.Signature, i int) types.Type {
	t := sig.Params().At(i).Type()
	if sig.Variadic() && i == sig.Params().Len()-1 {
		t = t.(*types.Slice).Elem()
	}
	return t
}

// recvSetup is the statements that make the receiver `rc`: a constant one, or one that has the source.
func (g *goGen) recvSetup(t *goTarget, taint bool) (string, string, bool) {
	tn := g.ts(t.recv)
	if !taint {
		return "rc := new(" + tn + ")", "", true
	}
	if e, ok := g.val(t.recv, true); ok {
		return "rc := new(" + tn + ")\n\t*rc = " + e, "", true
	}
	// a constructor of the package that takes the source: `New...` (the way that a user gets such a value)
	var names []string
	for _, n := range g.pkg.Scope().Names() {
		if strings.HasPrefix(n, "New") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		fn, _ := g.pkg.Scope().Lookup(n).(*types.Func)
		if fn == nil {
			continue
		}
		sig := fn.Type().(*types.Signature)
		if sig.TypeParams().Len() > 0 || sig.Results().Len() == 0 || sig.Variadic() {
			continue
		}
		rt := types.Unalias(sig.Results().At(0).Type())
		if p, ok := rt.(*types.Pointer); ok {
			rt = p.Elem()
		}
		if rt != types.Type(t.recv) {
			continue
		}
		var args []string
		tainted := false
		ok := true
		for i := 0; i < sig.Params().Len(); i++ {
			pt := sig.Params().At(i).Type()
			if e, k := g.val(pt, !tainted); k && !tainted && e != "" && func() bool { _, tk := g.val(pt, true); return tk }() {
				args = append(args, e)
				tainted = true
			} else if e, k := g.val(pt, false); k {
				args = append(args, e)
			} else {
				ok = false
			}
		}
		if !ok || !tainted {
			continue
		}
		lhs := "rc"
		if sig.Results().Len() == 2 {
			lhs = "rc, _"
		}
		return lhs + " := tp." + n + "(" + strings.Join(args, ", ") + ")", "via " + n, true
	}
	return "", "", false
}

type slot struct{ name, expr string }

func (g *goGen) nextID() int { return len(g.probes) }

// emit writes the function fixN: the setup, the call and the sinks of the places that the call writes into.
func (g *goGen) emit(sym, kind, variant string, t *goTarget, taintPos int, recvTaint bool, expect *bool, category string, partField string, outputs bool) {
	id := g.nextID()
	var sb strings.Builder
	fmt.Fprintf(&sb, "func fix%d() { // %s %s\n\ts := src()\n\t_ = s\n", id, sym, variant)
	recvName := ""
	if t.recv != nil {
		setup, via, _ := g.recvSetup(t, recvTaint)
		if recvTaint {
			variant += " (" + via + ")"
		}
		sb.WriteString("\t" + strings.ReplaceAll(setup, "\n\t", "\n\t") + "\n\t_ = rc\n")
		recvName = "rc."
	}
	var args []string
	var outs []string
	for i := 0; i < t.sig.Params().Len(); i++ {
		pt := paramType(t.sig, i)
		var e string
		if i == taintPos && partField != "" {
			e = g.fieldValue(pt, partField)
		} else {
			e, _ = g.val(pt, i == taintPos)
		}
		name := fmt.Sprintf("a%d", i)
		fmt.Fprintf(&sb, "\tvar %s %s = %s\n\t_ = %s\n", name, g.ts(pt), e, name)
		args = append(args, name)
		if i != taintPos && g.carries(pt) {
			outs = append(outs, fmt.Sprintf("arg%d:%s", i, name))
		}
	}
	call := fmt.Sprintf("%s%s(%s)", recvName, t.fn.Name(), strings.Join(args, ", "))
	if t.recv == nil {
		call = fmt.Sprintf("tp.%s(%s)", t.fn.Name(), strings.Join(args, ", "))
	}
	var places []string
	if outputs {
		var lhs []string
		for i := 0; i < t.sig.Results().Len(); i++ {
			rt := t.sig.Results().At(i).Type()
			if isBasic(rt, types.IsNumeric|types.IsBoolean) {
				lhs = append(lhs, "_")
				continue
			}
			lhs = append(lhs, fmt.Sprintf("r%d", i))
			outs = append(outs, fmt.Sprintf("result%d:r%d", i, i))
		}
		if strings.Join(lhs, "") != strings.Repeat("_", len(lhs)) {
			sb.WriteString("\t" + strings.Join(lhs, ", ") + " := " + call + "\n")
		} else {
			sb.WriteString("\t" + call + "\n")
		}
		if t.recv != nil && !(recvTaint) && (t.ptrRecv || g.carries(t.recv)) {
			outs = append(outs, "recv:rc")
		}
		for _, o := range outs {
			p := strings.SplitN(o, ":", 2)
			fmt.Fprintf(&sb, "\tsink%d_%s(%s)\n", id, p[0], p[1])
			g.sinks = append(g.sinks, fmt.Sprintf("sink%d_%s", id, p[0]))
			places = append(places, p[0])
		}
	} else {
		sb.WriteString("\t" + call + "\n")
	}
	sb.WriteString("}\n")
	g.funcs = append(g.funcs, sb.String())
	g.probes = append(g.probes, Probe{ID: id, Symbol: sym, Kind: kind, Variant: variant, Outputs: places, Expect: expect, Category: category})
}

// carries: a value that a call can write into (a pointer, a slice, a map, a writer).
func (g *goGen) carries(t types.Type) bool {
	if t.String() == "io.Writer" {
		return true
	}
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map:
		return true
	}
	return false
}

func (g *goGen) fieldValue(t types.Type, field string) string {
	if p, ok := t.Underlying().(*types.Pointer); ok {
		return "&" + g.ts(p.Elem()) + "{" + field + ": s}"
	}
	return g.ts(t) + "{" + field + ": s}"
}

func (g *goGen) flowProbes(r Row, t *goTarget) {
	n := 0
	if t.recv != nil {
		if _, _, ok := g.recvSetup(t, true); ok {
			g.emit(r.Symbol, "flow", "recv", t, -1, true, nil, "", "", true)
			n++
		} else {
			g.skip(r.Symbol, "recv", "the source cannot be put into the receiver (no conversion, no constructor New... that takes it)")
		}
	}
	for i := 0; i < t.sig.Params().Len(); i++ {
		if _, ok := g.val(paramType(t.sig, i), true); !ok {
			continue
		}
		g.emit(r.Symbol, "flow", fmt.Sprintf("arg%d", i), t, i, false, nil, "", "", true)
		n++
	}
	if n == 0 && t.recv == nil {
		g.skip(r.Symbol, "", "no position that the source can be put into")
	}
}

func (g *goGen) sinkProbes(r Row, t *goTarget) {
	s := r.Sink
	if len(s.Named) > 0 || s.WhenArgType != nil || s.Match != "" {
		g.skip(r.Symbol, "", "sink row with named / when_arg_type / match (not checked)")
		return
	}
	yes, no := true, false
	cat := s.Category
	np := t.sig.Params().Len()
	for _, a := range s.Args {
		if a >= np && !(t.sig.Variadic() && np > 0) {
			g.errs = append(g.errs, fmt.Sprintf("%s: the row declares arg%d, which the function does not have (%d parameters)", r.Symbol, a, np))
		}
	}
	declared := func(i int) bool {
		if len(s.Args) == 0 && len(s.Parts) == 0 && s.Field == "" {
			return true
		}
		for _, a := range s.Args {
			if a == i || t.sig.Variadic() && i == np-1 && a >= i {
				return true
			}
		}
		return false
	}
	parts := s.Parts
	if s.Field != "" {
		for _, a := range s.Args {
			parts = append(parts, model.SinkPart{Arg: &a, Field: s.Field})
		}
	}
	if len(parts) > 0 {
		for _, p := range parts {
			if p.Arg == nil || strings.Contains(p.Field, ".") || *p.Arg >= np {
				g.skip(r.Symbol, "", "sink field path that is not one field of an argument (not checked)")
				continue
			}
			g.emit(r.Symbol, "sink", fmt.Sprintf("arg%d.%s", *p.Arg, p.Field), t, *p.Arg, false, &yes, cat, p.Field, false)
		}
	} else {
		for i := 0; i < np; i++ {
			if _, ok := g.val(paramType(t.sig, i), true); !ok {
				if declared(i) {
					g.skip(r.Symbol, fmt.Sprintf("arg%d", i), "declared position whose type the source cannot be put into")
				}
				continue
			}
			e := &no
			if declared(i) {
				e = &yes
			}
			g.emit(r.Symbol, "sink", fmt.Sprintf("arg%d", i), t, i, false, e, cat, "", false)
		}
	}
	if t.recv != nil {
		if _, _, ok := g.recvSetup(t, true); ok {
			e := &no
			if s.Recv {
				e = &yes
			}
			g.emit(r.Symbol, "sink", "recv", t, -1, true, e, cat, "", false)
		} else if s.Recv {
			g.skip(r.Symbol, "recv", "declared receiver that the source cannot be put into")
		}
	}
	g.emit(r.Symbol, "sink", "const", t, -1, false, &no, cat, "", false)
}

func goVersion() string {
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	v := strings.TrimPrefix(strings.TrimSpace(string(out)), "go")
	if err != nil || v == "" {
		return "1.22"
	}
	if i := strings.Index(v, "-"); i > 0 {
		v = v[:i]
	}
	return v
}

const goHelpers = `
func src() string { return "" }

type terr struct{ s string }

func (e terr) Error() string { return e.s }

type trd struct{ s string }

func (r trd) Read(p []byte) (int, error) { return copy(p, r.s), nil }

type twr struct{ b []byte }

func (w *twr) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }
`

// write writes main.go (the calls), rules.yaml (the source and the sinks of the places) and go.mod of the dir.
func (g *goGen) write(dir, pkgPath, importPath string) error {
	if importPath == "" {
		importPath = pkgPath
	}
	body := strings.Join(g.funcs, "\n")
	var imps []string
	for path, a := range g.imports {
		if path == pkgPath {
			path = importPath
		}
		if strings.Contains(body, a+".") {
			imps = append(imps, fmt.Sprintf("\t%s %q", a, path))
		}
	}
	sort.Strings(imps)
	var sb strings.Builder
	sb.WriteString("package main\n\nimport (\n" + strings.Join(imps, "\n") + "\n)\n" + goHelpers + "\n")
	sb.WriteString(body)
	var calls []string
	for _, p := range g.probes {
		calls = append(calls, fmt.Sprintf("\tfix%d()", p.ID))
	}
	sb.WriteString("\nfunc main() {\n" + strings.Join(calls, "\n") + "\n}\n")
	for _, s := range g.sinks {
		fmt.Fprintf(&sb, "func %s(v any) {}\n", s)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(sb.String()), 0o644); err != nil {
		return err
	}
	var r strings.Builder
	r.WriteString("ruleset: diff/fx@1\nsources:\n  - symbol: fx.src\n    labels: [user_input]\nsinks:\n")
	for _, s := range g.sinks {
		fmt.Fprintf(&r, "  - symbol: fx.%s\n    category: model_diff\n", s)
	}
	return os.WriteFile(filepath.Join(dir, "rules.yaml"), []byte(r.String()), 0o644)
}
