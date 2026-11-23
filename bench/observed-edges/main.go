// Command observed-edges records the calls that happen when the Go zoo runs, and lists the ones that the graph of
// `rivus index` does not have (docs/DESIGN-observed-calls.md). It is a bench tool: CI does not run it.
//
//	observed-edges run -zoo bench/zoo-go [-rivus <path to rivus>]  the whole way, in a copy of the zoo (make observed-edges-go)
//	observed-edges compare -graph g.db -observed obs.json          the comparison of a file of observed calls
//
// The recording is in the copy only: the original tree is not touched.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: observed-edges run|compare [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(os.Args[2:], os.Stdout)
	case "compare":
		err = cmdCompare(os.Args[2:], os.Stdout)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "observed-edges:", err)
		os.Exit(1)
	}
}

func cmdCompare(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	graph := fs.String("graph", "", "the SQLite file of `rivus index -graph`")
	obs := fs.String("observed", "", "the file of observed calls (the common form: lang, edges)")
	verbose := fs.Bool("v", false, "list the call sites of the over-approximation table")
	fs.Parse(args)
	g, err := OpenGraph(*graph)
	if err != nil {
		return err
	}
	var o Observed
	if err := readJSON(*obs, &o); err != nil {
		return err
	}
	Compare(o, g).Print(w, *verbose)
	return nil
}

func moduleOf(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[0] == "module" {
			return f[1], nil
		}
	}
	return "", fmt.Errorf("%s/go.mod: no module line", dir)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		t := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(t, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(t, b, 0o644)
	})
}

func sh(dir string, env []string, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), env...)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

func cmdRun(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	zoo := fs.String("zoo", "bench/zoo-go", "the module of the cases (not changed: it is copied)")
	rivus := fs.String("rivus", "", "the rivus binary (by default ./cmd/rivus of the directory it runs in is built into the work directory)")
	cells := fs.String("cells", "cells", "the directory of the code that is instrumented, in the module")
	cmd := fs.String("cmd", "cmd/observe", "the command that runs the cases, in the module")
	hook := fs.String("hook", "mark.Run(", "the text of the line of the command after which rvobs.Dump goes")
	harness := fs.String("harness", "mark,cmd,rvobs", "the packages of the module that are not the code that is read (comma separated)")
	tags := fs.String("tags", "zooexec", "the build tags of the command that runs the cases")
	runArgs := fs.String("run-args", "-out cases.json", "the arguments of the command that runs the cases (it runs in the copy)")
	outJSON := fs.String("json", "", "write the observed calls (the common form) here")
	keep := fs.String("keep", "", "work in this (new) directory and keep it; by default a temporary one that is removed")
	verbose := fs.Bool("v", false, "list the call sites of the over-approximation table")
	fs.Parse(args)
	module, err := moduleOf(*zoo)
	if err != nil {
		return err
	}
	work := *keep
	if work == "" {
		if work, err = os.MkdirTemp("", "observed-edges-"); err != nil {
			return err
		}
		defer os.RemoveAll(work)
	} else if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	rivusAbs := ""
	if *rivus != "" {
		if rivusAbs, err = filepath.Abs(*rivus); err != nil {
			return err
		}
	} else {
		rivusAbs = filepath.Join(work, "rivus")
		if err := sh(".", nil, "go", "build", "-o", rivusAbs, "./cmd/rivus"); err != nil {
			return err
		}
	}
	plain, inst := filepath.Join(work, "plain"), filepath.Join(work, "inst")
	for _, d := range []string{plain, inst} {
		if err := copyTree(*zoo, d); err != nil {
			return err
		}
	}
	rvobsPath := module + "/rvobs"
	if err := os.MkdirAll(filepath.Join(inst, "rvobs"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(inst, "rvobs", "rvobs.go"), []byte(rvobsSource), 0o644); err != nil {
		return err
	}
	n, err := InstrumentTree(filepath.Join(inst, *cells), rvobsPath)
	if err != nil {
		return err
	}
	if err := InsertDump(filepath.Join(inst, *cmd, "main.go"), *hook, rvobsPath); err != nil {
		return err
	}
	fmt.Fprintf(w, "instrumented %d functions (in a copy)\n", n)
	if err := sh(inst, nil, "go", "vet", "-tags", *tags, "./"+*cells+"/..."); err != nil {
		return fmt.Errorf("the instrumented copy does not pass go vet: %w", err)
	}
	// -l: no inlining, so that a closure has one name (an inlined one is called f.func1.func1, a built one f.func1.1)
	exe := filepath.Join(work, "observe")
	if err := sh(inst, nil, "go", "build", "-tags", *tags, "-gcflags=all=-l", "-o", exe, "./"+*cmd); err != nil {
		return err
	}
	rawPath := filepath.Join(work, "raw.json")
	if err := sh(inst, []string{"RVOBS_OUT=" + rawPath}, exe, strings.Fields(*runArgs)...); err != nil {
		return err
	}
	g := filepath.Join(work, "graph.db")
	if err := sh(plain, nil, rivusAbs, "index", "-C", ".", "-graph", g, "-o", filepath.Join(work, "index.json"), "./..."); err != nil {
		return err
	}
	var raw struct {
		Edges []RawEdge `json:"edges"`
	}
	if err := readJSON(rawPath, &raw); err != nil {
		return err
	}
	gr, err := OpenGraph(g)
	if err != nil {
		return err
	}
	instReal, err := filepath.EvalSymlinks(inst) // the runtime writes the real path (/private/var on macOS)
	if err != nil {
		return err
	}
	obs := Normalize(raw.Edges, gr, instReal, module, strings.Split(*harness, ","))
	if *outJSON != "" {
		b, _ := json.MarshalIndent(obs, "", " ")
		if err := os.WriteFile(*outJSON, b, 0o644); err != nil {
			return err
		}
	}
	Compare(obs, gr).Print(w, *verbose)
	return nil
}
