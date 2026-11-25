package gossa

import "golang.org/x/tools/go/packages"

// VerifiedGo is the minor of the Go language that this rivus was verified with: the go line of its own go.mod, minor only
// (a patch release does not change the language, and comparing it would warn at every update of the toolchain). A project whose modules say
// a newer go line is read by a rivus that was not checked against that language: summary.go_version says so (ADR-127). A test fails when the
// go line of go.mod and this value differ.
const VerifiedGo = "1.27"

// goVersions returns the module path of the main module and the go line of every loaded module that has one (the target and the
// dependencies: a dependency written for a newer Go is as unverified as the target). A module that has no go line (an old go.mod) is left
// out. A replacement says its own go line.
func goVersions(pkgs []*packages.Package) (string, map[string]string) {
	main := ""
	out := map[string]string{}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		m := pkg.Module
		if m == nil {
			return
		}
		if m.Main {
			main = m.Path
		}
		v := m.GoVersion
		if r := m.Replace; r != nil && r.GoVersion != "" {
			v = r.GoVersion
		}
		if v != "" {
			out[m.Path] = v
		}
	})
	return main, out
}
