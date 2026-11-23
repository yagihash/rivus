package model

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// VerifiedVersion records which versions of a package the rows of a model were checked against
// (docs/DESIGN-package-models.md, 6.7). It is a record: the analysis does not read it, and a row does not
// change with it. Comparing it with the versions a project uses is a separate piece of work.
type VerifiedVersion struct {
	// Package is the module path (Go), the package name (Dart) or the crate (Rust).
	Package string `yaml:"package" json:"package"`
	// Versions is the range that was checked: a version, or comparators (`^1.4.0`, `>=1.0.0 <2.0.0`, `~1.2`).
	Versions string `yaml:"versions" json:"versions"`
	// CheckedWith is the one version that was read, and is inside Versions (optional).
	CheckedWith string `yaml:"checked_with" json:"checked_with,omitempty"`
	// Checked is the day, YYYY-MM-DD.
	Checked string `yaml:"checked" json:"checked"`
}

var (
	versionRe = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
	opRe      = regexp.MustCompile(`^(\^|~|>=|<=|>|<|=)?(.*)$`)
)

type semver [3]int

func parseVersion(s string, full bool) (semver, bool) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil || (full && m[3] == "") {
		return semver{}, false
	}
	var v semver
	for i := 0; i < 3; i++ {
		if m[i+1] != "" {
			n, err := strconv.Atoi(m[i+1])
			if err != nil {
				return semver{}, false
			}
			v[i] = n
		}
	}
	return v, true
}

func (a semver) less(b semver) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// bound is one term of a range (blanks and commas separate the terms).
type bound struct {
	op string
	v  semver
	// parts is how many numbers the term wrote (1.2 is two): ^ and ~ stay under the next of the last one that counts.
	parts int
}

func parseRange(s string) ([]bound, error) {
	terms := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' })
	if len(terms) == 0 {
		return nil, fmt.Errorf("the range is empty")
	}
	var out []bound
	for _, t := range terms {
		m := opRe.FindStringSubmatch(t)
		op, rest := m[1], m[2]
		if op == "" {
			op = "="
		}
		v, ok := parseVersion(rest, false)
		if !ok {
			return nil, fmt.Errorf("%q is not a version or a comparator (want 1.2.3, v1.2.3, ^1.2.0, ~1.2, >=1.0.0 <2.0.0)", t)
		}
		parts := strings.Count(strings.SplitN(strings.SplitN(strings.TrimPrefix(rest, "v"), "-", 2)[0], "+", 2)[0], ".") + 1
		out = append(out, bound{op: op, v: v, parts: parts})
	}
	return out, nil
}

func (b bound) holds(v semver) bool {
	switch b.op {
	case ">":
		return b.v.less(v)
	case ">=":
		return !v.less(b.v)
	case "<":
		return v.less(b.v)
	case "<=":
		return !b.v.less(v)
	case "~":
		hi := semver{b.v[0], b.v[1] + 1, 0}
		if b.parts == 1 {
			hi = semver{b.v[0] + 1, 0, 0}
		}
		return !v.less(b.v) && v.less(hi)
	case "^":
		hi := semver{b.v[0] + 1, 0, 0}
		if b.v[0] == 0 && b.parts > 1 {
			hi = semver{0, b.v[1] + 1, 0}
			if b.v[1] == 0 && b.parts > 2 {
				hi = semver{0, 0, b.v[2] + 1}
			}
		}
		return !v.less(b.v) && v.less(hi)
	}
	// "=": the numbers written (1.2 is 1.2.x)
	switch b.parts {
	case 1:
		return v[0] == b.v[0]
	case 2:
		return v[0] == b.v[0] && v[1] == b.v[1]
	}
	return v == b.v
}

// Validate says what is wrong with one record.
func (e VerifiedVersion) Validate(today time.Time) error {
	// a Go module whose path has the major in it is written with /v* (any major; MajorWildcard): the rest is checked as it is written
	if err := CheckMajorWildcard(e.Package, "package"); err != nil {
		return err
	}
	if err := CheckPackage(strings.Replace(e.Package, MajorWildcard, "/v2", 1)); err != nil {
		return err
	}
	if strings.Contains(e.Package, "...") {
		return fmt.Errorf("%q: a record is for one package, not a pattern", e.Package)
	}
	bs, err := parseRange(e.Versions)
	if err != nil {
		return fmt.Errorf("versions: %w", err)
	}
	if e.Checked == "" {
		return fmt.Errorf("checked is required (the day, YYYY-MM-DD)")
	}
	day, err := time.Parse("2006-01-02", e.Checked)
	if err != nil {
		return fmt.Errorf("checked: %q is not a day (YYYY-MM-DD)", e.Checked)
	}
	if day.After(today.AddDate(0, 0, 1)) {
		return fmt.Errorf("checked: %s is in the future", e.Checked)
	}
	if e.CheckedWith != "" {
		v, ok := parseVersion(e.CheckedWith, true)
		if !ok || strings.ContainsAny(e.CheckedWith, "^~<>= ") {
			return fmt.Errorf("checked_with: %q is not one version (1.2.3 or v1.2.3)", e.CheckedWith)
		}
		for _, b := range bs {
			if !b.holds(v) {
				return fmt.Errorf("checked_with: %s is outside versions %q", e.CheckedWith, e.Versions)
			}
		}
	}
	return nil
}

