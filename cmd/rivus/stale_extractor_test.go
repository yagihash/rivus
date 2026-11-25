package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An extractor of another version of the schema is not read, and the message says how to build the one of this rivus (#109).
func TestStaleExtractorStreamStops(t *testing.T) {
	stale := `{"schema_version":"rivus.ir/0"}` + "\n"
	for _, c := range []struct{ lang, want string }{
		{"rust", "rivus setup rust"},
		{"dart", "rivus setup dart"},
	} {
		_, err := readExtracted(c.lang, []byte(stale))
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "rivus.ir/0") {
			t.Errorf("%s: %v", c.lang, err)
		}
	}
	p := filepath.Join(t.TempDir(), "x.jsonl")
	if err := os.WriteFile(p, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readIR(p); err == nil || !strings.Contains(err.Error(), "older or newer") || !strings.Contains(err.Error(), "saved IR file") {
		t.Errorf("-ir (the message names a saved IR file): %v", err)
	}
}
