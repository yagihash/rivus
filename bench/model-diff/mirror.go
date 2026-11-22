package main

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/tools/go/packages"
)

// mirrorStd makes the code that the bodies are read from for a package of the standard library. rivus does not read the standard library
// (the models stand for it, ADR-015), so the files of the package are copied into the module as a package of its own
// (`stdmirror/<path>`) and the same calls are made to the copy: the models of the package do not apply to it, those of the others do.
// The `internal/...` packages that it imports are copied the same way (a package of another module may not import them), and the
// files are chosen for a machine that has no assembly, so the Go version of each function is the one that is read. The assembly
// files are not copied (a function that has no body there is read as a function without a body). A package that cannot be copied
// makes its rows "unverified".
//
// The files are chosen for `wasm` first (few files per package). A package that has a file for the architecture only (`os`:
// `executable_wasm.go` next to `executable_darwin.go`) then declares a name twice and does not build: the files are chosen again for the
// architecture of this machine (a function that has its body in assembly is then read as one without a body).
func mirrorStd(o GoOptions, g *goGen, dirB string) (string, error) {
	var last error
	for _, arch := range []string{"wasm", runtime.GOARCH} {
		dirA, err := mirrorStdFor(o, g, dirB, arch)
		if err != nil {
			return "", err
		}
		if last = mirrorErrors(dirA); last == nil {
			return dirA, nil
		}
	}
	return "", fmt.Errorf("the copy of %s does not build for wasm nor for %s: %v", o.Pkg, runtime.GOARCH, last)
}

// mirrorErrors type-checks the copy and returns the first error that is not a function without a body (the assembly is not copied).
func mirrorErrors(dir string) error {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax | packages.NeedImports | packages.NeedDeps | packages.NeedTypesInfo, Dir: dir,
		Env: append(os.Environ(), "GOFLAGS=-mod=mod")}
	ps, err := packages.Load(cfg, "./...")
	if err != nil {
		return err
	}
	var first error
	packages.Visit(ps, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			if first == nil && !strings.Contains(e.Msg, "missing function body") {
				first = fmt.Errorf("%s: %v", p.PkgPath, e)
			}
		}
	})
	return first
}

func mirrorStdFor(o GoOptions, g *goGen, dirB, arch string) (string, error) {
	dirA := filepath.Join(filepath.Dir(dirB), "fxA")
	if err := os.RemoveAll(dirA); err != nil {
		return "", err
	}
	ctx := build.Default
	ctx.GOARCH = arch
	ctx.CgoEnabled = false
	ctx.ToolTags = nil // no goexperiment: the register ABI is not for this machine
	m := &mirror{ctx: ctx, dir: dirA, done: map[string]bool{}}
	if err := m.copyPkg(o.Pkg); err != nil {
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

type mirror struct {
	ctx  build.Context
	dir  string
	done map[string]bool
}

// copyPkg copies a package of GOROOT and, the same way, the internal packages that it imports.
func (m *mirror) copyPkg(path string) error {
	if m.done[path] {
		return nil
	}
	m.done[path] = true
	src := filepath.Join(runtime.GOROOT(), "src", path)
	bp, err := m.ctx.ImportDir(src, 0)
	if err != nil {
		if _, ok := err.(*build.NoGoError); ok {
			return fmt.Errorf("%s: no Go file for the machine", path)
		}
		return err
	}
	dst := filepath.Join(m.dir, "stdmirror", filepath.FromSlash(path))
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
			if err := m.copyPkg(imp); err != nil {
				return err
			}
		}
	}
	return nil
}

var buildLineRe = regexp.MustCompile(`(?m)^//(?:go:build|\s*\+build) .*\n`)

var internalRe = regexp.MustCompile(`(^|/)internal(/|$)|^vendor/`)

func isInternal(p string) bool { return internalRe.MatchString(p) }
