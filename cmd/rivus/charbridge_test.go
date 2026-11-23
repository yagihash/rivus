package main

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A dependency that reads its argument one character at a time and writes it again (range s, DecodeRuneInString) stops the
// data of a source at the character read (a rune is a number: not followed). The caller that passed data of a source there is
// told with numeric_stop (char_read_in_callee: <the function>); a call that passed a constant is not; no flow is carried, so the
// findings and their confidence do not change (#198).
func writeCharBridgeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	third, app := filepath.Join(root, "third"), filepath.Join(root, "app")
	write(t, filepath.Join(third, "go.mod"), "module example.com/third\n\ngo 1.21\n")
	write(t, filepath.Join(third, "third.go"), `package third

import "unicode/utf8"

// Range writes the runes of s again, one at a time.
func Range(s string) string {
	var out []byte
	for _, r := range s {
		out = append(out, byte(r))
	}
	return string(out)
}

// Decode does it with DecodeRuneInString.
func Decode(s string) string {
	var out []byte
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		i += n
		out = append(out, byte(r))
	}
	return string(out)
}

// Pass hands its argument to Range: the stop is passed on to the callers of Pass.
func Pass(s string) string { return Range(s) }

// Plain does not read characters.
func Plain(s string) string { return s + "x" }
`)
	write(t, filepath.Join(app, "go.mod"), "module example.com/app\n\ngo 1.21\n\nrequire example.com/third v0.0.0\n\nreplace example.com/third => ../third\n")
	write(t, filepath.Join(app, "main.go"), `package main

import (
	"net/http"
	"os/exec"
	"strings"

	"example.com/third"
)

func viaRange(r *http.Request) {
	exec.Command(third.Range(r.URL.Query().Get("q"))).Run()
}

func viaDecode(r *http.Request) {
	exec.Command(third.Decode(r.URL.Query().Get("q"))).Run()
}

func viaPass(r *http.Request) {
	exec.Command(third.Pass(r.URL.Query().Get("q"))).Run()
}

func plain(r *http.Request) {
	exec.Command(third.Plain(r.URL.Query().Get("q"))).Run()
}

func constant(r *http.Request) {
	exec.Command(third.Range("ls")).Run()
	exec.Command(third.Decode("ls")).Run()
}

type holder struct{ Name string }

// two layers of the project's own functions between the entry and the dependency: the stop is passed on from a field, from a
// value made of a parameter, from a local that holds it
func f1(h *holder) string { return third.Range(h.Name) }
func g1(h *holder) string { return f1(h) }

func f2(s string) string { return third.Range(strings.TrimSpace(s)) }
func g2(s string) string { return f2(strings.ToLower(s)) }

func f3(s string) string { t := s + "x"; return third.Range(t) }
func g3(s string) string { u := s[1:]; return f3(u) }

func entryField(r *http.Request) {
	exec.Command(g1(&holder{Name: r.URL.Query().Get("q")})).Run()
}

func entryTrim(r *http.Request) { exec.Command(g2(r.URL.Query().Get("q"))).Run() }

func entryLocal(r *http.Request) { exec.Command(g3(r.URL.Query().Get("q"))).Run() }

func entryConst(r *http.Request) {
	exec.Command(g1(&holder{Name: "ls"})).Run()
	exec.Command(g2("ls")).Run()
	exec.Command(g3("ls")).Run()
}

func main() {
	http.HandleFunc("/e1", func(w http.ResponseWriter, r *http.Request) { entryField(r) })
	http.HandleFunc("/e2", func(w http.ResponseWriter, r *http.Request) { entryTrim(r) })
	http.HandleFunc("/e3", func(w http.ResponseWriter, r *http.Request) { entryLocal(r) })
	http.HandleFunc("/e4", func(w http.ResponseWriter, r *http.Request) { entryConst(r) })
	http.HandleFunc("/r", func(w http.ResponseWriter, r *http.Request) { viaRange(r) })
	http.HandleFunc("/d", func(w http.ResponseWriter, r *http.Request) { viaDecode(r) })
	http.HandleFunc("/p", func(w http.ResponseWriter, r *http.Request) { viaPass(r) })
	http.HandleFunc("/n", func(w http.ResponseWriter, r *http.Request) { plain(r) })
	http.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) { constant(r) })
	http.ListenAndServe(":0", nil)
}
`)
	return app
}

type charBridgeReport struct {
	Findings      []json.RawMessage `json:"findings"`
	Uncertainties []struct {
		Kind     string `json:"kind"`
		Detail   string `json:"detail"`
		Func     string `json:"func"`
		Position struct {
			Line int `json:"line"`
		} `json:"position"`
	} `json:"uncertainties"`
}

// notes are the char_read_in_callee notes: "<the function of the call>: <the function that read the characters>"
func (r charBridgeReport) notes() []string {
	var out []string
	for _, u := range r.Uncertainties {
		if strings.Contains(u.Func, "$") {
			continue // the handlers of the fixture: they pass the request itself, which is a source by its type
		}
		if u.Kind == "numeric_stop" && strings.HasPrefix(u.Detail, "char_read_in_callee: ") {
			d := strings.TrimPrefix(u.Detail, "char_read_in_callee: ")
			out = append(out, u.Func[strings.LastIndex(u.Func, ".")+1:]+": "+d[:strings.Index(d, ":")])
		}
	}
	sort.Strings(out)
	return out
}

func TestCharReadInCalleeNote(t *testing.T) {
	app := writeCharBridgeFixture(t)
	run := func(args ...string) charBridgeReport {
		t.Helper()
		out := runIn(t, binary, app, nil, append([]string{"analyze", "-dep-dir", "../third"}, append(args, "./...")...)...)
		var r charBridgeReport
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return r
	}
	want := []string{
		"entryField: example.com/third.Range", // through two functions of the project, from a field
		"entryLocal: example.com/third.Range", // from a local that holds a value made of the parameter
		"entryTrim: example.com/third.Range",  // from a value made of the parameter
		"viaDecode: example.com/third.Decode",
		"viaPass: example.com/third.Range", // passed on by Pass
		"viaRange: example.com/third.Range",
	}
	for _, w := range []string{"1", "4"} {
		r := run("-workers", w)
		got := r.notes()
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("-workers %s: notes %q, want %q", w, got, want)
		}
	}
	// -numeric-taint follows the numbers: nothing stops, nothing is said
	if got := run("-numeric-taint").notes(); len(got) != 0 {
		t.Errorf("-numeric-taint: notes %q, want none", got)
	}
}
