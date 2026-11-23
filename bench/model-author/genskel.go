package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yagihash/rivus/internal/model"
)

// -gen-skeleton: the placement of the package in the Go zoo, the skeleton (skeleton.go), the module files, and the template of the
// cells (docs/DESIGN-package-models.md, 6.11, stage 2).
//
// Placement (the command decides; nothing is left to the person): a package of another module goes to bench/zoo-go-pkg/<name>/ with a
// skeleton of its own (the dependency form: the skeleton is a module of its own, and the cells read it as a dependency). A package of
// the standard library goes to bench/zoo-go-pkg/<name with _ for />/ without a skeleton (the cells call the real package, as the
// cells of os and database_sql do): bench/zoo-go is taken only when the Rust and Dart cells that pair with it exist, which this
// command does not decide, so the standard library is placed by the form of #219 and #223 only.
//
// What is generated (and overwritten each run): skel/**/<package>.go (first line GeneratedHeader), skel/go.mod and go.mod. What is made
// once and then left alone: rules.yaml, and the cells (the template is written only when cells/ has no Go file). A file of skel/ that has
// another name than the generated ones (for example zoo_extra.go) is the person's, and is never touched.

// goZooPkgDirName: the directory of the package in bench/zoo-go-pkg. A package of the standard library: its path with _ for /. A
// package of another module: the last element of the path of its module without a major (`github.com/google/go-github/v93` is
// go_github, `cloud.google.com/go/storage` is storage), so that every package of the module has one place.
func goZooPkgDirName(pkg, module string) string {
	if isStdlib(pkg) || module == "" {
		return strings.ReplaceAll(pkg, "/", "_")
	}
	parts := strings.Split(module, "/")
	last := parts[len(parts)-1]
	if majorElemRe.MatchString("/"+last) && len(parts) > 1 {
		last = parts[len(parts)-2]
	}
	return strings.NewReplacer("-", "_", ".", "_").Replace(last)
}

// ZooPkgPlace is where the package goes.
type ZooPkgPlace struct {
	Dir  string // bench/zoo-go-pkg/<name>, absolute
	Name string
}

func zooPlace(repo, pkg, module string) ZooPkgPlace {
	n := goZooPkgDirName(pkg, module)
	return ZooPkgPlace{Dir: filepath.Join(repo, "bench", "zoo-go-pkg", n), Name: n}
}

func zooGoVersion(repo string) string {
	b, err := os.ReadFile(filepath.Join(repo, "bench", "zoo-go", "go.mod"))
	if err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "go ") {
				return strings.TrimSpace(strings.TrimPrefix(l, "go "))
			}
		}
	}
	return "1.27"
}

