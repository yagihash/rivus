package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A path longer than 200 steps keeps its first and last 100 steps; the ones in the middle are left out, and the
// output says how many (paths[].omitted_steps, and a depth_limit note "path_steps_omitted:" on the finding),
// so that the finding is not a clear one (PRINCIPLES.md, section 2). A path of 200 steps or less has neither.
func TestLongPathSaysWhatItOmitted(t *testing.T) {
	chain := func(n int) string {
		dir := t.TempDir()
		var b strings.Builder
		b.WriteString("package main\n\nimport (\n\t\"net/http\"\n\t\"os/exec\"\n)\n\n")
		b.WriteString("func main() { http.HandleFunc(\"/\", h); http.ListenAndServe(\":0\", nil) }\n")
		b.WriteString("func h(w http.ResponseWriter, r *http.Request) { f0(r.FormValue(\"x\")) }\n")
		for i := 0; i < n; i++ {
			if i < n-1 {
				fmt.Fprintf(&b, "func f%d(s string) { f%d(s) }\n", i, i+1)
			} else {
				fmt.Fprintf(&b, "func f%d(s string) { exec.Command(\"sh\", \"-c\", s).Run() }\n", i)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/chain\n\ngo 1.22\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	type result struct {
		Findings []struct {
			Confidence string `json:"confidence"`
			Paths      []struct {
				Steps        []json.RawMessage `json:"steps"`
				OmittedSteps int               `json:"omitted_steps"`
			} `json:"paths"`
			Uncertainties []struct {
				Kind   string `json:"kind"`
				Detail string `json:"detail"`
			} `json:"uncertainties"`
		} `json:"findings"`
	}
	run := func(n int) (steps, omitted int, notes []string, conf string) {
		var r result
		if err := json.Unmarshal(analyzeWith(t, chain(n), "", "./..."), &r); err != nil {
			t.Fatal(err)
		}
		if len(r.Findings) != 1 {
			t.Fatalf("chain of %d: %d findings", n, len(r.Findings))
		}
		f := r.Findings[0]
		for _, u := range f.Uncertainties {
			if u.Kind == "depth_limit" && strings.HasPrefix(u.Detail, "path_steps_omitted:") {
				notes = append(notes, u.Detail)
			}
		}
		return len(f.Paths[0].Steps), f.Paths[0].OmittedSteps, notes, f.Confidence
	}

	// the walk back from the sink to the source stops after 200 reasons: 600 assignments in one function
	{
		dir := t.TempDir()
		var b strings.Builder
		b.WriteString("package main\n\nimport (\n\t\"net/http\"\n\t\"os/exec\"\n)\n\nfunc main() { http.HandleFunc(\"/\", h); http.ListenAndServe(\":0\", nil) }\n")
		b.WriteString("func h(w http.ResponseWriter, r *http.Request) {\n\tx0 := r.FormValue(\"x\")\n")
		for i := 1; i < 600; i++ {
			fmt.Fprintf(&b, "\tx%d := x%d + \"a\"\n", i, i-1)
		}
		b.WriteString("\texec.Command(\"sh\", \"-c\", x599).Run()\n}\n")
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/walk\n\ngo 1.22\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var r result
		if err := json.Unmarshal(analyzeWith(t, dir, "", "./..."), &r); err != nil {
			t.Fatal(err)
		}
		if len(r.Findings) != 1 {
			t.Fatalf("walk: %d findings", len(r.Findings))
		}
		n := 0
		for _, u := range r.Findings[0].Uncertainties {
			if u.Kind == "depth_limit" && strings.Contains(u.Detail, "path_steps_omitted: the path was followed back") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("a path whose walk back was cut has %d notes of it, want 1", n)
		}
	}

	// CORE-1: a step that is left out of the view still tells what it has. Step 150 of 300 is a call through an
	// interface with two implementations: the finding has the dynamic_dispatch note.
	{
		dir := t.TempDir()
		var b strings.Builder
		b.WriteString("package main\n\nimport (\n\t\"net/http\"\n\t\"os/exec\"\n)\n\n")
		b.WriteString("type I interface{ do(s string) }\ntype A struct{}\ntype B struct{}\n")
		b.WriteString("func (A) do(s string) { f151(s) }\nfunc (B) do(s string) { f151(s) }\n")
		b.WriteString("func main() { http.HandleFunc(\"/\", h); http.ListenAndServe(\":0\", nil) }\n")
		b.WriteString("func h(w http.ResponseWriter, r *http.Request) { f0(r.FormValue(\"x\")) }\n")
		for i := 0; i < 300; i++ {
			switch {
			case i == 150:
				b.WriteString("func f150(s string) { var i I = A{}; if len(s) > 3 { i = B{} }; i.do(s) }\n")
			case i == 299:
				b.WriteString("func f299(s string) { exec.Command(\"sh\", \"-c\", s).Run() }\n")
			default:
				fmt.Fprintf(&b, "func f%d(s string) { f%d(s) }\n", i, i+1)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/mid\n\ngo 1.22\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var r result
		if err := json.Unmarshal(analyzeWith(t, dir, "", "./..."), &r); err != nil {
			t.Fatal(err)
		}
		if len(r.Findings) != 1 {
			t.Fatalf("mid: %d findings", len(r.Findings))
		}
		f := r.Findings[0]
		dispatch := false
		for _, u := range f.Uncertainties {
			dispatch = dispatch || u.Kind == "dynamic_dispatch"
		}
		if f.Paths[0].OmittedSteps == 0 || !dispatch {
			t.Errorf("the call through the interface at step 150: omitted %d, dynamic_dispatch in the finding: %v", f.Paths[0].OmittedSteps, dispatch)
		}
	}

	if steps, omitted, notes, _ := run(100); omitted != 0 || len(notes) != 0 || steps > 200 {
		t.Errorf("a short path: steps %d, omitted %d, notes %v", steps, omitted, notes)
	}
	steps, omitted, notes, conf := run(300)
	if steps != 200 || omitted == 0 || len(notes) != 1 || conf == "no_uncertainty_on_path" {
		t.Errorf("a path of more than 200 steps: steps %d, omitted %d, notes %v, confidence %s", steps, omitted, notes, conf)
	}
}
