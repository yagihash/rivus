package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The report holds snippets of the source: the file is made as the graph file is (0600, a file beside it moved into place), so that
// another user of the machine does not read it, and a run that stops half way does not leave half of a JSON in the place of the report.
func TestSaveMakesTheFilePrivateAndLeavesNothingBeside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "report.json")
	r := &Report{SchemaVersion: SchemaVersion}
	if err := r.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(path); err != nil { // replaces what is there
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}
	ents, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(ents) != 1 {
		t.Errorf("files beside the report: %v (%v)", ents, err)
	}
	if _, err := Load(path); err != nil {
		t.Errorf("what was saved cannot be loaded: %v", err)
	}
}

// A finding is its source and its sink, and each of them is found by its fingerprint: a finding whose source or sink has none is
// refused, as a site with none is (it was "not found", which says that the code was read and the place is not there).
func TestReadBookmarksRefusesAFindingWhoseSiteHasNoFingerprint(t *testing.T) {
	read := func(s string) error {
		path := filepath.Join(t.TempDir(), "bm.json")
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadBookmarks(path)
		return err
	}
	if err := read(`{"id":"f","source":{"id":"s1"},"sink":{"id":"s2"}}`); err == nil {
		t.Error("a finding with no fingerprint on its source and its sink was accepted")
	}
	if err := read(`{"id":"f","source":{"id":"s1","fingerprint":{"content":"a"}},"sink":{"id":"s2"}}`); err == nil {
		t.Error("a finding with no fingerprint on its sink was accepted")
	}
	if err := read(`{"id":"f","source":{"id":"s1","fingerprint":{"content":"a"}},"sink":{"id":"s2","fingerprint":{"content":"b"}}}`); err != nil {
		t.Errorf("a finding with a fingerprint on each: %v", err)
	}
	// with neither, the error names the source, every time (before, the two were looked at in the order of a map, and it was the source or the sink)
	for i := 0; i < 50; i++ {
		err := read(`{"id":"f","source":{"id":"s1"},"sink":{"id":"s2"}}`)
		if err == nil || !strings.Contains(err.Error(), "its source has no fingerprint") {
			t.Fatalf("the error of a finding with no fingerprint on either: %v", err)
		}
	}
}
