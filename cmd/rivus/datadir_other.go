//go:build !darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const pinnedToolchainNote = ""

func rustDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "rivus", "rust"), nil
}

func pinnedToolchainDir() (string, error) { return "", fmt.Errorf("no sandbox on this system") }
