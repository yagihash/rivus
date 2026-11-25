//go:build !darwin

package main

import (
	"context"
	"fmt"
	"io"
)

// There is no sandbox on this system: a Rust project that is not trusted is not read (nothing is run). A project that is
// trusted is read with -unsafe-host-exec.
func (o *isolationOpts) extractRustSandboxed(ctx context.Context, dir string, extractorArgs []string) ([]byte, error) {
	return nil, fmt.Errorf("the extractor of Rust runs the build scripts and the macros of the project, and the only isolation is the sandbox of macOS, which this system does not have, so nothing was run. If the code is yours or you trust it, -unsafe-host-exec runs it directly on this machine, without any isolation (the build scripts of the project run here, with your rights)")
}

func (o *isolationOpts) extractRustSandboxedTo(ctx context.Context, dir string, extractorArgs []string, w io.Writer) ([]byte, error) {
	return o.extractRustSandboxed(ctx, dir, extractorArgs)
}

// rustExtractorVersionSandboxed: there is nothing to run the extractor in, so it is not run (the version is then "unchecked").
func (o *isolationOpts) rustExtractorVersionSandboxed(ctx context.Context) (string, error) {
	return "", errNoIsolation
}

func sdkRoot() string { return "" }
