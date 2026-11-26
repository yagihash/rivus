package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// @elsewhere(go: <where>) (docs/DESIGN-zoo.md, "@na, @elsewhere and @todo") is the mark of a Dart cell whose form exists in Go and is
// measured in another place than a cell of the same name: a cell of a package of the Go zoo (bench/zoo-go-pkg/<package>), written by
// its name (F/pkg/<package>/<cell>), or a case of bench/gcp, written by bench/gcp/<package>/<case> (the method of the Server
// type of bench/gcp/<package>/<package>.go). The place is checked here: a name that no cell has fails the test. A place whose
// vulnerable case has a @miss (a case of bench/gcp that gcpKnownMisses holds) is counted as "elsewhere-miss": Go does not
// find it there either.

var zooElsewhereWhereRe = regexp.MustCompile(`^go:\s*(F/pkg/[a-z0-9-]+/[a-z0-9-]+|bench/gcp/[a-z]+/[A-Za-z0-9_]+)$`)

// zooElsewhereRef is the place that the @elsewhere of a cell names; false if the cell has none or the text is not in the form.
func zooElsewhereRef(c *zooCell) (string, bool) {
	if len(c.elsewhere) != 1 {
		return "", false
	}
	m := zooElsewhereWhereRe.FindStringSubmatch(strings.TrimSpace(c.elsewhere[0]))
	if m == nil {
		return "", false
	}
	return m[1], true
}

// zooElsewhereErrors checks the @elsewhere of a Dart cell: one mark, in the form, at a place that exists (targets: the places
// -> whether the place has a @miss), not with @na, and not for a cell that zoo-go has under the same name.
func zooElsewhereErrors(c *zooCell, targets map[string]bool, goHasCell bool) []string {
	if len(c.elsewhere) == 0 {
		return nil
	}
	var errs []string
	if len(c.elsewhere) > 1 {
		errs = append(errs, fmt.Sprintf("%s: %d @elsewhere marks (one for a cell)", c.name, len(c.elsewhere)))
	}
	if len(c.na) > 0 {
		errs = append(errs, fmt.Sprintf("%s: has both @na and @elsewhere (@na: the form is not in Go; @elsewhere: it is measured in another place)", c.name))
	}
	if goHasCell {
		errs = append(errs, fmt.Sprintf("%s: has @elsewhere(%s), and zoo-go has a cell of this name", c.name, c.elsewhere[0]))
	}
	if len(c.elsewhere) == 1 {
		ref, ok := zooElsewhereRef(c)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: @elsewhere(%s) is not go: F/pkg/<package>/<cell> or go: bench/gcp/<package>/<case>", c.name, c.elsewhere[0]))
		} else if _, there := targets[ref]; !there {
			errs = append(errs, fmt.Sprintf("%s: @elsewhere(%s): no such cell or case", c.name, c.elsewhere[0]))
		}
	}
	return errs
}

// zooElsewhereTargets lists the places that an @elsewhere can name, with whether the place has a @miss: the cells of the packages
// of the Go zoo (the @miss of a vulnerable case), the cases of bench/gcp (gcpKnownMisses).
func zooElsewhereTargets(t testing.TB) map[string]bool {
	t.Helper()
	targets := map[string]bool{}
	for _, dir := range zooPkgDirs(t, "go") {
		lines, cells := zooScan(t, dir)
		for _, c := range cells {
			miss, _ := zooVulnMisses(lines, c)
			targets[c.name] = miss > 0
		}
	}
	for _, pkg := range []string{"spanner", "bigquery"} {
		src, err := os.ReadFile(filepath.Join(zooBenchDir(t), "gcp", pkg, pkg+".go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range gcpServerMethods(string(src)) {
			targets["bench/gcp/"+pkg+"/"+name] = gcpKnownMisses[name] != ""
		}
	}
	return targets
}

// gcpServerMethods are the names of the methods of the Server type in a source of bench/gcp: the cases.
func gcpServerMethods(src string) []string {
	var names []string
	const p = "func (s *Server) "
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, p) {
			if i := strings.Index(line, "(w "); i > len(p) {
				names = append(names, line[len(p):i])
			}
		}
	}
	return names
}

