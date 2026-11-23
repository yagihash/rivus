package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The files that the build constraints leave out of the configuration (`//go:build ignore`, a `_windows.go` on another
// system) are counted in summary.outside_configuration (not in the tests); a project with none has no such field.
func TestFilesOutsideTheBuildConfigurationAreSaid(t *testing.T) {
	t.Parallel()
	run := func(files map[string]string, extra ...string) (oc *struct {
		Files    int      `json:"files"`
		Examples []string `json:"examples"`
	}, inTests bool) {
		dir := t.TempDir()
		files["go.mod"] = "module example.com/gt\n\ngo 1.23\n"
		files["main.go"] = "package main\n\nfunc main() {}\n"
		for n, c := range files {
			if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(binary, append(append([]string{"analyze"}, extra...), "./...")...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		var r struct {
			Summary struct {
				OutsideConfiguration *struct {
					Files    int      `json:"files"`
					Examples []string `json:"examples"`
				} `json:"outside_configuration"`
			} `json:"summary"`
			Stats struct {
				TestsSkipped *json.RawMessage `json:"tests_skipped"`
			} `json:"stats"`
		}
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		return r.Summary.OutsideConfiguration, r.Stats.TestsSkipped != nil && strings.Contains(string(*r.Stats.TestsSkipped), "outside_build")
	}
	got, inTests := run(map[string]string{
		"only_windows.go": "package main\n\nfunc win() {}\n",
		"ignored.go":      "//go:build ignore\n\npackage main\n\nfunc ign() {}\n",
	})
	if got == nil || got.Files != 2 || len(got.Examples) != 2 || inTests {
		t.Errorf("outside_configuration: %+v (in stats.tests_skipped: %v)", got, inTests)
	}
	// (also with -tests: the count did not depend on it)
	if withTests, _ := run(map[string]string{"only_windows.go": "package main\n\nfunc win() {}\n"}, "-tests"); withTests == nil || withTests.Files != 1 || len(withTests.Examples) != 1 || withTests.Examples[0] != "only_windows.go" {
		t.Errorf("outside_configuration with -tests: %+v", withTests)
	}
	if none, _ := run(map[string]string{}); none != nil {
		t.Errorf("a project with nothing left out has outside_configuration: %+v", none)
	}
}
