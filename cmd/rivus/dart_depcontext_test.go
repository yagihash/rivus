package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// dartTreeIR runs the Dart extractor on a directory under extractor/dart/test and returns the IDs of the functions, and
// the load errors. Every package of the tree needs its .dart_tool/package_config.json (a missing one is made offline).
func dartTreeIR(t *testing.T, dart, rel string, pkgs ...string) (funcs []string, loadErrors []ir.LoadError) {
	t.Helper()
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	for _, p := range pkgs {
		dir := filepath.Join(extractor, "test", p)
		if _, err := os.Stat(filepath.Join(dir, ".dart_tool", "package_config.json")); err == nil {
			continue
		}
		get := exec.Command(words[0], append(append([]string{}, words[1:]...), "pub", "get", "--offline")...)
		get.Dir = dir
		if out, err := get.CombinedOutput(); err != nil {
			t.Fatalf("run `dart pub get` in %s first (`--offline` failed: %v)\n%s", dir, err, out)
		}
	}
	cmd := exec.Command(words[0], append(append([]string{}, words[1:]...), "run", filepath.Join(extractor, "bin", "rivus_dart.dart"), filepath.Join(extractor, "test", rel))...)
	cmd.Dir = extractor
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("extractor on %s: %v\n%s", rel, err, stderr.String())
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	for sc.Scan() {
		var l struct {
			Kind      string              `json:"kind"`
			Func      struct{ ID string } `json:"func"`
			LoadError ir.LoadError        `json:"load_error"`
		}
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		switch l.Kind {
		case "func":
			funcs = append(funcs, l.Func.ID)
		case "load_error":
			loadErrors = append(loadErrors, l.LoadError)
		}
	}
	return funcs, loadErrors
}

func depPackages(ids []string) []string {
	seen := map[string]bool{}
	for _, id := range ids {
		if rest, ok := strings.CutPrefix(id, "package:"); ok {
			seen[strings.SplitN(rest, "/", 2)[0]] = true
		}
	}
	var out []string
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// `-C` may point at the parent of several Dart packages (no pubspec.yaml above them). A dependency that one of them
// imports is resolved by the package_config.json of that package, not by the one of the first package of the tree:
// otherwise its functions are named by file: URIs, the calls to them (package: URIs) do not reach them, and they are
// not analysed, with nothing said.
func TestDartDependenciesOfAParentOfPackagesAreNamedByPackageURIs(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	parent, parentErrs := dartTreeIR(t, dart, "monorepo", "monorepo/app", "monorepo/shared")
	app, _ := dartTreeIR(t, dart, "monorepo/app", "monorepo/app", "monorepo/shared")
	for _, id := range parent {
		if strings.HasPrefix(id, "file:") {
			t.Fatalf("a function of a dependency is named by a file: URI, not package:, so no call reaches it: %s", id)
		}
	}
	for _, e := range parentErrs {
		if strings.HasPrefix(e.Message, ir.DependencyNotLinkedPrefix) {
			t.Errorf("a dependency was not linked: %+v", e)
		}
	}
	got := strings.Join(depPackages(parent), ",")
	for _, want := range depPackages(app) {
		if !strings.Contains(","+got+",", ","+want+",") {
			t.Errorf("the packages with package: IDs are %s; -C of the app alone has %s too", got, want)
		}
	}
	for _, want := range []string{"package:fixnum/", "package:collection/", "package:shared/"} {
		n := 0
		for _, id := range parent {
			if strings.HasPrefix(id, want) {
				n++
			}
		}
		if n == 0 {
			t.Errorf("no function of %s in the IR of the parent", want)
		}
	}

	// rivus reads them: the functions of the dependencies are in the analysis
	extractor, _ := filepath.Abs("../../extractor/dart")
	run := func(dir string) int {
		cmd := exec.Command(binary, "analyze", "-lang", "dart", "-C", filepath.Join(extractor, "test", dir))
		cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze %s: %v", dir, err)
		}
		var r struct {
			Stats struct {
				Functions int `json:"functions"`
			} `json:"stats"`
		}
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		return r.Stats.Functions
	}
	if p, a := run("monorepo"), run("monorepo/app"); p == 0 || p < a {
		t.Errorf("stats.functions: %d for the parent, %d for the app alone: the dependencies are not read", p, a)
	}
}

// Two packages of the tree that read different versions of one dependency give the functions the same IDs: only one
// version can be read. The other is said (summary.unread, and a line on the standard error), not dropped silently.
func TestDartTwoVersionsOfADependencyInOneTreeAreSaid(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	funcs, errs := dartTreeIR(t, dart, "versions", "versions/a", "versions/b")
	for _, id := range funcs {
		if strings.HasPrefix(id, "file:") {
			t.Fatalf("a function of a dependency is named by a file: URI: %s", id)
		}
	}
	var said []ir.LoadError
	for _, e := range errs {
		if strings.HasPrefix(e.Message, ir.DependencyNotLinkedPrefix) && e.Unread && strings.Contains(e.Message, "collection") {
			said = append(said, e)
		}
	}
	if len(said) != 1 {
		t.Fatalf("the second version of collection is not said once as unread: %+v", errs)
	}

	extractor, _ := filepath.Abs("../../extractor/dart")
	cmd := exec.Command(binary, "analyze", "-lang", "dart", "-C", filepath.Join(extractor, "test", "versions"))
	cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, stderr.String())
	}
	var r struct {
		Summary struct {
			Unread *struct {
				Top []struct{ Package, Why string } `json:"top"`
			} `json:"unread"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	if r.Summary.Unread == nil || len(r.Summary.Unread.Top) == 0 || !strings.HasPrefix(r.Summary.Unread.Top[0].Why, ir.DependencyNotLinkedPrefix) {
		t.Errorf("summary.unread does not say it: %+v", r.Summary.Unread)
	}
	if !strings.Contains(stderr.String(), "dart pub get") {
		t.Errorf("the standard error does not say it:\n%s", stderr.String())
	}
}
