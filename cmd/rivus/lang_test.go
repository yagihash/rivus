package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectLangs(t *testing.T) {
	dir := t.TempDir()
	if got := detectLangs(dir); len(got) != 0 {
		t.Errorf("empty directory: %v", got)
	}
	for _, f := range []string{"go.mod", "Cargo.toml", "pubspec.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := detectLangs(dir), []string{"go", "rust", "dart"}; !reflect.DeepEqual(got, want) && !reflect.DeepEqual(got, []string{"go", "dart", "rust"}) {
		t.Errorf("detectLangs = %v, want go, dart and rust", got)
	}
}

func TestLanguagesFlag(t *testing.T) {
	o := analyzeOpts{}
	o.dir = t.TempDir()
	o.lang = "dart, rust"
	got, err := o.languages()
	if err != nil || !reflect.DeepEqual(got, []string{"dart", "rust"}) {
		t.Errorf("languages = %v, %v", got, err)
	}
	o.lang = "cobol"
	if _, err := o.languages(); err == nil {
		t.Error("an unknown language is accepted")
	}
	o.lang = "auto"
	if got, _ := o.languages(); !reflect.DeepEqual(got, []string{"go"}) {
		t.Errorf("nothing to tell by: %v", got)
	}
}

func TestMissingExtractorIsAnErrorWhenAsked(t *testing.T) {
	o := analyzeOpts{}
	o.dir = t.TempDir()
	o.lang = "rust"
	o.rustExtractor = "/nonexistent/rivus-rust"
	if _, err := o.runExtractor("rust"); err == nil {
		t.Error("a missing extractor is not an error")
	}
}
