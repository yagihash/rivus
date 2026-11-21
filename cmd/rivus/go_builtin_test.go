package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The result of a builtin function is made of its arguments: min / max, and the builtins of package unsafe. A builtin
// that this version does not know is taken to give its result from its arguments, and is said. recover() says that
// the value of a panic is not followed.
const goBuiltinSrc = `package main

import (
	"net/http"
	"os/exec"
	"strconv"
	"unsafe"
)

func main() {}

func fMin(w http.ResponseWriter, r *http.Request) {
	x := r.FormValue("x")
	exec.Command("sh", "-c", min(x, "zzz")).Run()
}

func fMax(w http.ResponseWriter, r *http.Request) {
	x := r.FormValue("x")
	exec.Command("sh", "-c", max("", x)).Run()
}

func fUnsafeString(w http.ResponseWriter, r *http.Request) {
	b := []byte(r.FormValue("x"))
	exec.Command("sh", "-c", unsafe.String(&b[0], len(b))).Run()
}

func fUnsafeSlice(w http.ResponseWriter, r *http.Request) {
	b := []byte(r.FormValue("x"))
	sl := unsafe.Slice(&b[0], len(b))
	exec.Command("sh", "-c", string(sl)).Run()
}

func fStringData(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("x")
	p := unsafe.StringData(s)
	exec.Command("sh", "-c", unsafe.String(p, len(s))).Run()
}

func fRangeFunc(w http.ResponseWriter, r *http.Request) {
	x := r.FormValue("x")
	seq := func(yield func(string) bool) { yield(x) }
	for v := range seq {
		exec.Command("sh", "-c", v).Run()
	}
}

// no source in the result: min of constants, and the length of the input (a number)
func nMin(w http.ResponseWriter, r *http.Request) {
	_ = r.FormValue("x")
	exec.Command("sh", "-c", min("a", "b")).Run()
}

func nLen(w http.ResponseWriter, r *http.Request) {
	exec.Command("sh", "-c", strconv.Itoa(len(r.FormValue("x")))).Run()
}

func fRecover(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if e := recover(); e != nil {
			exec.Command("sh", "-c", e.(string)).Run()
		}
	}()
	panic(r.FormValue("x"))
}
`

type goBuiltinResult struct {
	Findings []struct {
		Source struct {
			Func string `json:"func"`
		} `json:"source"`
		Uncertainties []struct {
			Kind   string `json:"kind"`
			Detail string `json:"detail"`
		} `json:"uncertainties"`
	} `json:"findings"`
	Uncertainties []struct {
		Kind   string `json:"kind"`
		Detail string `json:"detail"`
	} `json:"uncertainties"`
}

func goBuiltinProject(t *testing.T) string {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/gb\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(goBuiltinSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuiltinResultIsMadeOfItsArguments(t *testing.T) {
	dir := goBuiltinProject(t)
	cmd := exec.Command(binary, "analyze", "./...")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var r goBuiltinResult
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	// findings are grouped by sink: each function has its own, so the source function names the finding
	got := map[string]bool{}
	for _, f := range r.Findings {
		got[f.Source.Func[strings.LastIndex(f.Source.Func, ".")+1:]] = true
	}
	for _, want := range []string{"fMin", "fMax", "fUnsafeString", "fUnsafeSlice", "fStringData", "fRangeFunc"} {
		if !got[want] {
			t.Errorf("no finding of %s", want)
		}
	}
	for _, not := range []string{"nMin", "nLen"} {
		if got[not] {
			t.Errorf("a finding of %s, which has no source in its result", not)
		}
	}
	// recover() says that the value of a panic is not followed
	said := false
	for _, u := range r.Uncertainties {
		said = said || (u.Kind == "unsupported_construct" && strings.Contains(u.Detail, "recover()"))
	}
	if !said {
		t.Errorf("no note of recover(): %+v", r.Uncertainties)
	}
}

// A builtin that this version does not know: its result comes from its arguments, and it is said on the way.
func TestUnknownBuiltinKeepsTheFlowAndSaysSo(t *testing.T) {
	dir := goBuiltinProject(t)
	ex := exec.Command(binary, "extract", "./...")
	ex.Dir = dir
	ir, err := ex.Output()
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	text := strings.ReplaceAll(string(ir), `"symbol":"builtin.min"`, `"symbol":"builtin.zzzunknown"`)
	if text == string(ir) {
		t.Fatal("no builtin.min in the IR")
	}
	file := filepath.Join(dir, "ir.jsonl")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "analyze", "-ir", file)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("analyze -ir: %v", err)
	}
	var r goBuiltinResult
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range r.Findings {
		if strings.HasSuffix(f.Source.Func, ".fMin") {
			found = true
			note := false
			for _, u := range f.Uncertainties {
				note = note || (u.Kind == "unsupported_construct" && strings.Contains(u.Detail, "builtin.zzzunknown"))
			}
			if !note {
				t.Errorf("the finding through an unknown builtin has no note of it: %+v", f.Uncertainties)
			}
		}
	}
	if !found {
		t.Error("the flow through an unknown builtin is gone")
	}
}
