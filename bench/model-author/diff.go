package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yagihash/rivus/internal/model"
)

// The differential test (bench/model-diff) is run as it is: this file starts it and reads its report; the comparison is not done here.

// diffReport is the part of the report of bench/model-diff that is read here.
type diffReport struct {
	Counts     map[string]int `json:"counts"`
	Unverified []struct {
		Symbol string `json:"symbol"`
		Reason string `json:"reason"`
	} `json:"unverified"`
	Errors []string `json:"errors"`
}

func runDiff(out io.Writer, o *options, repo, target, slug string, l *loaded, api *API, m *Matched, summary string) error {
	reportPath := filepath.Join(o.work, slug+"-model-diff.json")
	args := []string{"run", "./bench/model-diff", "-pkg", o.pkg, "-lang", o.lang, "-repo", repo, "-json", reportPath, "-out", filepath.Join(o.work, "diff-"+slug)}
	if l.Version != "" {
		args = append(args, "-version", l.Version)
	}
	c := exec.Command("go", args...)
	c.Dir = repo
	var buf bytes.Buffer
	c.Stdout, c.Stderr = &buf, &buf
	err := c.Run()
	fmt.Fprint(out, buf.String())
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			return &exitError{ExitLoad, err}
		}
		code = ee.ExitCode()
	}
	var rep diffReport
	if data, rerr := os.ReadFile(reportPath); rerr == nil {
		_ = json.Unmarshal(data, &rep)
	}
	material := filepath.Join(o.work, slug+"-pr-material.md")
	if werr := os.WriteFile(material, []byte(Material(o.pkg, l, api, m, &rep, code, buf.String(), summary)), 0o644); werr != nil {
		return werr
	}
	fmt.Fprintf(out, "  material for the pull request: %s\n", material)
	switch code {
	case 0:
	case 1:
		return &exitError{ExitMiss, fmt.Errorf("model-diff did not pass (MISS %d, EXTRA %d, errors %d)", rep.Counts["MISS"], rep.Counts["EXTRA"], len(rep.Errors))}
	default:
		return &exitError{ExitLoad, fmt.Errorf("model-diff could not run (exit %d)", code)}
	}
	if o.writeVerified {
		if l.Module == "" || l.Version == "" {
			return &exitError{ExitUsage, fmt.Errorf("verified_versions is for a module that was fetched (the standard library has no version)")}
		}
		text, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		day := time.Now().Format("2006-01-02")
		upd, err := UpdateVerified(string(text), l.Module, l.Version, day)
		if err != nil {
			return err
		}
		if upd != string(text) {
			if err := os.WriteFile(target, []byte(upd), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(out, "  verified_versions written: %s %s (%s)\n", l.Module, l.Version, target)
		}
	}
	return nil
}

// Material is the text that the pull request takes its numbers from (a work file: it is not committed).
func Material(pkg string, l *loaded, api *API, m *Matched, rep *diffReport, code int, log string, summary string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# model-author: %s %s\n\n", pkg, l.Version)
	fmt.Fprintf(&b, "- layer 1: %d candidates (%d with a row, %d in rejected:, %d deferred:, %d undecided)\n", len(api.Items), len(m.ByRow), len(m.ByRejected), len(m.ByDeferred), len(m.Undecided))
	b.WriteString("\n## rejected: and deferred: (for the review)\n\n```\n" + strings.TrimSpace(summary) + "\n```\n\n")
	fmt.Fprintf(&b, "- layer 2: %d functions (list only)\n", len(api.Layer2))
	fmt.Fprintf(&b, "- stale rejected: entries: %d\n", len(m.Stale))
	fmt.Fprintf(&b, "- model-diff: exit %d; ", code)
	var keys []string
	for k := range rep.Counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s %d, ", k, rep.Counts[k])
	}
	fmt.Fprintf(&b, "unverified %d, errors %d\n\n", len(rep.Unverified), len(rep.Errors))
	b.WriteString("```\n" + strings.TrimSpace(log) + "\n```\n")
	return b.String()
}

var versionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