// runGenSkeleton makes the skeleton (not for the standard library), the module files, and the cell template.
func runGenSkeleton(out io.Writer, o *options, repo string, facts *Facts, l *loaded) error {
	place := zooPlace(repo, o.pkg, l.Module)
	std := isStdlib(o.pkg)
	fmt.Fprintf(out, "model-author go %s %s: -gen-skeleton\n", o.pkg, l.Version)
	rel, _ := filepath.Rel(repo, place.Dir)
	if std {
		fmt.Fprintf(out, "  placement: %s (the standard library: no skeleton, the cells call the real package)\n", rel)
	} else {
		fmt.Fprintf(out, "  placement: %s (a package of another module: a skeleton of its own, read as a dependency)\n", rel)
	}
	if err := os.MkdirAll(place.Dir, 0o755); err != nil {
		return &exitError{ExitSkeleton, err}
	}
	gv := zooGoVersion(repo)
	if !std {
		res, err := GenerateSkeleton(&skelInput{Root: l.Types, Module: l.Module, Version: l.Version, ModOf: l.ModOf, Facts: facts})
		if err != nil {
			return &exitError{ExitSkeleton, err}
		}
		skel := filepath.Join(place.Dir, "skel")
		if err := writeSkeleton(skel, res.Files); err != nil {
			return &exitError{ExitSkeleton, err}
		}
		skelMod := GeneratedHeader + "\nmodule " + l.Module + "\n\ngo " + gv + "\n\nrequire zoo v0.0.0\n\nreplace zoo => ../../../zoo-go\n"
		if err := writeModFile(filepath.Join(skel, "go.mod"), skelMod); err != nil {
			return &exitError{ExitSkeleton, err}
		}
		if err := checkSkeletonOwner(place.Dir, o.pkg); err != nil {
			return &exitError{ExitSkeleton, err}
		}
		top := GeneratedHeader + "\n// Package: " + o.pkg + "\nmodule zoopkg/" + place.Name + "\n\ngo " + gv + "\n\nrequire (\n\t" + l.Module + " " + l.Version + "\n\tzoo v0.0.0\n)\n\n" +
			"replace zoo => ../../zoo-go\n\nreplace " + l.Module + " => ./skel\n"
		if err := writeModFile(filepath.Join(place.Dir, "go.mod"), top); err != nil {
			return &exitError{ExitSkeleton, err}
		}
		fmt.Fprintf(out, "  skeleton: %d file(s), %d type(s), %d function(s), %d method(s)\n", len(res.Files), res.Types, res.Funcs, res.Methods)
		slug := slugRe.ReplaceAllString(o.pkg, "_")
		if len(res.Omitted) > 0 {
			f := filepath.Join(o.work, slug+"-skeleton-omitted.txt")
			if err := os.WriteFile(f, []byte(strings.Join(res.Omitted, "\n")+"\n"), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(out, "  left out (a type of another module is needed): %d: %s\n", len(res.Omitted), f)
		}
		for _, r := range res.RowsNotReproduced {
			fmt.Fprintf(out, "  row not reproduced in the skeleton: %s\n", r)
		}
		if err := buildDir(skel); err != nil {
			return &exitError{ExitSkeleton, fmt.Errorf("the skeleton does not build: %v", err)}
		}
		fmt.Fprintln(out, "  skeleton builds")
	} else {
		top := GeneratedHeader + "\nmodule zoopkg/" + place.Name + "\n\ngo " + gv + "\n\nrequire zoo v0.0.0\n\nreplace zoo => ../../zoo-go\n"
		if err := writeModFile(filepath.Join(place.Dir, "go.mod"), top); err != nil {
			return &exitError{ExitSkeleton, err}
		}
	}
	if err := scaffoldRules(place, o.pkg, std); err != nil {
		return &exitError{ExitSkeleton, err}
	}
	made, err := writeCellTemplate(out, place, o.pkg, l, facts)
	if err != nil {
		return &exitError{ExitCells, err}
	}
	if made {
		if err := buildDir(place.Dir); err != nil {
			return &exitError{ExitCells, fmt.Errorf("the cell template does not build: %v", err)}
		}
		fmt.Fprintf(out, "  next: ZOO_MARKS_UPDATE=1 go test ./cmd/rivus -run '^TestZooMarksLock$' (the marks of the new cells), then check the cells with TestZooPkgGo\n")
	}
	return nil
}

// checkSkeletonOwner: the skeleton of a directory is of one package of the module (the one that go.mod names in `// Package:`); another
// package of the same module would remove its files.
func checkSkeletonOwner(dir, pkg string) error {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return nil
	}
	for _, l := range strings.Split(string(b), "\n") {
		if p, ok := strings.CutPrefix(l, "// Package: "); ok && p != pkg {
			return fmt.Errorf("%s has the skeleton of %s: one package of a module has a skeleton (the skeleton of %s would replace it)", dir, p, pkg)
		}
	}
	return nil
}

func buildDir(dir string) error {
	c := exec.Command("go", "build", "./...")
	c.Dir = dir
	c.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=")
	if b, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%v\n%s", err, b)
	}
	return nil
}

func hasGenHeader(b []byte) bool {
	return bytes.HasPrefix(b, []byte(GeneratedHeader))
}

