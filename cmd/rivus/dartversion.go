package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"

	"github.com/yagihash/rivus/internal/report"
)

// VerifiedDart is the minor of the Dart SDK that the Dart extractor of this rivus was verified with: the DART_SDK of .github/workflows/ci.yml
// (the SDK that CI runs the extractor and the benches with), minor only (a patch release does not change the language). A package whose
// pubspec.yaml needs a newer SDK is read by an extractor that was not checked against that language: summary.dart_version says so (ADR-127).
// A test fails when DART_SDK and this value differ.
const VerifiedDart = "3.13"

// dartSDKConstraint reads environment.sdk of pubspec.yaml in dir, as written ("" for a project without one: no pubspec.yaml, no environment, no sdk).
// A pubspec.yaml that cannot be read is an error (the answer is then "unknown", and the note on the standard error says why).
func dartSDKConstraint(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "pubspec.yaml"))
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	var spec struct {
		Environment struct {
			SDK yaml.Node `yaml:"sdk"`
		} `yaml:"environment"`
	}
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return "", fmt.Errorf("pubspec.yaml: %w", err)
	}
	// the text as written ("sdk: 3.0" is a number to YAML, and the text of it is what pub reads); a list or a map is not a constraint
	if n := spec.Environment.SDK; n.Kind == yaml.ScalarNode {
		return n.Value, nil
	}
	return "", nil
}

// dartVersionOf is summary.dart_version for the Dart package in dir (the directory of -C).
func dartVersionOf(dir string) (*report.DartVersion, error) {
	c, err := dartSDKConstraint(dir)
	if err != nil {
		return report.CheckDartVersion(VerifiedDart, ""), err
	}
	return report.CheckDartVersion(VerifiedDart, c), nil
}

// dartVersionWarning is the one line on the standard error for a package that needs a newer Dart than this rivus was verified with ("" for any
// other answer). It does not stop the run (ADR-127).
func dartVersionWarning(d *report.DartVersion) string {
	if d == nil || d.Match != report.DartNewer {
		return ""
	}
	return fmt.Sprintf("rivus: WARNING: the Dart package says sdk %s (Dart %s or newer), newer than Dart %s that this rivus was verified with: the language may have what rivus was not checked against, and the result may be incomplete (summary.dart_version)", d.Target, d.Minimum, d.Verified)
}

// readDart says whether the Dart extractor ran in this run (a run from -ir or -index has no project to read the pubspec.yaml of).
func (o *analyzeOpts) readDart() bool {
	for _, l := range o.lockLangs {
		if l == langDart {
			return true
		}
	}
	return false
}
