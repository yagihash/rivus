//go:build darwin

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// A configuration or credentials that an earlier run left in the CARGO_HOME of rivus are removed before the stage that has a network (cargo
// reads $CARGO_HOME/config.toml); the sources that were fetched are not.
func TestFetchDropsAConfigurationOfTheCargoHome(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	for _, f := range []string{"config.toml", "config", "credentials.toml", "registry/src/keep"} {
		p := filepath.Join(home, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	var said bytes.Buffer
	if err := dropConfigOfCargoHome(home, &said); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"config.toml", "config", "credentials.toml"} {
		if _, err := os.Stat(filepath.Join(home, f)); err == nil {
			t.Errorf("%s was left", f)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "registry", "src", "keep")); err != nil {
		t.Errorf("what was fetched is removed: %v", err)
	}
	if said.Len() == 0 {
		t.Errorf("it removed without saying so")
	}
}