// writeSkeleton writes the generated files, and removes the generated files of an earlier run that are not made now.
func writeSkeleton(skel string, files []skelFile) error {
	keep := map[string]bool{}
	for _, f := range files {
		p := filepath.Join(skel, f.Dir, f.Name)
		keep[p] = true
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(f.Content), 0o644); err != nil {
			return err
		}
	}
	var stale []string
	_ = filepath.Walk(skel, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || keep[p] {
			return nil
		}
		if b, rerr := os.ReadFile(p); rerr == nil && hasGenHeader(b) {
			stale = append(stale, p)
		}
		return nil
	})
	for _, p := range stale {
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	// the directories that this left empty
	var dirs []string
	_ = filepath.Walk(skel, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && p != skel {
			dirs = append(dirs, p)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		if es, err := os.ReadDir(d); err == nil && len(es) == 0 {
			os.Remove(d)
		}
	}
	return nil
}

// normMod is a go.mod without comments and blank lines.
func normMod(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "//") {
			out = append(out, t)
		}
	}
	return strings.Join(out, "\n")
}

// writeModFile writes a go.mod that the generator owns. A file that has no header (an earlier, hand written one) is replaced when it
// says the same; one that says something else is not touched: it has been edited by the person.
func writeModFile(p, content string) error {
	if b, err := os.ReadFile(p); err == nil && !hasGenHeader(b) && normMod(string(b)) != normMod(content) {
		return fmt.Errorf("%s was written by hand and says something else than the generator would (not touched): remove it, or make it say the same", p)
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

func scaffoldRules(place ZooPkgPlace, pkg string, std bool) error {
	p := filepath.Join(place.Dir, "rules.yaml")
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	var b strings.Builder
	if std {
		fmt.Fprintf(&b, "# Rules of the standard library package %s in the Go zoo. The sources, sinks and sanitizers of zoo/mark are in\n", pkg)
		b.WriteString("# bench/zoo-go/rules.yaml (the tests give both files); the rows of the package are the built-in model, which is in force as always.\n")
		b.WriteString("# The standard library has no skeleton: the cells call the real package, and the zoo records only the sinks of zoo/mark.\n")
	} else {
		fmt.Fprintf(&b, "# Rules of the package %s of the Go zoo. The sources, sinks and sanitizers of zoo/mark are in bench/zoo-go/rules.yaml (the tests\n", place.Name)
		fmt.Fprintf(&b, "# give both files); the rows of %s itself are the built-in model, which is in force as always.\n", pkg)
	}
	fmt.Fprintf(&b, "ruleset: rivus/bench-zoo-go-pkg-%s@1\n", place.Name)
	return os.WriteFile(p, []byte(b.String()), 0o644)
}

// ---- the template of the cells

type cellRow struct {
	sink model.Sink
	fn   *types.Func
}

// writeCellTemplate makes cells/<category>/<category>.go for each category of the sink rows of the package, when cells/ has no Go file.
func writeCellTemplate(out io.Writer, place ZooPkgPlace, pkg string, l *loaded, facts *Facts) (bool, error) {
	cells := filepath.Join(place.Dir, "cells")
	exists := false
	_ = filepath.Walk(cells, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".go") {
			exists = true
		}
		return nil
	})
	if exists {
		fmt.Fprintln(out, "  cells: the package has cells already: not touched")
		return false, nil
	}
	groups := map[string][]cellRow{}
	var skipped []string
	for _, s := range facts.SinkRows {
		fn := lookupFunc(l.Types, s.Symbol)
		if fn == nil {
			skipped = append(skipped, s.Symbol+" (not found)")
			continue
		}
		cat := s.Category
		if cat == "" {
			cat = "other"
		}
		groups[cat] = append(groups[cat], cellRow{sink: s, fn: fn})
	}
	if len(groups) == 0 {
		fmt.Fprintln(out, "  cells: no sink row of the package: no template (add the rows, and run again)")
		return false, nil
	}
	made := false
	var cats []string
	for c := range groups {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, cat := range cats {
		rows := groups[cat]
		sort.Slice(rows, func(i, j int) bool { return rows[i].sink.Symbol < rows[j].sink.Symbol })
		src, left := cellSource(place, pkg, l.Types, cat, rows)
		skipped = append(skipped, left...)
		if src == "" {
			continue
		}
		dir := filepath.Join(cells, cat)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return false, err
		}
		if err := os.WriteFile(filepath.Join(dir, cat+".go"), []byte(src), 0o644); err != nil {
			return false, err
		}
		fmt.Fprintf(out, "  cells: template %s (%d sink row(s) called)\n", filepath.Join("cells", cat), len(rows)-len(left))
		made = true
	}
	for _, s := range skipped {
		fmt.Fprintf(out, "  cells: no call for %s (the template cannot build it: write it by hand)\n", s)
	}
	return made, nil
}

