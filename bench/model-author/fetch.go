package main

import (
	"fmt"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Fetching and reading a package. Only the source is fetched (`go get` in a directory of its own that has nothing but a go.mod), and
// it is read as source by `go list` (no `-export`, no `-compiled`: nothing of the module is compiled, and no generator, cgo or code
// of the module runs). cgo is off, `GOTOOLCHAIN=local` keeps the toolchain from fetching another one, and no `go run`, `go test` or
// `go build` is made on what was fetched.

// loaded is a package read from source.
type loaded struct {
	Types   *types.Package
	Module  string // the module path ("" for the standard library)
	Version string // the version of the module ("" for the standard library)
}

func isStdlib(pkg string) bool {
	return !strings.Contains(strings.SplitN(pkg, "/", 2)[0], ".")
}

func goEnv() []string {
	return append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOWORK=off")
}

// FetchAndLoad fetches pkg@version into dir (not for the standard library) and reads its types.
func FetchAndLoad(dir, pkg, version string) (*loaded, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, &exitError{ExitFetch, err}
	}
	gv, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		return nil, &exitError{ExitFetch, err}
	}
	gomod := "module fx\n\ngo " + strings.TrimPrefix(strings.TrimSpace(string(gv)), "go") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		return nil, &exitError{ExitFetch, err}
	}
	if !isStdlib(pkg) {
		at := pkg
		if version != "" {
			at += "@" + version
		} else {
			at += "@latest"
		}
		c := exec.Command("go", "get", at)
		c.Dir = dir
		c.Env = goEnv()
		if out, err := c.CombinedOutput(); err != nil {
			return nil, &exitError{ExitFetch, fmt.Errorf("go get %s: %v\n%s", at, err, out)}
		}
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:  dir,
		Env:  goEnv(),
	}
	pkgs, err := packages.Load(cfg, pkg)
	if err != nil {
		return nil, &exitError{ExitLoad, err}
	}
	if len(pkgs) != 1 || pkgs[0].Types == nil {
		return nil, &exitError{ExitLoad, fmt.Errorf("cannot load %s", pkg)}
	}
	if len(pkgs[0].Errors) > 0 {
		return nil, &exitError{ExitLoad, fmt.Errorf("cannot read %s: %v", pkg, pkgs[0].Errors[0])}
	}
	l := &loaded{Types: pkgs[0].Types}
	if m := pkgs[0].Module; m != nil {
		l.Module, l.Version = m.Path, m.Version
	}
	return l, nil
}
