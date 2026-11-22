package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanCargoConfig(t *testing.T) {
	for _, c := range []struct {
		name, text      string
		vendored, needs bool
	}{
		{"none", "[build]\njobs = 2\n", false, false},
		{"registry", "[registries.evil]\nindex = \"sparse+http://x/\"\n", false, true},
		{"dotted", "registries.evil.index = \"sparse+http://x/\"\n", false, true},
		{"alias", "[credential-alias]\nx = \"y\"\n", false, true},
		{"replaced by a registry", "[source.crates-io]\nreplace-with = \"m\"\n[source.m]\nregistry = \"sparse+http://x/\"\n", false, true},
		{"replaced by a repository", "[source.crates-io]\nreplace-with = \"m\"\n[source.m]\ngit = \"http://x/y\"\n", false, true},
		{"vendored", "[source.crates-io]\nreplace-with = \"vendored-sources\"\n[source.vendored-sources]\ndirectory = \"vendor\"\n", true, false},
		{"directory outside", "[source.crates-io]\nreplace-with = \"v\"\n[source.v]\ndirectory = \"../vendor\"\n", false, false},
	} {
		v, n := scanCargoConfig(c.text, "/p/proj")
		if v != c.vendored || n != c.needs {
			t.Errorf("%s: vendored %v needs %v, want %v %v", c.name, v, n, c.vendored, c.needs)
		}
	}
}

// A project whose configuration names a registry is stopped with the reason, before anything with a network runs.
func TestFetchStopsForAProjectThatNeedsItsConfiguration(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cargo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cargo", "config.toml"), []byte("[registries.evil]\nindex = \"sparse+http://127.0.0.1:1/\"\ncredential-provider = [\"./prov.sh\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := planFetch(dir)
	if err == nil || !strings.Contains(err.Error(), "-no-fetch") || !strings.Contains(err.Error(), "not used in the stage that fetches") {
		t.Errorf("not stopped with a reason: %v", err)
	}
}