// lookupFunc finds the function or the method of a symbol (`pkg.F`, `(*pkg.T).M`, `(pkg.T).M`) in the package.
func lookupFunc(pkg *types.Package, sym string) *types.Func {
	s := sym
	recv := ""
	if strings.HasPrefix(s, "(") {
		end := strings.Index(s, ")")
		if end < 0 || end+1 >= len(s) || s[end+1] != '.' {
			return nil
		}
		recv = strings.TrimPrefix(s[1:end], "*")
		s = s[end+2:]
		if !strings.HasPrefix(recv, pkg.Path()+".") {
			return nil
		}
		tn, _ := pkg.Scope().Lookup(strings.TrimPrefix(recv, pkg.Path()+".")).(*types.TypeName)
		if tn == nil {
			return nil
		}
		n, ok := tn.Type().(*types.Named)
		if !ok {
			return nil
		}
		for i := 0; i < n.NumMethods(); i++ {
			if n.Method(i).Name() == s {
				return n.Method(i)
			}
		}
		return nil
	}
	if !strings.HasPrefix(s, pkg.Path()+".") {
		return nil
	}
	fn, _ := pkg.Scope().Lookup(strings.TrimPrefix(s, pkg.Path()+".")).(*types.Func)
	return fn
}

type cellCtx struct {
	pkg     *types.Package
	imports map[string]string
	names   map[string]string
}

func (c *cellCtx) qual(p *types.Package) string {
	if p == nil {
		return ""
	}
	n, ok := c.imports[p.Path()]
	if !ok {
		n = p.Name()
		for i := 2; c.names[n] != "" || n == "mark"; i++ {
			n = p.Name() + strconv.Itoa(i)
		}
		c.imports[p.Path()] = n
		c.names[n] = p.Path()
	}
	return n
}

func (c *cellCtx) ts(t types.Type) string { return types.TypeString(t, c.qual) }

// zeroLit is an expression of a value of the type for an argument that is not dangerous ("", false if the template cannot make one).
func (c *cellCtx) zeroLit(t types.Type) (string, bool) {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Kind() == types.String:
			if _, named := t.(*types.Named); named {
				return c.ts(t) + `("")`, true
			}
			return `""`, true
		case u.Kind() == types.Bool:
			return "false", true
		case u.Info()&types.IsNumeric != 0:
			if _, named := t.(*types.Named); named {
				return c.ts(t) + "(0)", true
			}
			return "0", true
		}
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Signature, *types.Interface:
		if _, tp := t.(*types.TypeParam); !tp {
			return "nil", true
		}
	case *types.Struct:
		if n, ok := t.(*types.Named); ok && n.Obj().Exported() && n.TypeParams().Len() == 0 {
			return c.ts(t) + "{}", true
		}
	}
	return "", false
}

// dangerLit is an expression of the type t that holds the string s at the field path (or is s itself when the path is empty).
func (c *cellCtx) dangerLit(t types.Type, path []string, s string) (string, bool) {
	if len(path) == 0 {
		is, isb := stringKind(t)
		switch {
		case is:
			if bt, ok := t.(*types.Basic); ok && bt.Kind() == types.String {
				return s, true
			}
			return c.ts(t) + "(" + s + ")", true
		case isb:
			return c.ts(t) + "(" + s + ")", true
		case isInterface(t):
			return s, true
		}
		return "", false
	}
	base := t
	ptr := false
	if p, ok := t.Underlying().(*types.Pointer); ok {
		base, ptr = p.Elem(), true
	}
	st, ok := base.Underlying().(*types.Struct)
	if !ok {
		return "", false
	}
	if n, isNamed := base.(*types.Named); !isNamed || !n.Obj().Exported() || n.TypeParams().Len() > 0 {
		return "", false
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if f.Name() == path[0] && f.Exported() {
			inner, ok := c.dangerLit(f.Type(), path[1:], s)
			if !ok {
				return "", false
			}
			lit := c.ts(base) + "{" + f.Name() + ": " + inner + "}"
			if ptr {
				lit = "&" + lit
			}
			return lit, true
		}
	}
	return "", false
}

