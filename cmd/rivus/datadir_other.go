//go:build !darwin && !linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const pinnedToolchainNote = ""

func rustDataDir() (string, error) {
	if d := os.Getenv("RIVUS_RUST_DATA_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "rivus", "rust"), nil
}

func pinnedToolchainDir() (string, error) { return "", fmt.Errorf("no sandbox on this system") }
