package main

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// mirrorStd makes the code that the bodies are read from for a package of the standard library. rivus does not read the standard library
// (the models stand for it, ADR-015), so the files of the package are copied into the module as a package of its own
// (`stdmirror/<path>`) and the same calls are made to the copy: the models of the package do not apply to it, those of the others do.
// The `internal/...` packages that it imports are copied the same way (a package of another module may not import them), and the
// files are chosen for a machine that has no assembly, so the Go version of each function is the one that is read. The assembly
// files are not copied (a function that has no body there is read as a function without a body). A package that cannot be copied
// makes its rows "unverified".
func mirrorStd(o GoOptions, g *goGen, dirB string) (string, error) {
	dirA := filepath.Join(filepath.Dir(dirB), "fxA")
	if err := os.RemoveAll(dirA); err != nil {
		return "", err
	}
	ctx := build.Default
	ctx.GOARCH = "wasm" // a machine with few files per package (the assembly is not copied: a function that has none is read as one without a body)
	ctx.CgoEnabled = false
	ctx.ToolTags = nil // no goexperiment: the register ABI is not for this machine
	done := map[string]bool{}
	var copyPkg func(path string) error
	copyPkg = func(path string) error {
		if done[path] {
			return nil
		}
		done[path] = true
		src := filepath.Join(runtime.GOROOT(), "src", path)
		bp, err := ctx.ImportDir(src, 0)
		if err != nil {
			if _, ok := err.(*build.NoGoError); ok {
				return fmt.Errorf("%s: no Go file for the generic machine", path)
			}
			return err
		}
		dst := filepath.Join(dirA, "stdmirror", filepath.FromSlash(path))
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		for _, f := range bp.GoFiles {
			b, err := os.ReadFile(filepath.Join(src, f))
			if err != nil {
				return err
			}
			text := string(b)
			for _, imp := range bp.Imports {
				if isInternal(imp) {
					text = strings.ReplaceAll(text, `"`+imp+`"`, `"fx/stdmirror/`+imp+`"`)
				}
			}
			// the files were chosen for the machine above: the name and the constraint must not leave them out on this one
			text = buildLineRe.ReplaceAllString(text, "")
			if err := os.WriteFile(filepath.Join(dst, strings.ReplaceAll(f, "_", "-")), []byte(text), 0o644); err != nil {
				return err
			}
		}
		for _, imp := range bp.Imports {
			if isInternal(imp) {
				if err := copyPkg(imp); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := copyPkg(o.Pkg); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dirA, "go.mod"), []byte("module fx\n\ngo "+goVersion()+"\n"), 0o644); err != nil {
		return "", err
	}
	if err := g.write(dirA, o.Pkg, "fx/stdmirror/"+o.Pkg); err != nil {
		return "", err
	}
	return dirA, nil
}

var buildLineRe = regexp.MustCompile(`(?m)^//(?:go:build|\s*\+build) .*\n`)

var internalRe = regexp.MustCompile(`(^|/)internal(/|$)|^vendor/`)

func isInternal(p string) bool { return internalRe.MatchString(p) }
