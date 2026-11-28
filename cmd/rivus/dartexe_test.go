package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yagihash/rivus/internal/extractorhash"
)

// dartReal is the command of RIVUS_DART as it was set (before useDartExe puts the wrapper in its place), and dartExePath is the
// extractor that was compiled from the source of this tree ("" when it runs from source).
var dartReal, dartExePath string

// useDartExe compiles the Dart extractor once for the whole run of the tests (`dart run` from source starts for 5 s or more every
// time; the executable, in a fraction of a second, writes the same IR: TestDartExeSameIR), and puts a wrapper in the place of RIVUS_DART: the
// tests run `<RIVUS_DART> run <extractor>/bin/rivus_dart.dart <args>`, and the wrapper runs the executable for that, and dart for anything
// else (`pub get`, the other scripts). RIVUS_DART_FROM_SOURCE=1 keeps `dart run` from source (the other way of the comparison).
// RIVUS_DART_EXE_DIR is a directory that outlives the run (CI keeps it in a cache): the executable is looked for there, by its key (the hash of
// the source, the version of the SDK, the system: dartExeKey), and compiled there when it is not, so a stale one is never taken for the current one.
// Without it, the executable is compiled into the temporary directory of the run, as before.
func useDartExe(dir string) {
	dartReal = os.Getenv("RIVUS_DART")
	if dartReal == "" || os.Getenv("RIVUS_DART_FROM_SOURCE") == "1" {
		return
	}
	src, _ := filepath.Abs("../../extractor/dart")
	base := filepath.Join(dir, "dart-exe")
	if d := os.Getenv("RIVUS_DART_EXE_DIR"); d != "" {
		base = d
	}
	exe, _, err := dartExeFor(src, strings.Fields(dartReal)[0], base)
	if err != nil {
		if os.Getenv("RIVUS_REQUIRE_DART") != "" {
			fmt.Fprintf(os.Stderr, "rivus tests: the Dart extractor could not be compiled (run `dart pub get` in extractor/dart): %v\n", err)
			os.RemoveAll(dir)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "rivus tests: the Dart extractor could not be compiled, so dart run is used (%v)\n", err)
		return
	}
	dartExePath = exe
	w := filepath.Join(dir, "dart-wrapper.sh")
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = run ]; then case \"$2\" in *bin/rivus_dart.dart) shift 2; exec %q \"$@\";; esac; fi\nexec %s \"$@\"\n", exe, dartReal)
	if err := os.WriteFile(w, []byte(script), 0o755); err != nil {
		panic(err)
	}
	os.Setenv("RIVUS_DART", w)
}

// The compiled extractor and `dart run` write the same IR byte for byte, on the sample package and on a monorepo (which reads its
// path dependency). A difference between a compiled program and the same one run from source (what Platform says, the paths of
// resources) shows here.
func TestDartExeSameIR(t *testing.T) {
	t.Parallel()
	if dartReal == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	if dartExePath == "" {
		t.Skip("the extractor runs from source in this run")
	}
	ensureDartSamplePub(t, dartReal)
	extractor, _ := filepath.Abs("../../extractor/dart")
	words := strings.Fields(dartReal)
	for _, target := range []string{"test/sample", "test/monorepo/app"} {
		if _, err := os.Stat(filepath.Join(extractor, target, ".dart_tool", "package_config.json")); err != nil {
			t.Logf("%s: no package config (dart pub get was not run there), not compared", target)
			continue
		}
		run := func(name string, args ...string) []byte {
			cmd := exec.Command(name, args...)
			cmd.Dir = extractor
			var out, errb bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errb
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s %s: %v\n%s", name, target, err, errb.String())
			}
			return out.Bytes()
		}
		fromExe := run(dartExePath, target)
		fromSrc := run(words[0], append(append([]string{}, words[1:]...), "run", "bin/rivus_dart.dart", target)...)
		if !bytes.Equal(fromExe, fromSrc) {
			t.Errorf("%s: the compiled extractor writes other IR than dart run (%d and %d bytes)", target, len(fromExe), len(fromSrc))
		}
		if len(fromExe) == 0 {
			t.Errorf("%s: no IR", target)
		}
	}
}

// Each compile writes a file of its own (os.CreateTemp), not rivus-dart.tmp that two compiles share: before, the one that failed
// removed what the other was writing, whose rename then failed (or a half-written program was moved into place). A fake `dart` stands
// for the compiler: it records the file it was told to write (-o), and writes it. The compiles are one after the other: what is checked is
// the name, not the timing (a test of the timing was not stable on the CI).
func TestCompileDartExeWritesAFileOfItsOwn(t *testing.T) {
	work := t.TempDir()
	fake := filepath.Join(work, "fake-dart.sh")
	log := filepath.Join(work, "outs")
	script := "for a; do out=$a; done\necho \"$out\" >> '" + log + "'\necho built > \"$out\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	src, _ := filepath.Abs("../../extractor/dart")
	dir := filepath.Join(work, "exe")
	for i := 0; i < 2; i++ {
		if _, err := compileDartExe(src, "/bin/sh "+fake, dir); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	outs := strings.Fields(string(b))
	if len(outs) != 2 || outs[0] == outs[1] {
		t.Errorf("the two compiles were told to write %v: they share a file", outs)
	}
	for _, o := range outs {
		if filepath.Base(o) == "rivus-dart.tmp" {
			t.Errorf("the fixed name rivus-dart.tmp is back: %s", o)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "rivus-dart")); err != nil || strings.TrimSpace(string(b)) != "built" {
		t.Errorf("the executable: %q %v", b, err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Errorf("files are left in %s: %v", dir, ents)
	}
	// a compile that fails leaves nothing behind either
	if err := os.WriteFile(fake, []byte("exit 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir2 := filepath.Join(work, "exe2")
	if _, err := compileDartExe(src, "/bin/sh "+fake, dir2); err == nil {
		t.Error("a failed compile is an error")
	}
	if ents, _ := os.ReadDir(dir2); len(ents) != 0 {
		t.Errorf("files are left after a failed compile: %v", ents)
	}
}

// The hashes of the source of the extractors are set at build time (-ldflags -X, `make build`); a build without them leaves "unknown", the
// value that internal/extractorhash names, and the name of the variable is the one that the build gives (a renamed variable would be an
// -X that sets nothing, without an error).
func TestSourceHashVariablesAreTheOnesTheBuildSets(t *testing.T) {
	if rustSourceHash != extractorhash.Unknown || dartSourceHash != extractorhash.Unknown {
		t.Skip("rivus of this test was built with the hashes")
	}
	for _, f := range []string{"../../Makefile", "../../scripts/build-release-archives.sh"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"main.rustSourceHash", "main.dartSourceHash"} {
			if !strings.Contains(string(b), name) {
				t.Errorf("%s does not set %s", f, name)
			}
		}
	}
}