type cellCall struct {
	recvType string // the type of the receiver variable ("" for a function)
	call     string // the call, with %s where the receiver variable goes
	danger   bool
}

// buildCall makes the call of the row: the dangerous positions get s (the text that the section passes: a variable, or a constant).
func (c *cellCtx) buildCall(row cellRow, s string, recv string) (string, string, bool) {
	sig := row.fn.Type().(*types.Signature)
	if sig.TypeParams().Len() > 0 {
		return "", "", false
	}
	recvT := ""
	callee := ""
	if sig.Recv() != nil {
		rt := sig.Recv().Type()
		if p, ok := rt.(*types.Pointer); ok {
			rt = p.Elem()
		}
		n, ok := rt.(*types.Named)
		if !ok || isInterface(rt) || n.TypeParams().Len() > 0 || !n.Obj().Exported() {
			return "", "", false
		}
		recvT = c.ts(rt)
		callee = recv + "." + row.fn.Name()
	} else {
		callee = c.qual(row.fn.Pkg()) + "." + row.fn.Name()
	}
	whole := map[int]bool{}
	parts := map[int][]string{}
	for _, a := range row.sink.WholeArgs(sig.Params().Len()) {
		whole[a] = true
	}
	for _, p := range row.sink.PartList() {
		if p.Arg != nil && !strings.Contains(p.Field, "[]") {
			parts[*p.Arg] = strings.Split(p.Field, ".")
		}
	}
	var args []string
	hasDanger := false
	for i := 0; i < sig.Params().Len(); i++ {
		t := sig.Params().At(i).Type()
		if sig.Variadic() && i == sig.Params().Len()-1 {
			if whole[i] {
				e := t.(*types.Slice).Elem()
				if lit, ok := c.dangerLit(e, nil, s); ok {
					args = append(args, lit)
					hasDanger = true
				}
			}
			continue
		}
		switch {
		case whole[i]:
			lit, ok := c.dangerLit(t, nil, s)
			if !ok {
				if z, ok2 := c.zeroLit(t); ok2 && !isStringLike(t) {
					args = append(args, z)
					continue
				}
				return "", "", false
			}
			args = append(args, lit)
			hasDanger = true
		case parts[i] != nil:
			lit, ok := c.dangerLit(t, parts[i], s)
			if !ok {
				return "", "", false
			}
			args = append(args, lit)
			hasDanger = true
		default:
			z, ok := c.zeroLit(t)
			if !ok {
				return "", "", false
			}
			args = append(args, z)
		}
	}
	if !hasDanger {
		return "", "", false
	}
	return recvT, callee + "(" + strings.Join(args, ", ") + ")", true
}

func isStringLike(t types.Type) bool {
	a, b := stringKind(t)
	return a || b
}

func cleanExpr(cat, e string) string {
	switch cat {
	case "sql_injection":
		return "mark.CleanSQL(" + e + ")"
	case "command_injection", "resource_name":
		return "mark.CleanCmd(" + e + ")"
	case "xss":
		return "mark.CleanHTML(" + e + ")"
	case "path_traversal":
		return "mark.CleanPath(" + e + ")"
	}
	// a category that has no class of its own is recorded as every class: clean the text of all four
	return "mark.CleanPath(mark.CleanHTML(mark.CleanSQL(mark.CleanCmd(" + e + "))))"
}

func sinkFor(cat, e string) string { return recordStr(cat, e) }

