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
	dir := writeProjectModule(t)
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

// A module that go.work lists is the project, and so is one that a replace directive points to by a directory; -dep-dir
// reads the packages under a directory as dependencies, and says so in summary.dep_dirs (ADR-114).
func TestWorkspaceAndReplacedModulesAreTheProject(t *testing.T) {
	t.Parallel()
	type result struct {
		sinks    []string
		depFuncs int
		depDirs  []string
	}
	run := func(wd string, files map[string]string, extra ...string) (res result) {
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
		cmd := exec.Command(binary, append(append([]string{"analyze"}, extra...), "./...")...)
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
				DepDirs         []string `json:"dep_dirs"`
				DependencySites *struct {
					Functions int `json:"dependency_functions"`
				} `json:"dependency_sites"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		res.depDirs = r.Summary.DepDirs
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
	linked := map[string]string{
		"app/go.mod":  "module example.com/app\n\ngo 1.22\n\nrequire example.com/lib v0.0.0\n\nreplace example.com/lib => ../lib\n",
		"app/main.go": app("example.com/lib", "lib.Run"),
		"lib/go.mod":  "module example.com/lib\n\ngo 1.22\n",
		"lib/a.go":    strings.Replace(exe, "%s", "lib", 1),
	}
	replaced := run("app", linked)
	if len(replaced.sinks) != 1 || replaced.sinks[0] != "example.com/lib.Run" || replaced.depFuncs != 0 || replaced.depDirs != nil {
		t.Errorf("a module that a replace points to by a directory: %+v", replaced)
	}
	// a copy of a library that is put in place by a replace: the directory is named, and the output says so
	copied := run("app", linked, "-dep-dir", "../lib")
	if len(copied.sinks) != 0 || copied.depFuncs == 0 || len(copied.depDirs) != 1 || copied.depDirs[0] != "../lib" {
		t.Errorf("-dep-dir: %+v (its sites are not searched, and the output says so)", copied)
	}
}

// writeProjectModule writes a module of a command that calls two packages of the module, and a package that the command
// does not reach.
func writeProjectModule(t *testing.T) string {
	t.Helper()
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
	return dir
}
