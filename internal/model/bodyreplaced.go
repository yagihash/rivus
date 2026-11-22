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
	if unit == root {
		return true
	}
	return b.Tree() && strings.HasPrefix(unit, root+"/")
}

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
	if r.declared.exact[unit] {
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
