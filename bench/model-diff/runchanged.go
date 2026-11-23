package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/yagihash/rivus/internal/model"
)

// runChanged is what a pull request that changes a model runs (`make model-diff-changed BASE=<sha>`): the differential test of each
// package whose rows differ from the ones at the commit `base`, one after the other, and one table at the end. Only the languages in
// `langs` are run; a changed package of another language is listed as not run (never as passed). The exit status is 1 when one package
// failed (a MISS, an EXTRA that no note allows, an error, a row whose symbol is not in the package), 2 when the tool could not run.
func runChanged(repo, base, langs, out string, extra []string) int {
	pkgs, err := changedPackages(repo, base)
	if err != nil {
		return fail(err)
	}
	reviewed, _ := addedReviewed(repo, base)
	for _, l := range reviewed {
		fmt.Println(l) // a notice for the pull request, never a failure
	}
	if out == "" {
		d, err := os.MkdirTemp("", "model-diff-changed-")
		if err != nil {
			return fail(err)
		}
		out = d
	}
	want := map[string]bool{}
	for _, l := range strings.Fields(strings.ReplaceAll(langs, ",", " ")) {
		want[l] = true
	}
	builtins, err := BuiltinFiles(repo)
	if err != nil {
		return fail(err)
	}
	type line struct{ pkg, status, detail string }
	var table []line
	failed := false
	for i, entry := range pkgs {
		lang, pkg, _ := strings.Cut(entry, " ")
		if !want[lang] {
			table = append(table, line{entry, "not run", "the language is not in -langs (" + langs + "): not checked"})
			continue
		}
		args := []string{"-lang", lang, "-pkg", pkg, "-repo", repo, "-out", filepath.Join(out, fmt.Sprint(i)), "-json", filepath.Join(out, fmt.Sprintf("report-%d.json", i))}
		concrete := pkg
		if model.HasMajorWildcard(pkg) {
			v := verifiedVersion(builtins, pkg)
			major, ok := majorOfVersion(v)
			if !ok {
				table = append(table, line{entry, "FAIL", "the package has /v* and no verified_versions entry with checked_with gives the major to check"})
				failed = true
				continue
			}
			args = append(args, "-version", v)
			concrete = model.ConcreteMajor(pkg, major)
		}
		if !hasRows(builtins, lang, concrete) {
			table = append(table, line{entry, "nothing", "no row of the package is left (the rows were removed)"})
			continue
		}
		fmt.Printf("== model-diff %s\n", entry)
		switch code := realMain(append(args, extra...)); code {
		case 0:
			table = append(table, line{entry, "ok", ""})
		case 1:
			table = append(table, line{entry, "FAIL", "see the table above"})
			failed = true
		default:
			table = append(table, line{entry, "ERROR", "the tool could not run (exit " + fmt.Sprint(code) + ")"})
			failed = true
		}
	}
	fmt.Printf("== model-diff of the changed packages since %s: %d package(s)\n", base, len(pkgs))
	for _, l := range table {
		fmt.Printf("  %-8s %-50s %s\n", l.status, l.pkg, l.detail)
	}
	if failed {
		return 1
	}
	return 0
}

// hasRows: the built-in files have a row of the package that the test looks at.
func hasRows(builtins []string, lang, pkg string) bool {
	for _, f := range builtins {
		rows, _, err := LoadRows(f, lang, pkg)
		if err == nil && len(rows) > 0 {
			return true
		}
	}
	return false
}

// verifiedVersion is the version that a built-in file has checked the rows of the package with (`checked_with` of its verified_versions
// entry for that package or for the module that has it, written with /v* like the symbols): what a package of any major needs to say which major is read.
func verifiedVersion(builtins []string, pkg string) string {
	for _, f := range builtins {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var rs model.RuleSet
		if yaml.Unmarshal(data, &rs) != nil {
			continue
		}
		for _, v := range rs.VerifiedVersions {
			if (v.Package == pkg || strings.HasPrefix(pkg, v.Package+"/")) && v.CheckedWith != "" {
				return v.CheckedWith
			}
		}
	}
	return ""
}
