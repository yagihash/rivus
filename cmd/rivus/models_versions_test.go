package main

import (
	"path/filepath"
	"testing"
)

func TestModelsVersionsListsTheRecords(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "r.yaml")
	write(t, f, "ruleset: t@1\nverified_versions:\n  - package: shelf_router\n    versions: \">=1.1.0 <2.0.0\"\n    checked_with: 1.1.5\n    checked: 2026-11-22\n")
	var got struct {
		V []struct {
			Package, Versions, Checked, Source string
			CheckedWith                        string `json:"checked_with"`
		} `json:"verified_versions"`
	}
	rivusJSON(t, &got, "models", "versions", "-no-builtin-rules", "-rules", f)
	if len(got.V) != 1 || got.V[0].Package != "shelf_router" || got.V[0].Versions != ">=1.1.0 <2.0.0" || got.V[0].CheckedWith != "1.1.5" || got.V[0].Checked != "2026-11-22" || got.V[0].Source != f {
		t.Fatalf("%+v", got)
	}
	// with the built-in rules and no record: a list, not null
	rivusJSON(t, &got, "models", "versions")
}