func (rs *RuleSet) validateVerified() error {
	seen := map[string]bool{}
	for i, e := range rs.VerifiedVersions {
		what := fmt.Sprintf("verified_versions[%d]", i)
		if err := e.Validate(time.Now()); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		if seen[e.Package] {
			return fmt.Errorf("%s: %s is recorded twice", what, e.Package)
		}
		seen[e.Package] = true
	}
	return nil
}

// Verified is a record and the rule set it is in.
type Verified struct {
	VerifiedVersion
	// Source is the rule set (file or "builtin:...") that has the record.
	Source string `json:"source"`
	// Lang is the language of the rule set ("go", "dart", "rust"; see RuleSetLanguage), "" when the file does not say. Not in the JSON
	// of `rivus models versions`.
	Lang string `json:"-"`
}

// RuleSetLanguage is the language that a rule set is for, from the name of a built-in file (`builtin:dart-dio.yaml`) or the name of
// the rule set (`rivus/dart/dio@1`). "" for any other file (a rule file of a project has no marker of the language): its records are
// compared with the dependencies of every language.
func RuleSetLanguage(rs *RuleSet) string {
	for _, lang := range []string{"go", "dart", "rust"} {
		if strings.HasPrefix(rs.Origin, "builtin:"+lang+"-") || strings.HasPrefix(rs.Name, "rivus/"+lang+"/") {
			return lang
		}
	}
	return ""
}

func mergeVerified(sets []*RuleSet) []Verified {
	var out []Verified
	for _, rs := range sets {
		for _, e := range rs.VerifiedVersions {
			out = append(out, Verified{e, rs.Origin, RuleSetLanguage(rs)})
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

// Holds reports whether the version (v1.2.3, 1.2.3, with a pre-release or +incompatible suffix that is not read) is in the range
// of the record. A text that is not a version is not in it.
func (e VerifiedVersion) Holds(version string) bool {
	v, ok := parseVersion(version, false)
	if !ok {
		return false
	}
	bs, err := parseRange(e.Versions)
	if err != nil {
		return false
	}
	for _, b := range bs {
		if !b.holds(v) {
			return false
		}
	}
	return true
}

// UnverifiedVersion is a package of the program that a model has a verified_versions record for (the record names the package,
// or names it by /v*), used at a version that no record covers: the rows were applied to a version that nobody checked them
// against.
type UnverifiedVersion struct {
	// Package is the module path (Go; with the major that the program has, when the record says /v*), the pub package (Dart)
	// or the crate (Rust) that the program uses.
	Package string `json:"package"`
	// Used is the version the program is built with. When the lock file has several versions of the package (Cargo.lock can),
	// it is the first one (in the order of the file) that is outside the ranges.
	Used string `json:"used"`
	// AllUsed are all the versions of the package that the program uses, when there are several (sorted as the lock file has
	// them). Empty for one version: the JSON of Go has no such key.
	AllUsed []string `json:"all_used,omitempty"`
	// Verified are the ranges of the records of the package (all of them, sorted).
	Verified []string `json:"verified"`
	// Source says where Used comes from (go.mod, pubspec.lock, Cargo.lock).
	Source string `json:"source"`
}

// UnverifiedVersions lists the modules of the program (module path -> version) that have a record, and are used at a version
// outside every range of their records. A module with no record is not listed: most built-in files have none, and a note on
// every run for them would say nothing. Sorted by package. Nil when there is none.
func (r *Rules) UnverifiedVersions(used map[string]string, source string) []UnverifiedVersion {
	many := make(map[string][]string, len(used))
	for mod, ver := range used {
		many[mod] = []string{ver}
	}
	return r.UnverifiedVersionsOf(many, "go", source, MatchMajor)
}

// UnverifiedVersionsOf is UnverifiedVersions for a lock file: a package can be there at several versions, and it is listed when any
// of them is outside every range of its records (once, with the versions). match says whether a record's package is the used one
// (lang is the language of the lock file: the records of another language are not compared. MatchMajor for Go; SamePackage, the name as it is, for Dart; SameCrate for Rust).
func (r *Rules) UnverifiedVersionsOf(used map[string][]string, lang, source string, match func(pattern, name string) bool) []UnverifiedVersion {
	var out []UnverifiedVersion
	// only the records of the language of the lock file (and the ones that do not say a language)
	var recs []Verified
	for _, rec := range r.Verified {
		if rec.Lang == "" || rec.Lang == lang {
			recs = append(recs, rec)
		}
	}
	for pkg, vers := range used {
		var ranges []string
		var outside []string
		for _, rec := range recs {
			if match(rec.Package, pkg) {
				ranges = append(ranges, rec.Versions)
			}
		}
		if len(ranges) == 0 {
			continue
		}
		for _, ver := range vers {
			covered := false
			for _, rec := range recs {
				if match(rec.Package, pkg) && rec.Holds(ver) {
					covered = true
					break
				}
			}
			if !covered {
				outside = append(outside, ver)
			}
		}
		if len(outside) == 0 {
			continue
		}
		sort.Strings(ranges)
		u := UnverifiedVersion{Package: pkg, Used: outside[0], Verified: ranges, Source: source}
		if len(vers) > 1 {
			u.AllUsed = vers
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Package < out[j].Package })
	return out
}

// SameCrate says whether a record's package is the crate: Cargo reads - and _ in a crate name as the same.
func SameCrate(pattern, name string) bool {
	return strings.ReplaceAll(pattern, "_", "-") == strings.ReplaceAll(name, "_", "-")
}

// SamePackage says whether a record's package is the package (the name as it is: Dart).
func SamePackage(pattern, name string) bool { return pattern == name }
