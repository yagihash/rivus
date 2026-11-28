package docsgen

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// PassLineRow is a target of bench/deps-targets.json that has `pass_line` (the name of its row of the pass line).
type PassLineRow struct {
	Line   string
	Target string
	Lang   string
	Commit string
	Args   []string
	TimeS  float64
	MemGB  float64
}

// LoadPassLine reads the targets that have `pass_line`, in the order of their line names (A, C, D, then the others by name).
func LoadPassLine(file string) ([]PassLineRow, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	var out []PassLineRow
	for name, raw := range all {
		if strings.HasPrefix(name, "_") {
			continue
		}
		var t struct {
			PassLine string   `json:"pass_line"`
			Lang     string   `json:"lang"`
			Commit   string   `json:"commit"`
			Args     []string `json:"args"`
			TimeS    float64  `json:"time_limit_s"`
			MemGB    float64  `json:"mem_cap_gb"`
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("%s: %s: %w", file, name, err)
		}
		if t.PassLine == "" {
			continue
		}
		out = append(out, PassLineRow{t.PassLine, name, t.Lang, t.Commit, t.Args, t.TimeS, t.MemGB})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := len(out[i].Line) == 1, len(out[j].Line) == 1
		if a != b {
			return a
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Target < out[j].Target // the file is read into a map: targets of one line come in the order of their names
	})
	return out, nil
}

// PassLineTable renders the rows as Markdown. It holds the limits only; the results are in the ledger.
func PassLineTable(rows []PassLineRow) string {
	var b strings.Builder
	b.WriteString("<!-- 生成物(`make docs-gen WHAT=passline`)。bench/deps-targets.json の `pass_line` を持つ対象から作る。コミットしない -->\n\n# 合格基準の行\n\n")
	b.WriteString("| 行 | 対象 | 言語 | コミット | 追加のフラグ | 時間の上限 | メモリの上限 |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		c := r.Commit
		if len(c) > 8 {
			c = c[:8]
		}
		flags := "なし"
		if len(r.Args) > 0 {
			flags = "`" + strings.Join(r.Args, " ") + "`"
		}
		fmt.Fprintf(&b, "| %s | `%s` | %s | `%s` | %s | %g 秒 | %g GiB |\n", r.Line, r.Target, r.Lang, c, flags, r.TimeS, r.MemGB)
	}
	return b.String()
}
