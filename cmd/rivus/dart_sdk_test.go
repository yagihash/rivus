package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The Dart extractor can be a single executable (`dart compile exe`, which a formula can put in bin): it finds the Dart
// SDK (the `dart:*` libraries that the analysis reads) from RIVUS_DART_SDK or from the `dart` that PATH finds, not only
// from where its own executable is. The zoo gives the same findings as with `dart run`.
func TestDartExtractorCompiledToOneExecutable(t *testing.T) {
	t.Parallel()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	words := strings.Fields(dart)
	src, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	// The executable TestMain compiled from this same source (dartExePath, kept outside the SDK and the extractor directory) is the
	// one under test; it is compiled here only when TestMain did not (RIVUS_DART_FROM_SOURCE=1, or the compile failed there).
	exe := dartExePath
	if exe == "" {
		exe = filepath.Join(t.TempDir(), "rivus-dart")
		build := exec.Command(words[0], append(append([]string{}, words[1:]...), "compile", "exe", filepath.Join(src, "bin", "rivus_dart.dart"), "-o", exe)...)
		build.Dir = src
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("dart compile exe: %v\n%s", err, out)
		}
	}
	zooDartRoot(t) // writes the pubspec.yaml of bench/zoo-dart/cells (generated, not committed)
	zoo, err := filepath.Abs("../../bench/zoo-dart/cells")
	if err != nil {
		t.Fatal(err)
	}
	findings := func(extractor string) json.RawMessage {
		cmd := exec.Command(binary, "analyze", "-rules", "rules.yaml", "./...")
		cmd.Dir = zoo
		cmd.Env = append(os.Environ(), "RIVUS_DART_EXTRACTOR="+extractor)
		var out bytes.Buffer
		cmd.Stdout = &out
		// exit code 4 (not complete): the zoo has the chains that stop by the pass limit on purpose (F/state/global-chain); the test compares findings
		if err := notFailed(cmd.Run()); err != nil {
			t.Fatalf("%s: %v", extractor, err)
		}
		var j struct {
			Findings json.RawMessage `json:"findings"`
		}
		if err := json.Unmarshal(out.Bytes(), &j); err != nil {
			t.Fatal(err)
		}
		return j.Findings
	}
	want := findings(dart + " run " + filepath.Join(src, "bin", "rivus_dart.dart"))
	if got := findings(exe); string(got) != string(want) {
		t.Errorf("the compiled extractor (outside the SDK, found by PATH) gives other findings than dart run")
	}
}
