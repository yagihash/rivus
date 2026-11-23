package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// What the build does not ship is not read unless -tests asks (ADR-062): the sample has a test/ with a flow in
// its main, and a dependency declared for development only. By default the flow is not a finding and the output
// says how much was left out; with -tests it is read.
func TestDartTestsAreNotReadUnlessAsked(t *testing.T) {
	t.Parallel()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	ensureDartSamplePub(t, dart)
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	sample := filepath.Join(extractor, "test", "sample")
	env := []string{"RIVUS_DART_EXTRACTOR=" + dart + " run " + filepath.Join(extractor, "bin", "rivus_dart.dart")}
	type site struct {
		Position ir.Position `json:"position"`
	}
	type result struct {
		Findings []struct {
			Source site `json:"source"`
			Sink   site `json:"sink"`
		} `json:"findings"`
		Stats struct {
			TestsSkipped *ir.TestsSkipped `json:"tests_skipped"`
		} `json:"stats"`
	}
	run := func(extra ...string) (result, string) {
		args := append([]string{"analyze", "-lang", "dart", "-C", sample, "-rules", filepath.Join(sample, "rules.yaml")}, extra...)
		cmd := exec.Command(binary, args...)
		cmd.Env = append(os.Environ(), env...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil && len(out) == 0 {
			t.Fatalf("rivus %v: %v\n%s", args, err, stderr.String())
		}
		var r result
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return r, stderr.String()
	}
	inTest := func(r result) (n int) {
		for _, f := range r.Findings {
			if strings.HasPrefix(f.Sink.Position.File, "test/") || strings.HasPrefix(f.Source.Position.File, "test/") {
				n++
			}
		}
		return n
	}

	def, stderr := run()
	if n := inTest(def); n != 0 {
		t.Errorf("by default, %d findings are in test/", n)
	}
	ts := def.Stats.TestsSkipped
	if ts == nil || ts.Files != 1 || ts.DevDependencies != 1 || len(ts.DevDependencyNames) != 1 || ts.DevDependencyNames[0] != "matcher" {
		t.Errorf("stats.tests_skipped = %+v, want 1 file and the dev dependency matcher", ts)
	}
	if !strings.Contains(stderr, "were not read (--tests reads them") {
		t.Errorf("stderr does not say what was left out:\n%s", stderr)
	}

	with, stderr := run("-tests")
	if n := inTest(with); n == 0 {
		t.Errorf("with -tests, the flow in test/ is not found")
	}
	if with.Stats.TestsSkipped != nil || strings.Contains(stderr, "were not read") {
		t.Errorf("with -tests nothing is left out, but: %+v\n%s", with.Stats.TestsSkipped, stderr)
	}
	// what is shipped is found either way
	if len(with.Findings) != len(def.Findings)+inTest(with) {
		t.Errorf("-tests adds the findings of the tests only: %d with (%d in test/), %d without", len(with.Findings), inTest(with), len(def.Findings))
	}
}

// "rivus sites" reads the languages that the directory holds, as analyze does. It read only Go: a Dart directory gave
// no site, with no error and no word about the language that was not read (a list of sites that was taken for "the
// rules have nothing to say").
func TestSitesReadsTheLanguagesOfTheDirectory(t *testing.T) {
	t.Parallel()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	ensureDartSamplePub(t, dart)
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "sites", "-C", filepath.Join(extractor, "test", "sample"))
	cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("sites: %v\n%s", err, stderr.String())
	}
	var r struct {
		Sites []struct {
			Role string `json:"role"`
		} `json:"sites"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Sites) == 0 {
		t.Error("no site of the Dart directory: sites reads only Go")
	}
	// the language that is not read is said: a Dart directory whose extractor is not there is an error, not an empty list
	bad := exec.Command(binary, "sites", "-C", filepath.Join(extractor, "test", "sample"), "-lang", "dart", "-dart-extractor", "rivus-dart-does-not-exist-xyz")
	if err := bad.Run(); err == nil {
		t.Error("sites -lang dart without the extractor did not fail")
	}
}
