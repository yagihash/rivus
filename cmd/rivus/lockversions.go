package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/yagihash/rivus/internal/model"
)

// The extractors of Dart and Rust do not write the versions of the dependencies into the IR (docs/DESIGN-package-models.md, 6.7 and
// 6.10), so the core reads them from the lock file of the project: the one that the build uses. Each reader gives the versions by
// package, in the order of the file; nothing for a file that is not there (a project that has not been locked: no note, no error).

// dartLockVersions reads pubspec.lock in dir: the hosted packages (a path or a git dependency has no version to compare).
func dartLockVersions(dir string) (map[string][]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "pubspec.lock"))
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var lock struct {
		Packages map[string]struct {
			Version string `yaml:"version"`
			Source  string `yaml:"source"`
		} `yaml:"packages"`
	}
	if err := yaml.Unmarshal(b, &lock); err != nil {
		return nil, fmt.Errorf("pubspec.lock: %w", err)
	}
	out := map[string][]string{}
	for name, p := range lock.Packages {
		if p.Source == "hosted" && p.Version != "" {
			out[name] = []string{p.Version}
		}
	}
	return out, nil
}

// unverifiedFromLocks is summary.unverified_versions for the languages whose versions are in a lock file (Dart; Rust is next): the lock file
// of -C, for each of them that was read. A lock file that cannot be read is said on the standard error, and gives no note.
func (o *analyzeOpts) unverifiedFromLocks(rules *model.Rules) []model.UnverifiedVersion {
	var out []model.UnverifiedVersion
	for _, lang := range o.lockLangs {
		var used map[string][]string
		var err error
		var source string
		var match func(pattern, name string) bool
		switch lang {
		case langDart:
			used, err = dartLockVersions(o.dir)
			source, match = "pubspec.lock", model.SamePackage
		default:
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "rivus: the versions of the %s dependencies are not compared with verified_versions: %v\n", lang, err)
			continue
		}
		out = append(out, rules.UnverifiedVersionsOf(used, source, match)...)
	}
	return out
}
