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

// AdrStates is the vocabulary of the 状態 of an ADR.
var AdrStates = []string{"有効", "一部覆った", "覆った"}

// AdrKeys are the keys of the header of an ADR file (docs/adr/README.md). The first two are required.
var AdrKeys = []string{"状態", "関連", "状態の詳細", "要旨"}

// Adr is one ADR file.
type Adr struct {
	Number    int
	HeadNum   int // the number in the title line `# ADR-NNN: ...`
	File      string
	Title     string
	Fields    map[string]string
	State     string // the first word of 状態
	Overrides []int  // the numbers after `→` in 状態
}

var (
	adrNameRe  = regexp.MustCompile(`^([0-9]{3})-[a-z0-9][a-z0-9-]*\.md$`)
	adrTitleRe = regexp.MustCompile(`^# ADR-([0-9]+): (.*)$`)
	adrRefRe   = regexp.MustCompile(`ADR-([0-9]+)`)
	adrStateRe = regexp.MustCompile(`^([^(（\s]+)`)
)

// AdrName reports whether name is `NNN-<name>.md` and gives NNN.
func AdrName(name string) (int, bool) {
	m := adrNameRe.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}
	n, _ := strconv.Atoi(m[1])
	return n, true
}

// ParseAdr reads the title line and the header of one ADR file.
func ParseAdr(name, body string) (Adr, error) {
	num, ok := AdrName(name)
	if !ok {
		return Adr{}, fmt.Errorf("%s: not NNN-<name>.md", name)
	}
	a := Adr{Number: num, File: name, Fields: map[string]string{}}
	lines := strings.Split(body, "\n")
	seenTitle := false
	for i, l := range lines {
		if i >= headLine {
			break
		}
		if !seenTitle {
			tm := adrTitleRe.FindStringSubmatch(l)
			if tm == nil {
				return a, fmt.Errorf("%s: the first line is not `# ADR-NNN: <title>`", name)
			}
			a.HeadNum, _ = strconv.Atoi(tm[1])
			a.Title = strings.TrimSpace(tm[2])
			seenTitle = true
			continue
		}
		if l == "" {
			if len(a.Fields) > 0 {
				break // the header ends at the blank line after its lines
			}
			continue
		}
		if fm := fieldRe.FindStringSubmatch(l); fm == nil {
			break
		} else {
			for _, k := range AdrKeys {
				if fm[1] == k {
					if _, dup := a.Fields[k]; dup {
						return a, fmt.Errorf("%s: the header has %q twice", name, k)
					}
					a.Fields[k] = strings.TrimSpace(fm[2])
				}
			}
		}
	}
	if !seenTitle {
		return a, fmt.Errorf("%s: empty file", name)
	}
	st := a.Fields["状態"]
	if sm := adrStateRe.FindStringSubmatch(st); sm != nil {
		a.State = sm[1]
	}
	if i := strings.Index(st, "→"); i >= 0 {
		for _, m := range adrRefRe.FindAllStringSubmatch(st[i:], -1) {
			n, _ := strconv.Atoi(m[1])
			a.Overrides = append(a.Overrides, n)
		}
	}
	return a, nil
}

// LoadAdr reads every `NNN-<name>.md` of dir, in the order of the numbers. Errors of one file do not stop the others.
func LoadAdr(dir string) ([]Adr, []error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{err}
	}
	var out []Adr
	var errs []error
	for _, e := range ents {
		if _, ok := AdrName(e.Name()); !ok {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		a, err := ParseAdr(e.Name(), string(b))
		if err != nil {
			errs = append(errs, err)
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Number != out[j].Number {
			return out[i].Number < out[j].Number
		}
		return out[i].File < out[j].File
	})
	return out, errs
}

// AdrTable renders the ADRs as Markdown (links are relative to docs/).
func AdrTable(adrs []Adr) string {
	var b strings.Builder
	b.WriteString("<!-- 生成物(`make docs-gen`)。ADR のファイル(docs/adr/)のヘッダから作る。コミットしない -->\n\n# ADR の一覧\n\n")
	b.WriteString("| 番号 | 題 | 状態 | 関連 | ファイル |\n| --- | --- | --- | --- | --- |\n")
	for _, a := range adrs {
		fmt.Fprintf(&b, "| ADR-%03d | %s | %s | %s | [%s](adr/%s) |\n", a.Number, cell(a.Title), cell(a.Fields["状態"]), cell(a.Fields["関連"]), a.File, a.File)
	}
	return b.String()
}
