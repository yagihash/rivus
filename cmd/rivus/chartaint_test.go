package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// -char-taint (#185): a byte or a rune read out of a text carries the taint of the text, through the arithmetic with a
// constant and back into a string. A length, an index, a comparison and a number made of the character do not.
const charTaintSrc = `package main

import (
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

func main() {}

// --- the character carries the taint

func fIndex(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	var out []byte
	for i := 0; i < len(s); i++ {
		out = append(out, s[i])
	}
	exec.Command("sh", "-c", string(out)).Run()
}

func fRange(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	var out []byte
	for _, c := range s {
		out = append(out, byte(c))
	}
	exec.Command("sh", "-c", string(out)).Run()
}

func fUpper(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		out = append(out, c)
	}
	exec.Command("sh", "-c", string(out)).Run()
}

func fPlusOne(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	var out []byte
	for i := 0; i < len(s); i++ {
		out = append(out, s[i]+1)
	}
	exec.Command("sh", "-c", string(out)).Run()
}

type lexer struct {
	in  string
	pos int
	ch  byte
}

func (l *lexer) read() {
	l.ch = l.in[l.pos]
	l.pos++
}

func fField(w http.ResponseWriter, r *http.Request) {
	l := &lexer{in: r.FormValue("x")}
	var out []byte
	for l.pos < len(l.in) {
		l.read()
		out = append(out, l.ch)
	}
	exec.Command("sh", "-c", string(out)).Run()
}

func one(c byte) string { return string(c) }

func fHelper(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	out := ""
	for i := 0; i < len(s); i++ {
		out += one(s[i])
	}
	exec.Command("sh", "-c", out).Run()
}

func escape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' {
			b.WriteString("''")
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func fEscape(w http.ResponseWriter, r *http.Request) {
	exec.Command("sh", "-c", escape(r.FormValue("x"))).Run()
}

const digits = "0123456789abcdef"

// a table looked up by a character: the value is made of it (hex)
func fTable(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	var out []byte
	for i := 0; i < len(s); i++ {
		out = append(out, digits[s[i]>>4])
	}
	exec.Command("sh", "-c", string(out)).Run()
}

// --- it does not

func nLen(w http.ResponseWriter, r *http.Request) {
	exec.Command("sh", "-c", strconv.Itoa(len(r.FormValue("x")))).Run()
}

func nIndex(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	n := 0
	for i := 0; i < len(s); i++ {
		n = i
	}
	exec.Command("sh", "-c", strconv.Itoa(n)).Run()
}

func nCompare(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	ok := s[0] == '/'
	exec.Command("sh", "-c", strconv.FormatBool(ok)).Run()
}

// the length of what was made of characters: the characters are in the string, the length is not
func nLenOfChars(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	var out []byte
	for i := 0; i < len(s); i++ {
		out = append(out, s[i])
	}
	exec.Command("sh", "-c", strconv.Itoa(len(string(out)))).Run()
}

// arithmetic on a number that was made of a character by a conversion to another number type: not a character any more
func nIntArith(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	n := int(s[0]) + 1
	m := n * 2
	exec.Command("sh", "-c", strconv.Itoa(m)).Run()
}

func nNumber(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	n := int(s[0] - '0')
	exec.Command("sh", "-c", strconv.Itoa(n)).Run()
}
`

type charTaintOut struct {
	Findings []struct {
		Source struct {
			Func string `json:"func"`
		} `json:"source"`
		Uncertainties []struct {
			Kind string `json:"kind"`
		} `json:"uncertainties"`
	} `json:"findings"`
}

func charTaintRun(t *testing.T, dir string, args ...string) (map[string][]string, string) {
	t.Helper()
	cmd := exec.Command(binary, append([]string{"analyze"}, append(args, "./...")...)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err = notFailed(err); err != nil {
		t.Fatalf("analyze %v: %v\n%s", args, err, stderr.String())
	}
	var r charTaintOut
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, f := range r.Findings {
		name := f.Source.Func[strings.LastIndex(f.Source.Func, ".")+1:]
		var kinds []string
		for _, u := range f.Uncertainties {
			kinds = append(kinds, u.Kind)
		}
		got[name] = kinds
	}
	return got, stderr.String()
}

func TestCharTaint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/ct\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(charTaintSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	flows := []string{"fIndex", "fRange", "fUpper", "fPlusOne", "fField", "fHelper", "fEscape", "fTable"}
	stops := []string{"nLen", "nIndex", "nCompare", "nNumber", "nLenOfChars", "nIntArith"}

	off, _ := charTaintRun(t, dir)
	for _, n := range append(append([]string{}, flows...), stops...) {
		if _, ok := off[n]; ok {
			t.Errorf("without the flag: a finding of %s", n)
		}
	}

	on, _ := charTaintRun(t, dir, "-char-taint")
	for _, n := range flows {
		kinds, ok := on[n]
		if !ok {
			t.Errorf("-char-taint: no finding of %s", n)
			continue
		}
		has := false
		for _, k := range kinds {
			has = has || k == "char_transform"
		}
		if !has {
			t.Errorf("-char-taint: %s has no char_transform note: %v", n, kinds)
		}
	}
	for _, n := range stops {
		if _, ok := on[n]; ok {
			t.Errorf("-char-taint: a finding of %s (a length, an index, a comparison or a number)", n)
		}
	}

	// the findings of the others do not change, and with -numeric-taint the flag does nothing (and says so once)
	both, msg := charTaintRun(t, dir, "-char-taint", "-numeric-taint")
	num, _ := charTaintRun(t, dir, "-numeric-taint")
	if len(both) != len(num) {
		t.Errorf("-char-taint with -numeric-taint changed the findings: %d, %d", len(both), len(num))
	}
	if strings.Count(msg, "-char-taint") != 1 {
		t.Errorf("want one line about -char-taint with -numeric-taint, got %q", msg)
	}

	// the same with one worker and with several, and run twice (what the mark does to the identity of an element does not
	// depend on the order of the work)
	one, _ := charTaintRun(t, dir, "-char-taint", "-workers", "1")
	many, _ := charTaintRun(t, dir, "-char-taint", "-workers", "4")
	if len(one) != len(on) || len(many) != len(on) {
		t.Errorf("findings with -char-taint: %d, %d (1 worker), %d (4 workers)", len(on), len(one), len(many))
	}
	for n, kinds := range on {
		if strings.Join(kinds, ",") != strings.Join(one[n], ",") || strings.Join(kinds, ",") != strings.Join(many[n], ",") {
			t.Errorf("%s: the notes depend on the workers: %v, %v, %v", n, kinds, one[n], many[n])
		}
	}
}