// VerifiedRange is the range that a check of one version stands for: up to the next major version (up to the next minor for 0.x).
func VerifiedRange(version string) (string, error) {
	m := versionRe.FindStringSubmatch(version)
	if m == nil {
		return "", fmt.Errorf("%q is not a version", version)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	base := m[1] + "." + m[2] + "." + m[3]
	if major == 0 {
		return fmt.Sprintf(">=%s <0.%d.0", base, minor+1), nil
	}
	return fmt.Sprintf(">=%s <%d.0.0", base, major+1), nil
}

// UpdateVerified writes the record of the module into verified_versions of a model file: it replaces the record of the package if
// there is one, adds one to the list if there is a list, and adds the list before `sinks:` (or at the end) if there is none.
func UpdateVerified(text, module, version, day string) (string, error) {
	rng, err := VerifiedRange(version)
	if err != nil {
		return "", err
	}
	block := []string{
		"  - package: " + module,
		fmt.Sprintf("    versions: %q", rng),
		"    checked_with: " + version,
		"    checked: " + day,
	}
	lines := strings.Split(text, "\n")
	head := -1
	for i, l := range lines {
		if l == "verified_versions:" {
			head = i
		}
	}
	if head < 0 {
		// add the whole key before the first top-level section, or at the end
		at := len(lines)
		for i, l := range lines {
			if strings.HasPrefix(l, "sources:") || strings.HasPrefix(l, "sinks:") || strings.HasPrefix(l, "sanitizers:") || strings.HasPrefix(l, "propagators:") {
				at = i
				break
			}
		}
		add := append([]string{"verified_versions:"}, block...)
		add = append(add, "")
		return strings.Join(append(append(append([]string{}, lines[:at]...), add...), lines[at:]...), "\n"), nil
	}
	end := head + 1
	for end < len(lines) && (strings.HasPrefix(lines[end], "  ")) {
		end++
	}
	// entries between head+1 and end
	for i := head + 1; i < end; {
		if strings.HasPrefix(lines[i], "  - package: ") {
			j := i + 1
			for j < end && !strings.HasPrefix(lines[j], "  - ") {
				j++
			}
			if strings.TrimSpace(strings.TrimPrefix(lines[i], "  - package: ")) == module {
				// a version that the record already covers changes nothing (the file is not touched: no new range, no new day)
				for _, l := range lines[i+1 : j] {
					if v, ok := strings.CutPrefix(strings.TrimSpace(l), "versions:"); ok {
						rng := strings.Trim(strings.TrimSpace(v), "\"'")
						if (model.VerifiedVersion{Package: module, Versions: rng}).Holds(version) {
							return text, nil
						}
					}
				}
				return strings.Join(append(append(append([]string{}, lines[:i]...), block...), lines[j:]...), "\n"), nil
			}
			i = j
			continue
		}
		i++
	}
	return strings.Join(append(append(append([]string{}, lines[:end]...), block...), lines[end:]...), "\n"), nil
}

var deadRe = regexp.MustCompile(`(?m)^\s+(?:go|dart) (\S+) \| *$`)

// ParseDeadRows reads the symbols that the test lists as not hit (RIVUS_DEAD_ROWS_PRINT=1) for Go or Dart.
func ParseDeadRows(log string) []string {
	var out []string
	for _, m := range deadRe.FindAllStringSubmatch(log, -1) {
		out = append(out, m[1])
	}
	return out
}

// DeadAllowLines are the lines to add: the rows of the package that no cell hits and the file does not allow yet.
func DeadAllowLines(dead []string, pkg, existing string) []string {
	have := map[string]bool{}
	for _, l := range strings.Split(existing, "\n") {
		if s, _, ok := strings.Cut(l, " | "); ok {
			have[strings.TrimSpace(s)] = true
		}
	}
	var out []string
	for _, s := range dead {
		if inPackage(s, pkg) && !have[s] {
			out = append(out, s+" | dependency: no cell of the zoo calls it (the package is not in the zoo's dependencies); the declaration is checked by bench/model-diff")
		}
	}
	sort.Strings(out)
	return out
}

func writeDeadAllow(out io.Writer, repo, target, pkg, lang string, f *Facts) error {
	if target == "" {
		return &exitError{ExitUsage, fmt.Errorf("-write-dead-allow needs the model file (-model)")}
	}
	if lang == "dart" && os.Getenv("RIVUS_DART") == "" {
		return &exitError{ExitLoad, fmt.Errorf("-write-dead-allow for Dart runs the zoo test, which reads Dart with the extractor: set RIVUS_DART to the dart command (RIVUS_DART=dart), and run `dart pub get` in extractor/dart")}
	}
	c := exec.Command("go", "test", "./cmd/rivus", "-run", "^TestBuiltinRowsAreHitByTheZoo$/"+lang+"$", "-count=1", "-v")
	c.Dir = repo
	c.Env = append(os.Environ(), "RIVUS_DEAD_ROWS_PRINT=1")
	log, _ := c.CombinedOutput() // the test fails while a row is not allowed: that is what is read
	dead := ParseDeadRows(string(log))
	if len(dead) == 0 && !bytes.Contains(log, []byte("rows of "+lang+" are not hit")) {
		return &exitError{ExitLoad, fmt.Errorf("the test did not list the rows that are not hit:\n%s", log)}
	}
	path := strings.TrimSuffix(target, ".yaml") + ".dead-allow"
	old, _ := os.ReadFile(path)
	lines := DeadAllowLines(dead, pkg, string(old))
	if len(lines) == 0 {
		fmt.Fprintf(out, "  .dead-allow: nothing to add (%s)\n", path)
		return nil
	}
	text := string(old)
	if text == "" {
		text = "# The rows of " + filepath.Base(target) + " that no call in the zoo or the curated targets hits (TestBuiltinRowsAreHitByTheZoo). \"symbol | reason\"; see bench/parity/README.md.\n"
	} else if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "  .dead-allow: %d line(s) added to %s\n", len(lines), path)
	return nil
}
