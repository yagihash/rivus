package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// copyTree copies a directory (the files only; the fixtures are small).
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The Rust extractor told --body-replaced does not read the crate (it lowers none of its bodies), says how many functions the
// crate has (body_not_read), and the analysis is the one that the filter makes (-drop-after-read), whichever of the two
// spellings of the crate name the declaration uses (`dep-crate` of Cargo and a model, `dep_crate` of rustc). The fixture is the
// sample crate of the extractor and its dependency, with the dependency renamed so that its name has a `-`. It needs the
// extractor (RIVUS_RUST): the sandbox of macOS runs it where there is one; elsewhere (the CI job) the fixture, which is in this
// repository and has no build script and no macro, is run on the machine as the other tests of the extractor do.
// What differs is the summary of the declared packages (functions_declared against functions), and the count of the functions
// that the extractor could not read (fn_skipped: a crate that is not read has none).
func TestBodyReplacedOfARustCrateIsTheSameAsTheFilter(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	src, err := filepath.Abs("../../extractor/rust/test")
	if err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(t.TempDir(), "sample")
	copyTree(t, filepath.Join(src, "sample"), proj)
	copyTree(t, filepath.Join(src, "dep"), filepath.Join(proj, "dep"))
	// the dependency is `dep-crate` (rustc: dep_crate), and inside the workspace root of the copy
	rewrite := func(path string, pairs ...string) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.NewReplacer(pairs...).Replace(string(b))), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rewrite(filepath.Join(proj, "Cargo.toml"), `path = "../dep"`, `path = "dep"`, "depcrate", "dep-crate", "[workspace]", "[workspace]\nexclude = [\"dep\"]")
	rewrite(filepath.Join(proj, "Cargo.lock"), "depcrate", "dep-crate", `"../dep"`, `"dep"`)
	rewrite(filepath.Join(proj, "dep", "Cargo.toml"), "depcrate", "dep-crate")
	srcFiles, _ := filepath.Glob(filepath.Join(proj, "src", "*.rs"))
	for _, f := range srcFiles {
		rewrite(f, "depcrate", "dep_crate")
	}
	rules := filepath.Join(proj, "rules.yaml")

	run := func(args ...string) map[string]any {
		all := []string{"analyze", "-lang", "rust", "-C", proj, "-rules", rules, "-no-project-rules", "-detail", "summary", "-yes", "-rust-extractor", bin}
		if runtime.GOOS != "darwin" {
			all = append(all, "-unsafe-host-exec")
		}
		cmd := exec.Command(binary, append(all, args...)...)
		cmd.Env = append(os.Environ(), "RIVUS_RUST_EXTRACTOR="+bin)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze %v: %v\n%s", args, err, stderr.String())
		}
		var r map[string]any
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	replaced := func(r map[string]any) []any {
		v, _ := r["summary"].(map[string]any)["body_replaced_packages"].([]any)
		return v
	}
	// the report without what differs by the way the package was left out, and without the times
	same := func(r map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range r {
			out[k] = v
		}
		sum := map[string]any{}
		for k, v := range r["summary"].(map[string]any) {
			// the memory figures and the GC share are of the machine and its load, like the times (#359)
			if k != "body_replaced_packages" && k != "body_replaced_ignored" && !slices.Contains(machineSummaryKeys, k) {
				sum[k] = v
			}
		}
		out["summary"] = sum
		// the list of the rule sets names the flag that declared the package
		var rules []any
		for _, x := range r["rules"].([]any) {
			if x != "-body-replaced" {
				rules = append(rules, x)
			}
		}
		out["rules"] = rules
		out["stats"] = map[string]any{"functions": r["stats"].(map[string]any)["functions"]}
		return out
	}

	read := run()
	if replaced(read) != nil {
		t.Fatalf("without a declaration: %v", replaced(read))
	}
	filter := run("-body-replaced", "dep-crate", "-drop-after-read")
	fr := replaced(filter)
	if len(fr) != 1 || fr[0].(map[string]any)["functions"] == nil || fr[0].(map[string]any)["functions_declared"] != nil {
		t.Fatalf("summary of the filter: %v", fr)
	}
	if fn(filter) >= fn(read) {
		t.Fatalf("the filter leaves out nothing: %v functions, %v read", fn(filter), fn(read))
	}
	for _, spelling := range []string{"dep-crate", "dep_crate"} {
		got := run("-body-replaced", spelling)
		gr := replaced(got)
		if len(gr) != 1 || gr[0].(map[string]any)["functions_declared"] == nil || gr[0].(map[string]any)["functions"] != nil {
			t.Fatalf("%s: summary of the extractor: %v (the crate was read, or not found: a declaration that misses says nothing)", spelling, gr)
		}
		if !reflect.DeepEqual(same(got), same(filter)) {
			a, _ := json.Marshal(same(got))
			b, _ := json.Marshal(same(filter))
			t.Errorf("%s: the extractor that does not read the crate and the filter differ:\n%s\n%s", spelling, a, b)
		}
	}
	// -read-declared-bodies is the run without the declaration
	if again := run("-body-replaced", "dep_crate", "-read-declared-bodies"); !reflect.DeepEqual(same(again), same(read)) {
		t.Errorf("-read-declared-bodies is not the run without the declaration")
	}
}

func fn(r map[string]any) float64 { return r["stats"].(map[string]any)["functions"].(float64) }