func cellSource(place ZooPkgPlace, pkg string, tp *types.Package, cat string, rows []cellRow) (string, []string) {
	c := &cellCtx{pkg: tp, imports: map[string]string{}, names: map[string]string{}}
	c.imports[pkg] = tp.Name()
	c.names[tp.Name()] = pkg
	cell := strings.ReplaceAll(cat, "_", "-")
	pkgSlug := strings.ReplaceAll(place.Name, "_", "-")
	tag := "pkg-" + pkgSlug + "-" + cell
	var left []string

	// the receiver variables: one for each type of the rows, in the order of the first row
	body := func(section string) []string {
		var recvs []string
		recvVar := map[string]string{}
		var lines []string
		srcVar := "q"
		if section == "decoy" {
			srcVar = "t"
		}
		for _, r := range rows {
			arg := srcVar
			if section == "decoy" {
				arg = `"constant"`
			}
			tmpRecv := "RECV"
			rt, call, ok := c.buildCall(r, arg, tmpRecv)
			if !ok {
				continue
			}
			if rt != "" {
				v, seen := recvVar[rt]
				if !seen {
					v = "r" + strconv.Itoa(len(recvVar))
					recvVar[rt] = v
					recvs = append(recvs, v+" := new("+rt+")")
				}
				call = strings.Replace(call, tmpRecv, v, 1)
			}
			mark := ""
			switch section {
			case "vuln":
				mark = " // @sink(" + tag + "-vuln)"
			case "fixed":
				mark = " // @sink(none)"
			case "decoy":
				mark = " // @sink(none) @nosink(" + tag + "-decoy)"
			}
			lines = append(lines, call+mark)
		}
		var out []string
		switch section {
		case "vuln":
			out = append(out, "q := mark.Src(\""+tag+"-vuln\") // @source("+tag+"-vuln)")
		case "fixed":
			out = append(out, "q := "+cleanExpr(cat, "mark.Src(\""+tag+"-fixed\")"))
		case "decoy":
			out = append(out, "t := mark.Src(\""+tag+"-decoy\") // @source("+tag+"-decoy)")
		}
		out = append(out, recvs...)
		out = append(out, lines...)
		if section == "decoy" {
			out = append(out, sinkFor(cat, "t")+" // @sink("+tag+"-decoy)")
		}
		return out
	}
	// a row that no section can call is listed once, and a category that has no row to call has no cell
	var callable []cellRow
	for _, r := range rows {
		if _, _, ok := c.buildCall(r, "q", "r"); !ok {
			left = append(left, r.sink.Symbol)
		} else {
			callable = append(callable, r)
		}
	}
	if len(callable) == 0 {
		return "", left
	}
	rows = callable
	vuln, fixed, decoy := body("vuln"), body("fixed"), body("decoy")

	var b strings.Builder
	fmt.Fprintf(&b, "// A template that make model made from the sink rows of %s (category %s; generator: model-author skeleton %d). The command makes it once\n// and never overwrites a cell: check each call, and write what the template cannot.\n\n", pkg, cat, skelGenVersion)
	fmt.Fprintf(&b, "// Package %s is a cell of the package %s.\n//\n// @cell(F/pkg/%s/%s)\n// @shape: a value of a source reaches the sinks of the category %s of %s; the same value cleaned does not; a constant in the dangerous place does not\n", cat, place.Name, pkgSlug, cell, cat, pkg)
	fmt.Fprintf(&b, "package %s\n\n", cat)
	imp := []string{`"zoo/mark"`}
	for p, n := range c.imports {
		if n == path.Base(p) {
			imp = append(imp, strconv.Quote(p))
		} else {
			imp = append(imp, n+" "+strconv.Quote(p))
		}
	}
	sort.Slice(imp, func(i, j int) bool { return importPath(imp[i]) < importPath(imp[j]) })
	b.WriteString("import (\n")
	for _, i := range imp {
		b.WriteString("\t" + i + "\n")
	}
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "func init() {\n\tmark.Case(\"F/pkg/%s/%s/vuln\", vuln)\n\tmark.Case(\"F/pkg/%s/%s/fixed\", fixed)\n\tmark.Case(\"F/pkg/%s/%s/decoy\", decoy)\n}\n\n", pkgSlug, cell, pkgSlug, cell, pkgSlug, cell)
	for _, s := range []struct {
		name string
		body []string
	}{{"vuln", vuln}, {"fixed", fixed}, {"decoy", decoy}} {
		fmt.Fprintf(&b, "func %s() { // @%s\n", s.name, s.name)
		for _, l := range s.body {
			b.WriteString("\t" + l + "\n")
		}
		b.WriteString("}\n\n")
	}
	return gofmtOr(b.String()), left
}

func gofmtOr(src string) string {
	if out, err := format.Source([]byte(src)); err == nil {
		return string(out)
	}
	return src
}
