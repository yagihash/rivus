package main

import (
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/ir"
)

// The Dart extractor writes the neutral IR of a sample package; rivus reads it
// and finds the flows that the annotations of the sample say it must find, and
// no other. It needs the Dart SDK: RIVUS_DART names the command that runs it,
// for example "dart" or "mise exec dart@3.13.6 -- dart".
func TestDartExtractor(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		t.Skip("RIVUS_DART is not set")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	args := append(words[1:], "run", "bin/rivus_dart.dart", "test/sample")
	cmd := exec.Command(words[0], args...)
	cmd.Dir = extractor
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v\n%s", err, stderr.String())
	}
	checkSampleFlows(t, stream, filepath.Join(extractor, "test", "sample", "rules.yaml"),
		filepath.Join(extractor, "test", "sample"),
		[]string{"lib/app.dart", "lib/web.dart", "lib/server.dart", "lib/patterns.dart", "lib/callbacks.dart", "lib/syntax.dart", "lib/grpc_flows.dart", "lib/review_try.dart", "lib/review_ctor.dart", "lib/review_misc.dart", "lib/gen_model.dart", "lib/gen_model.g.dart", "lib/review_issues.dart", "lib/cfg_flows.dart", "lib/.generated/gen_hidden.dart", "lib/src/build/command.dart"}, 94)

	// What the extractor could not express is said in the IR, and the sample has nothing of the kind
	// (REVIEW-2026-11-19: nothing is left out without a trace).
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range prog.Funcs {
		if f.Dep {
			continue
		}
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if in.Op == ir.OpUnsupported {
					t.Errorf("%s: unsupported: %s", f.ID, in.Reason)
				}
			}
		}
	}
}

// Where the dependencies of a project are is said by its .dart_tool/package_config.json, which is
// not trusted: it can point at any directory. What is outside the project, the pub cache and the SDKs is
// not read unless it is asked for (yagihash/rivus #39), and the stream says that it was left out.
func TestDartDependenciesOutsideTheProjectAreNotReadUnlessAsked(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		t.Skip("RIVUS_DART is not set")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	project := filepath.Join(base, "project")
	for dir, files := range map[string]map[string]string{
		filepath.Join(outside, "lib"): {"secret.dart": "String leak() => 'sensitive-value';\n"},
		filepath.Join(project, "lib"): {"app.dart": "import 'package:other/secret.dart';\nString run() => leak();\n"},
		filepath.Join(project, ".dart_tool"): {"package_config.json": `{"configVersion":2,"packages":[` +
			`{"name":"other","rootUri":"` + (&url.URL{Scheme: "file", Path: outside + "/"}).String() + `","packageUri":"lib/"},` +
			`{"name":"app","rootUri":"../","packageUri":"lib/"}]}`},
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, text := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(project, "pubspec.yaml"), []byte("name: app\nenvironment:\n  sdk: ^3.8.0\ndependencies:\n  other:\n    path: ../outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "pubspec.yaml"), []byte("name: other\nenvironment:\n  sdk: ^3.8.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	extract := func(extra ...string) *ir.Program {
		words := strings.Fields(dart)
		args := append(words[1:], "run", "bin/rivus_dart.dart")
		args = append(args, extra...)
		args = append(args, project)
		cmd := exec.Command(words[0], args...)
		cmd.Dir = extractor
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("the extractor failed: %v", err)
		}
		prog, err := ir.Read(bytes.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		return prog
	}
	has := func(p *ir.Program, name string) bool {
		for _, f := range p.Funcs {
			if strings.HasSuffix(string(f.ID), name) {
				return true
			}
		}
		return false
	}

	refused := extract()
	if !has(refused, "app.dart#run") {
		t.Error("the project itself is not read")
	}
	if has(refused, "secret.dart#leak") {
		t.Error("a dependency outside the project was read without being asked")
	}
	said := false
	for _, e := range refused.LoadErrors {
		if strings.Contains(e.Message, "--allow-deps-under") {
			said = true
		}
	}
	if !said {
		t.Errorf("the stream does not say that a dependency was left out: %+v", refused.LoadErrors)
	}
	if allowed := extract("--allow-deps-under", outside); !has(allowed, "secret.dart#leak") {
		t.Error("a dependency under a directory that was asked for is not read")
	}
}

// A function that cannot be lowered costs that function, not the package: it is said in the stream
// (a load_error), and the rest is written (yagihash/rivus #37). RIVUS_DART_FAIL_IN makes one fail, as
// an input that the extractor does not expect would.
func TestDartExtractorLeavesOutAFunctionThatFails(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		t.Skip("RIVUS_DART is not set")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	cmd := exec.Command(words[0], append(words[1:], "run", "bin/rivus_dart.dart", "test/sample")...)
	cmd.Dir = extractor
	cmd.Env = append(os.Environ(), "RIVUS_DART_FAIL_IN=review_issues.dart#hiddenDirectory")
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed as a whole: %v", err)
	}
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	var failed, others int
	for _, f := range prog.Funcs {
		if strings.HasSuffix(string(f.ID), "review_issues.dart#hiddenDirectory") {
			t.Errorf("%s is in the stream", f.ID)
		}
		others++
	}
	for _, e := range prog.LoadErrors {
		if strings.HasSuffix(e.Package, "review_issues.dart#hiddenDirectory") && strings.Contains(e.Message, "left out") {
			failed++
		}
	}
	if failed != 1 {
		t.Errorf("%d load_errors for the function that failed, want 1: %+v", failed, prog.LoadErrors)
	}
	if others < 100 {
		t.Errorf("only %d functions were written: the rest of the package is lost", others)
	}
}

// The shapes of control flow that the spike's tests checked (yagihash/rivus #38): the finally block is
// copied for every way out of the try, and && || ?: are branches, not operators.
func TestDartExtractorControlFlowShapes(t *testing.T) {
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		t.Skip("RIVUS_DART is not set")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(dart)
	cmd := exec.Command(words[0], append(words[1:], "run", "bin/rivus_dart.dart", "test/sample")...)
	cmd.Dir = extractor
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor failed: %v", err)
	}
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	callsOf := func(fn, callee string) (calls, branches int) {
		for _, f := range prog.Funcs {
			if f.ID != ir.FuncID("package:sample/cfg_shapes.dart#"+fn) {
				continue
			}
			for _, b := range f.Blocks {
				if b.Term.Kind == ir.TermBranch {
					branches++
				}
				for _, in := range b.Instrs {
					if in.Op == ir.OpCall && in.Callee != nil && strings.HasSuffix(in.Callee.Symbol, "#"+callee) {
						calls++
					}
				}
			}
		}
		return
	}
	if n, _ := callsOf("finallyManyExits", "fin"); n < 4 {
		t.Errorf("finallyManyExits: the finally block is copied %d times, want one for each of the four ways out", n)
	}
	if n, _ := callsOf("nestedTry", "fin"); n < 2 {
		t.Errorf("nestedTry: the inner finally block is copied %d times, want 2 (the normal end and the exception)", n)
	}
	// a && (b || cond()) is two branches, the if is one, and ?: is one
	if calls, branches := callsOf("shortCircuit", "cond"); calls != 1 || branches < 4 {
		t.Errorf("shortCircuit: %d calls of cond and %d branches, want 1 call (made only when needed) and 4 or more branches", calls, branches)
	}
}
