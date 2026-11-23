// Package docsgen builds the table of the issues from the files of docs/issues (the table is not committed: `make docs-gen`).
//
// The header of an issue file is the lines `- <key>: <value>` right under the `# ` title (docs/issues/README.md):
// 状態, 種類, 言語, ROADMAP の節, 守るもの, and the optional 題 (the title of the table, when the  line is too long for it). A key that is missing gives an empty cell.
package docsgen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// States is the vocabulary of 状態.
var States = []string{"open", "partial", "closed", "deferred", "wontfix"}

// Keys are the keys of the header.
var Keys = []string{"状態", "種類", "言語", "ROADMAP の節", "守るもの", "題"}

// Issue is one numbered issue file.
type Issue struct {
	Number int
	File   string // base name
	Title  string
	Fields map[string]string // the values of Keys (as written)
	State  string            // the first word of 状態, without the `**`
}

var (
	numRe    = regexp.MustCompile(`^([0-9]+)-[^/]+\.md$`)
	titleRe  = regexp.MustCompile(`^# (?:#?[0-9]+:? )?(.*)$`)
	fieldRe  = regexp.MustCompile(`^- ([^:]+): ?(.*)$`)
	stateRe  = regexp.MustCompile(`^\*{0,2}([a-z]+)`)
	headLine = 30 // the header is within the first lines
)

// Parse reads the header of one issue file.
func Parse(name, body string) (Issue, error) {
	m := numRe.FindStringSubmatch(name)
	if m == nil {
		return Issue{}, fmt.Errorf("%s: not <number>-<name>.md", name)
	}
	num, _ := strconv.Atoi(m[1])
	is := Issue{Number: num, File: name, Fields: map[string]string{}}
	lines := strings.Split(body, "\n")
	seenTitle := false
	for i, l := range lines {
		if i >= headLine {
			break
		}
		if !seenTitle {
			if tm := titleRe.FindStringSubmatch(l); tm != nil {
				is.Title = strings.TrimSpace(tm[1])
				seenTitle = true
			}
			continue
		}
		if strings.HasPrefix(l, "## ") {
			break
		}
		if fm := fieldRe.FindStringSubmatch(l); fm != nil {
			for _, k := range Keys {
				if fm[1] == k {
					if _, dup := is.Fields[k]; dup {
						return is, fmt.Errorf("%s: the header has %q twice", name, k)
					}
					is.Fields[k] = strings.TrimSpace(fm[2])
				}
			}
		}
	}
	if !seenTitle {
		return is, fmt.Errorf("%s: no title line (# ...)", name)
	}
	if t := is.Fields["題"]; t != "" {
		is.Title = t
	}
	if sm := stateRe.FindStringSubmatch(is.Fields["状態"]); sm != nil {
		is.State = sm[1]
	}
	return is, nil
}

// Load reads every numbered issue file of dir, in the order of the numbers. Errors of one file do not stop the others.
func Load(dir string) ([]Issue, []error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{err}
	}
	var out []Issue
	var errs []error
	for _, e := range ents {
		if !numRe.MatchString(e.Name()) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		is, err := Parse(e.Name(), string(b))
		if err != nil {
			errs = append(errs, err)
		}
		out = append(out, is)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, errs
}

func cell(s string) string {
	if s == "" {
		return ""
	}
	return strings.ReplaceAll(s, "|", `\|`)
}

// Table renders the issues as Markdown (links are relative to docs/): the open ones, then the closed ones.
func Table(issues []Issue) string {
	var b strings.Builder
	b.WriteString("<!-- 生成物(`make docs-gen`)。課題のファイル(docs/issues/)のヘッダから作る。コミットしない -->\n\n# 課題の一覧\n\n")
	for _, closed := range []bool{false, true} {
		if closed {
			b.WriteString("\n## 閉じたもの\n\n")
		} else {
			b.WriteString("## 開いているもの\n\n")
		}
		b.WriteString("| 番号 | 題 | 種類 | 言語 | ROADMAP の節 | 状態 | 守るもの |\n| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, is := range issues {
			if (is.State == "closed") != closed {
				continue
			}
			fmt.Fprintf(&b, "| [#%d](issues/%s) | %s | %s | %s | %s | %s | %s |\n", is.Number, is.File, cell(is.Title),
				cell(is.Fields["種類"]), cell(is.Fields["言語"]), cell(is.Fields["ROADMAP の節"]), is.State, cell(is.Fields["守るもの"]))
		}
	}
	return b.String()
}