// TestZooElsewhereMarks holds the checks of @elsewhere (zooElsewhereErrors) without the Dart SDK: TestZooDart skips without it.
func TestZooElsewhereMarks(t *testing.T) {
	targets := map[string]bool{"F/pkg/gin/query": false, "F/pkg/x/missed": true, "bench/gcp/bigquery/VRun": false}
	for _, tc := range []struct {
		name   string
		c      *zooCell
		goHas  bool
		want   []string // the substrings of the errors, in order
		isMiss bool
	}{
		{"no mark", &zooCell{name: "F/a/x"}, false, nil, false},
		{"a cell of a package", &zooCell{name: "F/a/x", elsewhere: []string{"go: F/pkg/gin/query"}}, false, nil, false},
		{"a case of bench/gcp", &zooCell{name: "F/a/x", elsewhere: []string{"go: bench/gcp/bigquery/VRun"}}, false, nil, false},
		{"the place has a @miss", &zooCell{name: "F/a/x", elsewhere: []string{"go: F/pkg/x/missed"}}, false, nil, true},
		{"no such cell", &zooCell{name: "F/a/x", elsewhere: []string{"go: F/pkg/gin/none"}}, false, []string{"no such cell or case"}, false},
		{"no such case", &zooCell{name: "F/a/x", elsewhere: []string{"go: bench/gcp/bigquery/VNone"}}, false, []string{"no such cell or case"}, false},
		{"free text", &zooCell{name: "F/a/x", elsewhere: []string{"go: measured in zoo-go-pkg (gin)"}}, false, []string{"is not go:"}, false},
		{"not go:", &zooCell{name: "F/a/x", elsewhere: []string{"rust: F/pkg/gin/query"}}, false, []string{"is not go:"}, false},
		{"both @na", &zooCell{name: "F/a/x", na: []string{"go: x"}, elsewhere: []string{"go: F/pkg/gin/query"}}, false, []string{"both @na and @elsewhere"}, false},
		{"Go has the cell", &zooCell{name: "F/a/x", elsewhere: []string{"go: F/pkg/gin/query"}}, true, []string{"zoo-go has a cell"}, false},
		{"two marks", &zooCell{name: "F/a/x", elsewhere: []string{"go: F/pkg/gin/query", "go: F/pkg/gin/query"}}, false, []string{"2 @elsewhere marks"}, false},
	} {
		errs := zooElsewhereErrors(tc.c, targets, tc.goHas)
		if len(errs) != len(tc.want) {
			t.Errorf("%s: errors %q, want %q", tc.name, errs, tc.want)
			continue
		}
		for i, w := range tc.want {
			if !strings.Contains(errs[i], w) {
				t.Errorf("%s: error %q has no %q", tc.name, errs[i], w)
			}
		}
		if ref, ok := zooElsewhereRef(tc.c); ok && len(tc.want) == 0 && targets[ref] != tc.isMiss {
			t.Errorf("%s: the place %s has a @miss: %v, want %v", tc.name, ref, targets[ref], tc.isMiss)
		}
	}
}

// TestZooElsewhereParse reads @elsewhere the way @na is read: the reason ends at the first ")" (so a ")" is not in it), and the
// mark of a line is not a @na.
func TestZooElsewhereParse(t *testing.T) {
	line := "// @elsewhere(go: F/pkg/gin/query)"
	m := zooElsewhereRe.FindStringSubmatch(line)
	if m == nil || m[1] != "go: F/pkg/gin/query" {
		t.Fatalf("@elsewhere is read as %q", m)
	}
	if zooNaRe.MatchString(line) {
		t.Error("@elsewhere is read as @na")
	}
	if zooElsewhereRe.MatchString("// @na(go: no such form)") {
		t.Error("@na is read as @elsewhere")
	}
	if got := gcpServerMethods("func (s *Server) VRun(w http.ResponseWriter, r *http.Request) {\nfunc other() {}\n"); len(got) != 1 || got[0] != "VRun" {
		t.Errorf("gcpServerMethods = %q", got)
	}
}
