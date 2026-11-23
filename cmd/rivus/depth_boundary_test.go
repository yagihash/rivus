package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// The depth of the access paths in a dependency (-dep-k) against what is told apart: a struct in a package that is read as a
// dependency has a value in the field of its 3rd level (o.A.B.C), and two neighbours: D under the same 2nd-level node (o.A.B.D),
// and X under the same 1st-level node (o.A.X). The project reads the neighbour and passes it to a sink.
//
//	depth 3 (the old default): D and X are told apart from C: no finding for them
//	depth 2 (the default):     D is the same cell as C (/A/B): reported (precision lost, the note says the path was cut); X is not
//	depth 1:                   X is reported too
//
// The flow itself (C to the sink) is found at every depth: lowering the depth adds flows and never removes one. The truth of each
// depth is written out below, so that a change of the default or of the cut moves a line of this table and nothing else.
func TestDepthBoundaryGo(t *testing.T) {
	root := t.TempDir()
	third, app := filepath.Join(root, "third"), filepath.Join(root, "app")
	write(t, filepath.Join(third, "go.mod"), "module example.com/third\n\ngo 1.21\n")
	write(t, filepath.Join(third, "third.go"), `package third

type N3 struct{ C, D string }

type N2 struct {
	B N3
	X string
}

type Outer struct{ A N2 }

// Build puts the value into the field of the 3rd level; the neighbours hold constants.
func Build(v string) Outer {
	var o Outer
	o.A.B.C = v
	o.A.B.D = "clean"
	o.A.X = "clean"
	return o
}
`)
	write(t, filepath.Join(app, "go.mod"), "module example.com/app\n\ngo 1.21\n\nrequire example.com/third v0.0.0\n\nreplace example.com/third => ../third\n")
	write(t, filepath.Join(app, "main.go"), `package main

import (
	"net/http"
	"os/exec"

	"example.com/third"
)

func vuln(r *http.Request) {
	o := third.Build(r.URL.Query().Get("q"))
	exec.Command(o.A.B.C).Run()
}

func decoyLevel3(r *http.Request) {
	o := third.Build(r.URL.Query().Get("q"))
	exec.Command(o.A.B.D).Run()
}

func decoyLevel2(r *http.Request) {
	o := third.Build(r.URL.Query().Get("q"))
	exec.Command(o.A.X).Run()
}

func main() {
	http.HandleFunc("/v", func(w http.ResponseWriter, r *http.Request) { vuln(r) })
	http.HandleFunc("/d3", func(w http.ResponseWriter, r *http.Request) { decoyLevel3(r) })
	http.HandleFunc("/d2", func(w http.ResponseWriter, r *http.Request) { decoyLevel2(r) })
	http.ListenAndServe(":0", nil)
}
`)
	type finding struct {
		Sink struct {
			Func string `json:"func"`
		} `json:"sink"`
		Uncertainties []struct {
			Kind   string `json:"kind"`
			Detail string `json:"detail"`
		} `json:"uncertainties"`
	}
	type report struct {
		Findings []finding `json:"findings"`
		Summary  struct {
			AccessPathDepth *struct {
				Project    int `json:"project"`
				Dependency int `json:"dependency"`
			} `json:"access_path_depth"`
		} `json:"summary"`
	}
	run := func(args ...string) map[string]finding {
		t.Helper()
		out := runIn(t, binary, app, nil, append([]string{"analyze", "-dep-dir", "../third"}, append(args, "./...")...)...)
		var r report
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		got := map[string]finding{}
		for _, f := range r.Findings {
			name := f.Sink.Func[strings.LastIndex(f.Sink.Func, ".")+1:]
			got[name] = f
		}
		return got
	}
	// vuln: found at every depth. decoyLevel3 and decoyLevel2: reported where the cut puts the neighbour in the cell of the value.
	table := []struct {
		args                           []string
		vuln, decoyLevel3, decoyLevel2 bool
	}{
		{[]string{"-dep-k", "3"}, true, false, false}, // the old default
		{[]string{}, true, true, false},               // the default: dependencies at depth 2
		{[]string{"-dep-k", "2"}, true, true, false},
		{[]string{"-dep-k", "1"}, true, true, true},
		{[]string{"-dep-k", "0"}, true, false, false}, // 0: the same as -k (3)
	}
	for _, c := range table {
		got := run(c.args...)
		for name, want := range map[string]bool{"vuln": c.vuln, "decoyLevel3": c.decoyLevel3, "decoyLevel2": c.decoyLevel2} {
			if _, ok := got[name]; ok != want {
				t.Errorf("%v: %s reported = %v, want %v", c.args, name, ok, want)
			}
		}
		// where the neighbour is reported only because of the cut, the finding says that the path was cut
		if f, ok := got["decoyLevel3"]; ok {
			noted := false
			for _, u := range f.Uncertainties {
				noted = noted || u.Kind == "access_path_truncated"
			}
			if !noted {
				t.Errorf("%v: decoyLevel3 is reported because of the cut and has no access_path_truncated note", c.args)
			}
		}
	}
}
