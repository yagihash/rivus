package taint

import "strings"

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
