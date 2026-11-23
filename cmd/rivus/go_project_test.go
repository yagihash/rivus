package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The project is the packages of the same module (ADR-114), not only the ones the pattern names: the pattern says what is
// asked for, and the sources and sinks of every package of the module that the analysis reaches are listed. A run that is
// narrowed to one command has the same findings, for the flows that start there, as one of the whole module; the
// functions of the module are not counted as dependencies.
func TestPackagesOfTheSameModuleAreTheProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, text string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/pj\n\ngo 1.22\n")
	write("cmd/app/main.go", "package main\n\nimport (\n\t\"net/http\"\n\n\t\"example.com/pj/internal/run\"\n\t\"example.com/pj/internal/store\"\n)\n\nfunc h(w http.ResponseWriter, r *http.Request) {\n\tv := r.FormValue(\"x\")\n\trun.Run(v)\n\tstore.Read(v)\n}\n\nfunc main() { http.HandleFunc(\"/\", h); http.ListenAndServe(\":0\", nil) }\n")
	write("internal/run/run.go", "package run\n\nimport \"os/exec\"\n\nfunc Run(v string) { exec.Command(\"sh\", \"-c\", v).Run() }\n")
	write("internal/store/s.go", "package store\n\nimport \"os\"\n\nfunc Read(v string) { os.ReadFile(v) }\n")
	write("other/o.go", "package other\n\nimport (\n\t\"net/http\"\n\t\"os/exec\"\n)\n\nfunc H(w http.ResponseWriter, r *http.Request) { exec.Command(r.FormValue(\"y\")).Run() }\n")

	run := func(wd string, args ...string) (flows []string, depSites bool) {
		t.Helper()
		cmd := exec.Command(binary, append([]string{"analyze"}, args...)...)
		cmd.Dir = filepath.Join(dir, wd)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze %v: %v", args, err)
		}
		var r struct {
			Findings []struct {
				Source struct{ Func string }
				Sink   struct {
					Symbol string
					Func   string
				}
			} `json:"findings"`
			Summary struct {
				DependencySites *json.RawMessage `json:"dependency_sites"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		for _, f := range r.Findings {
			flows = append(flows, f.Source.Func+" -> "+f.Sink.Symbol+" in "+f.Sink.Func)
		}
		sort.Strings(flows)
		return flows, r.Summary.DependencySites != nil
	}
	whole, wholeDep := run(".", "./...")
	narrowed, narrowedDep := run(".", "./cmd/...")
	inDir, inDirDep := run("cmd/app", "./...")
	if len(whole) != 3 {
		t.Fatalf("the whole module: %v", whole)
	}
	// what starts at cmd/app: the two flows through the packages of the module; the one in other is not asked for
	var started []string
	for _, f := range whole {
		if strings.HasPrefix(f, "example.com/pj/cmd/app.") {
			started = append(started, f)
		}
	}
	if len(started) != 2 {
		t.Fatalf("flows that start at cmd/app: %v", started)
	}
	for name, got := range map[string][]string{"./cmd/...": narrowed, "-C cmd/app": inDir} {
		if strings.Join(got, "\n") != strings.Join(started, "\n") {
			t.Errorf("%s: %v, want %v", name, got, started)
		}
	}
	if wholeDep || narrowedDep || inDirDep {
		t.Errorf("the functions of the module are counted as dependencies (whole %v, narrowed %v, in the directory %v)", wholeDep, narrowedDep, inDirDep)
	}
}

// A module that go.work lists is the project; one that a replace directive points to (also by a directory: the zoo's
// skeletons stand for libraries so) is a dependency (ADR-114).
func TestWorkspaceModulesAreTheProjectAndReplacedOnesAreNot(t *testing.T) {
	t.Parallel()
	type result struct {
		sinks    []string
		depFuncs int
	}
	run := func(wd string, files map[string]string) result {
		t.Helper()
		base := t.TempDir()
		for name, text := range files {
			path := filepath.Join(base, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(binary, "analyze", "./...")
		cmd.Dir = filepath.Join(base, wd)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		var r struct {
			Findings []struct {
				Sink struct{ Func string }
			} `json:"findings"`
			Summary struct {
				DependencySites *struct {
					Functions int `json:"dependency_functions"`
				} `json:"dependency_sites"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		var res result
		for _, f := range r.Findings {
			res.sinks = append(res.sinks, f.Sink.Func)
		}
		if d := r.Summary.DependencySites; d != nil {
			res.depFuncs = d.Functions
		}
		return res
	}
	app := func(imp, call string) string {
		return "package main\n\nimport (\n\t\"net/http\"\n\n\t\"" + imp + "\"\n)\n\nfunc h(w http.ResponseWriter, r *http.Request) { " + call + "(r.FormValue(\"x\")) }\n\nfunc main() { http.HandleFunc(\"/\", h); http.ListenAndServe(\":0\", nil) }\n"
	}
	exe := "package %s\n\nimport \"os/exec\"\n\nfunc Run(v string) { exec.Command(\"sh\", \"-c\", v).Run() }\n"
	work := run("app", map[string]string{
		"go.work":       "go 1.22\n\nuse (\n\t./app\n\t./shared\n)\n",
		"app/go.mod":    "module example.com/app\n\ngo 1.22\n\nrequire example.com/shared v0.0.0\n",
		"app/main.go":   app("example.com/shared", "shared.Run"),
		"shared/go.mod": "module example.com/shared\n\ngo 1.22\n",
		"shared/a.go":   strings.Replace(exe, "%s", "shared", 1),
	})
	if len(work.sinks) != 1 || work.sinks[0] != "example.com/shared.Run" || work.depFuncs != 0 {
		t.Errorf("a module that go.work lists: %+v", work)
	}
	replaced := run("app", map[string]string{
		"app/go.mod":  "module example.com/app\n\ngo 1.22\n\nrequire example.com/lib v0.0.0\n\nreplace example.com/lib => ../lib\n",
		"app/main.go": app("example.com/lib", "lib.Run"),
		"lib/go.mod":  "module example.com/lib\n\ngo 1.22\n",
		"lib/a.go":    strings.Replace(exe, "%s", "lib", 1),
	})
	if len(replaced.sinks) != 0 || replaced.depFuncs == 0 {
		t.Errorf("a module that a replace points to: %+v (a dependency: its sites are not searched, and the output says so)", replaced)
	}
}
