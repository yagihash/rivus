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
	"github.com/yagihash/rivus/internal/report"
)

// rustc_public panics on an async closure (FIXME(async_closures) in the conversion of a type). The extractor catches the panic for
// each function: it skips the function (fn_skipped), says so in one line, and reads the others. The analysis says it in
// summary.functions_skipped, in a note at a call of a skipped function of the project (project_body_unread), and in a last
// line on the standard error (docs/DESIGN-rust-extractor-panic.md). extractor/rust/test/skipped is the crate (std only).
func TestRustSkipsTheFunctionsItCannotRead(t *testing.T) {
	bin := os.Getenv("RIVUS_RUST")
	if bin == "" {
		skipOrFail(t, "RIVUS_RUST")
	}
	bin, _ = filepath.Abs(bin)
	root, _ := filepath.Abs("../../extractor/rust/test/skipped")
	cmd := exec.Command(bin, "--no-deps", "--locked", "--offline", root)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stream, err := cmd.Output()
	if err != nil {
		t.Fatalf("the extractor must go on after the panic of a function: %v\n%s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "panicked at") {
		t.Errorf("the panic of a function that is skipped is said in one line, not as a panic:\n%s", stderr.String())
	}
	if n := strings.Count(stderr.String(), "rivus-rust: skipped "); n != 4 {
		t.Errorf("one line for each skipped function (4: the two closures of run, skipped_named and its closure), got %d:\n%s", n, stderr.String())
	}
	prog, err := ir.Read(bytes.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, f := range prog.Funcs {
		have[string(f.ID)] = true
	}
	for _, id := range []string{"skipped::before", "skipped::after", "skipped::flows", "skipped::calls_the_skipped"} {
		if !have[id] {
			t.Errorf("the function %s is read, and is not in the program", id)
		}
	}
	for _, id := range []string{"skipped::skipped_named", "skipped::run::{closure#0}"} {
		if have[id] {
			t.Errorf("the function %s could not be read, and is in the program", id)
		}
	}
	if len(prog.FnSkipped) != 4 {
		t.Fatalf("fn_skipped has %d records, want 4: %+v", len(prog.FnSkipped), prog.FnSkipped)
	}
	for _, s := range prog.FnSkipped {
		if s.Unit != "skipped" || s.Dependency || !strings.Contains(s.Why, "async_closures") {
			t.Errorf("fn_skipped %+v", s)
		}
	}

	dir := t.TempDir()
	irFile := filepath.Join(dir, "p.jsonl")
	if err := os.WriteFile(irFile, stream, 0o644); err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(root, "rules.yaml")
	an := exec.Command(binary, "analyze", "-ir", irFile, "-rules", rules, "-no-project-rules")
	var anErr bytes.Buffer
	an.Stderr = &anErr
	out, err := an.Output()
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, anErr.String())
	}
	var rep report.Report
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	fs := rep.Summary.FunctionsSkipped
	if fs == nil || fs.Functions != 4 || fs.Project != 4 || fs.Dependencies != 0 || len(fs.Crates) != 1 || fs.Crates[0].Crate != "skipped" || fs.Crates[0].Functions != 4 {
		t.Errorf("summary.functions_skipped = %+v", fs)
	}
	lines := strings.Split(strings.TrimSpace(anErr.String()), "\n")
	if last := lines[len(lines)-1]; !strings.Contains(last, "WARNING: 4 function(s) of the project were skipped") {
		t.Errorf("the last line of the standard error says that functions of the project were skipped: %q", last)
	}
	kinds := map[string][]string{} // the function that holds the sink -> the kinds of the notes of its finding
	for _, f := range rep.Findings {
		for _, u := range f.Uncertainties {
			kinds[f.Sink.Func] = append(kinds[f.Sink.Func], u.Kind)
		}
		if _, ok := kinds[f.Sink.Func]; !ok {
			kinds[f.Sink.Func] = nil
		}
	}
	if k, ok := kinds["skipped::flows"]; !ok || len(k) != 0 {
		t.Errorf("the flow that does not touch a skipped function is found, with no note: %v (%v)", ok, k)
	}
	if k := kinds["skipped::calls_the_skipped"]; len(k) != 1 || k[0] != "project_body_unread" {
		t.Errorf("the call of a skipped function of the project has the note project_body_unread: %v", k)
	}
}
