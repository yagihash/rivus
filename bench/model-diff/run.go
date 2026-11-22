package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// RunOptions: how `rivus analyze` is called.
type RunOptions struct {
	Rivus, Lang, Repo string
	Timeout           string
	Rules             []string // the built-in files of the run that reads the bodies, without the rows of the package
}

// analyze runs `rivus analyze` in the dir and returns its JSON, the time and the peak of the memory.
func analyze(o RunOptions, dir string, args ...string) ([]byte, Run, error) {
	a := []string{"analyze", "-lang", o.Lang, "-C", dir, "-detail", "summary", "-timeout", o.Timeout}
	a = append(a, args...)
	c := exec.Command(o.Rivus, a...)
	c.Env = os.Environ()
	if o.Lang == "dart" {
		c.Env = append(c.Env, "RIVUS_DART_EXTRACTOR=dart run "+filepath.Join(o.Repo, "extractor", "dart", "bin", "rivus_dart.dart"))
		if os.Getenv("RIVUS_DART") == "" {
			c.Env = append(c.Env, "RIVUS_DART=dart")
		}
	}
	var out, errb bytes.Buffer
	c.Stdout, c.Stderr = &out, &errb
	t0 := time.Now()
	err := c.Run()
	run := Run{Seconds: time.Since(t0).Seconds()}
	if ru, ok := c.ProcessState.SysUsage().(*syscall.Rusage); ok {
		rss := int64(ru.Maxrss)
		if runtime.GOOS == "darwin" {
			rss /= 1024
		}
		run.MaxRSS = rss / 1024
	}
	for _, l := range strings.Split(errb.String(), "\n") {
		if strings.Contains(l, "not built") && (strings.Contains(l, " fx (") || strings.Contains(l, "fx/stdmirror")) {
			return nil, run, fmt.Errorf("the generated code does not build (a bug of the generator, or of the package): %s", l)
		}
	}
	if out.Len() == 0 {
		return nil, run, fmt.Errorf("rivus analyze: %v\n%s", err, errb.String())
	}
	return out.Bytes(), run, nil
}

// runPair runs A (the built-in rules without the rows of the package, the bodies read) and B (the built-in rules, and the model if it is not one of them).
func runPair(o RunOptions, gen *Gen, modelFile string, builtinIn bool, filtered []string) (a, b Hits, runs map[string]Run, err error) {
	runs = map[string]Run{}
	var extra []string
	if !builtinIn {
		extra = []string{"-rules", modelFile}
	}
	if gen.HasFlows {
		args := []string{"-no-builtin-rules", "-read-declared-bodies"}
		for _, f := range filtered {
			args = append(args, "-rules", f)
		}
		args = append(args, "-rules", filepath.Join(gen.DirA, "rules.yaml"))
		data, run, e := analyze(o, gen.DirA, args...)
		if e != nil {
			return nil, nil, nil, e
		}
		runs["A"] = run
		if a, err = ReadHits(data); err != nil {
			return
		}
	}
	data, run, e := analyze(o, gen.Dir, append(extra, "-rules", filepath.Join(gen.Dir, "rules.yaml"))...)
	if e != nil {
		return nil, nil, nil, e
	}
	runs["B"] = run
	b, err = ReadHits(data)
	return
}
