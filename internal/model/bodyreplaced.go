package model

import (
	"fmt"
	"sort"
	"strings"
)

// BodyReplaced is the declaration of a package: the model stands for the code of the package, and the code is not
// read by the analysis (docs/DESIGN-package-models.md). A call of a function of the package has the model when the
// model has a row for it, and the uncertainty unmodeled_library_call when it has none (what -no-deps does for the
// package alone).
type BodyReplaced struct {
	// Package is the unit the way the functions are told apart: the import path of a Go package, `package:name` of a
	// Dart package, the name of a Rust crate. It names the package it says and no other (`example.com/m/a` does not
	// take `example.com/m/a/b`). With `/...` at the end it names the package and everything under it, as the go
	// tool does (`example.com/m/a/...`).
	Package string `yaml:"package" json:"package"`
	// Reason says why the model can stand for the code: what was checked (it is shown in the summary of the report).
	Reason string `yaml:"reason" json:"reason"`
}

// Subpackages is the ending of a package that names the package and what is under it.
const Subpackages = "/..."

// Tree reports whether the declaration names the package and what is under it.
func (b BodyReplaced) Tree() bool { return strings.HasSuffix(b.Package, Subpackages) }

// Root is the package without the ending that says its subpackages.
func (b BodyReplaced) Root() string { return strings.TrimSuffix(b.Package, Subpackages) }

// Covers reports whether the declaration names the unit.
func (b BodyReplaced) Covers(unit string) bool {
	root := b.Root()
	if unit == root || (!b.Tree() && sameCrateName(root, unit)) {
		return true
	}
	return b.Tree() && strings.HasPrefix(unit, root+"/")
}

// plainName reports whether the name has no separator of a path (`/`, `.`, `:`). A Go package outside the standard
// library has a dot in its path, a Dart package `package:`; what is left is a Rust crate (and the standard packages
// of Go, which have no `-` or `_`).
func plainName(s string) bool { return s != "" && !strings.ContainsAny(s, "/.:") }

// sameCrateName reports whether two names are the same Rust crate: Cargo reads `-` and `_` of a crate name as the same
// (`actix-web` of Cargo.lock and of a model, `actix_web` of rustc, which is what the units of the IR say).
func sameCrateName(a, b string) bool { return plainName(a) && plainName(b) && SameCrate(a, b) }

// CheckPackage says what is wrong with the name of a package in a declaration (-body-replaced of the flag, or
// body_replaced_packages of a rule file).
func CheckPackage(pkg string) error {
	root := strings.TrimSuffix(pkg, Subpackages)
	switch {
	case pkg == "" || root == "":
		return fmt.Errorf("a package is required (the import path of a Go package, package:name of a Dart package, the name of a Rust crate)")
	case strings.ContainsAny(pkg, " \t\r\n*"):
		return fmt.Errorf("%q is not the name of a package (no spaces, and no pattern other than a /... ending)", pkg)
	case strings.Contains(root, "..."):
		return fmt.Errorf("%q: ... is allowed only at the end, as /...", pkg)
	case root == "file:":
		return fmt.Errorf("%q is the code of the project itself: its body is what is analysed", pkg)
	case strings.HasSuffix(root, "/"):
		return fmt.Errorf("%q: write the package without a trailing slash", pkg)
	}
	return nil
}

func (rs *RuleSet) validateBodyReplaced() error {
	seen := map[string]bool{}
	for i, b := range rs.BodyReplaced {
		what := fmt.Sprintf("body_replaced_packages[%d]", i)
		if err := CheckPackage(b.Package); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		if strings.TrimSpace(b.Reason) == "" {
			return fmt.Errorf("%s: reason is required (what was checked for %s: the rows of the model, and the callbacks the package makes into the code)", what, b.Package)
		}
		if seen[b.Package] {
			return fmt.Errorf("%s: %s is declared twice", what, b.Package)
		}
		seen[b.Package] = true
	}
	return nil
}

// Declaration is a declaration and where it was made.
type Declaration struct {
	BodyReplaced
	// Source is the rule set (file, "builtin:..." or the flag) that declares it.
	Source string
}

// declaredSet answers whether a unit is named by any declaration.
type declaredSet struct {
	exact map[string]bool
	trees []string
}

func indexDeclared(ds []Declaration) *declaredSet {
	d := &declaredSet{exact: map[string]bool{}}
	for _, x := range ds {
		d.exact[x.Root()] = true
		if !x.Tree() && plainName(x.Root()) {
			d.exact[strings.ReplaceAll(x.Root(), "-", "_")] = true // the spelling of rustc
		}
		if x.Tree() {
			d.trees = append(d.trees, x.Root())
		}
	}
	return d
}

// HasDeclared reports whether any package is declared.
func (r *Rules) HasDeclared() bool { return r != nil && len(r.Declared) > 0 }

// ReplacedBody reports whether the unit is a package whose body the model replaces.
func (r *Rules) ReplacedBody(unit string) bool {
	if r == nil || r.declared == nil || unit == "" {
		return false
	}
	if r.declared.exact[unit] || (plainName(unit) && r.declared.exact[strings.ReplaceAll(unit, "-", "_")]) {
		return true
	}
	for _, t := range r.declared.trees {
		if strings.HasPrefix(unit, t+"/") {
			return true
		}
	}
	return false
}

// WithDeclarations returns the rules with these declarations in place of the ones the rule sets made (none: the
// packages are read as they are).
func (r *Rules) WithDeclarations(ds []Declaration) *Rules {
	x := *r
	x.Declared = ds
	x.declared = indexDeclared(ds)
	return &x
}

// mergeDeclarations lists the declarations of the rule sets, sorted by package and source (the report and the key of
// an index do not depend on the order the files were given in).
func mergeDeclarations(sets []*RuleSet) []Declaration {
	var out []Declaration
	for _, rs := range sets {
		for _, b := range rs.BodyReplaced {
			out = append(out, Declaration{BodyReplaced: b, Source: rs.Origin})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		return out[i].Source < out[j].Source
	})
	return out
}

// UnitOf is what a function ID says about the unit of code it belongs to: the
// package of a Go function (the import path), the package of a Dart library
// ("package:name"; the files of the project itself are one unit, "file:"), the
// crate of a Rust function.
func UnitOf(funcID string) string {
	switch {
	case strings.HasPrefix(funcID, "package:"):
		rest := funcID[len("package:"):]
		if i := strings.IndexAny(rest, "/#"); i >= 0 {
			rest = rest[:i]
		}
		return "package:" + rest
	case strings.HasPrefix(funcID, "file:"):
		return "file:"
	case strings.Contains(funcID, "::"):
		// "krate::path::f", "<krate::T as krate::Trait>::m"
		s := strings.TrimLeft(funcID, "<&*(")
		if i := strings.Index(s, "::"); i > 0 {
			return s[:i]
		}
	}
	return goPackageOf(funcID)
}

// goPackageOf extracts the import path from a function ID such as
// "example.com/m/pkg.F", "(*example.com/m/pkg.T).M" or "example.com/m/pkg.F$1".
func goPackageOf(funcID string) string {
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
