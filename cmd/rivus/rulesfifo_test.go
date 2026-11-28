//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A project is not trusted: a rule file that it brings is read only when it is a regular file. `mkfifo .rivus/rules/a.yaml` made os.Open wait for
// a writer for ever (the -timeout of the analysis is not started yet at this point), and rivus hung.
func TestProjectRuleFileThatIsAPipeIsRefused(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, ".rivus", "rules")
	if err := os.MkdirAll(rules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(rules, "a.yaml"), 0o644); err != nil {
		t.Skipf("no named pipe here: %v", err)
	}
	c := &common{dir: dir, noBuiltin: true}
	done := make(chan error, 1)
	go func() {
		_, err := c.loadRules()
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("err = %v, want one that says that it is not a regular file", err)
		}
	case <-time.After(10 * time.Second):
		// (the goroutine is stuck in open: let it go by opening the other end)
		if f, err := os.OpenFile(filepath.Join(rules, "a.yaml"), os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
		t.Fatal("rivus waits for the writer of a pipe that the project brings")
	}
}

// A regular file is read as before.
func TestProjectRuleFileThatIsRegularIsRead(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, ".rivus", "rules")
	if err := os.MkdirAll(rules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rules, "a.yaml"), []byte("sources: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &common{dir: dir, noBuiltin: true}
	if _, err := c.loadRules(); err != nil && strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("a regular file was refused: %v", err)
	}
}
