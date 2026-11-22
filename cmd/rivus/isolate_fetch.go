package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fetchPlan is what the cargo configuration of the project says about the stage that fetches.
type fetchPlan struct {
	hasConfig bool // .cargo/config(.toml) is there
	vendored  bool // it replaces the sources with a directory of the project: there is nothing to fetch
}

// planFetch reads, as text (nothing is run), the cargo configuration of the project. The configuration is never used in
// the stage that fetches (fetchArgs covers it). What is decided here is only whether that stage is skipped (the project
// is vendored) or stopped with a reason (the project needs its configuration to fetch: its own registry, a source that
// is replaced by something else).
func planFetch(dir string) (fetchPlan, error) {
	var p fetchPlan
	if fi, err := os.Lstat(filepath.Join(dir, ".cargo")); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return p, fmt.Errorf("%s/.cargo is a symbolic link: it cannot be kept out of the stage that fetches", dir)
	}
	for _, f := range []string{".cargo/config.toml", ".cargo/config"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		p.hasConfig = true
		vendored, needs := scanCargoConfig(string(b), dir)
		if vendored {
			p.vendored = true
		} else if needs {
			return p, fmt.Errorf("the cargo configuration of the project (%s) names registries or replaces sources. It is not used in the stage that fetches "+
				"(the stage has a network, and the configuration can make cargo run a program of the project there). "+
				"Fetch the dependencies yourself, or vendor them (`cargo vendor`, with [source] pointing at a directory of the project), then use -no-fetch", f)
		}
	}
	return p, nil
}

// scanCargoConfig looks at the text of a cargo configuration (a scan of its tables, not a parser). vendored: a [source.*]
// table has a `directory` inside the project. needs: it names registries, a registry, credential aliases, a patch, or a
// source of the kind registry or git. A mistake in either is harmless: the configuration is not used in the stage whatever
// this says (a wrong "vendored" skips the stage, a wrong "needs" stops it with a reason).
func scanCargoConfig(text, dir string) (vendored, needs bool) {
	table := ""
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasPrefix(l, "[") {
			table = strings.Trim(strings.TrimSpace(strings.Trim(l, "[]")), "\"'")
			l = ""
		}
		key, val, _ := strings.Cut(l, "=")
		key, val = strings.TrimSpace(key), strings.Trim(strings.TrimSpace(val), "\"' ")
		full := table
		if key != "" {
			full = strings.TrimPrefix(table+"."+key, ".")
		}
		for _, pre := range []string{"registries", "registry", "credential-alias", "patch"} {
			if full == pre || strings.HasPrefix(full, pre+".") {
				needs = true
			}
		}
		if !strings.HasPrefix(full, "source.") {
			continue
		}
		switch {
		case strings.HasSuffix(full, ".directory"):
			if filepath.IsAbs(val) {
				if rel, err := filepath.Rel(dir, val); err == nil && !strings.HasPrefix(rel, "..") {
					vendored = true
				}
			} else if val != "" && !strings.HasPrefix(filepath.Clean(val), "..") {
				vendored = true
			}
		case strings.HasSuffix(full, ".registry"), strings.HasSuffix(full, ".git"), strings.HasSuffix(full, ".local-registry"):
			needs = true
		}
	}
	if vendored {
		needs = false
	}
	return vendored, needs
}
