package main

import (
	"fmt"
	"strings"
)

// cltDir is the Command Line Tools: the one developer directory that the sandbox lets a build read (the profile of
// extractRustSandboxed names it), unlike Xcode.app.
const cltDir = "/Library/Developer/CommandLineTools"

// developerDirFacts is what decideDeveloperDir looks at: nothing is read from the machine inside it, so a test can fake all of it.
type developerDirFacts struct {
	goos string
	// userEnv are the NAME=VALUE pairs of -isolation-env (nothing else is passed to the build)
	userEnv []string
	// cltExists is whether cltDir is a directory; selected is `xcode-select -p` ("" when it could not be asked)
	cltExists bool
	selected  string
}

// decideDeveloperDir says which DEVELOPER_DIR the build gets. The linker step of a build script runs xcrun, which reads the developer
// directory; under the sandbox it cannot read Xcode.app, so with Xcode.app selected the link fails. Rules, in order:
//  1. not macOS: nothing is set (value "").
//  2. -isolation-env has DEVELOPER_DIR: it is used as it is (source "user"; it is already in the environment, nothing is added).
//  3. the Command Line Tools exist: DEVELOPER_DIR is set to them (source "default").
//  4. they do not exist and the selected developer directory is inside an .app (Xcode.app): an error, before anything runs.
//  5. otherwise nothing is set (a selected directory that is not an .app is left to xcode-select).
func decideDeveloperDir(f developerDirFacts) (value, source string, err error) {
	if f.goos != "darwin" {
		return "", "", nil
	}
	for _, kv := range f.userEnv {
		if v, ok := strings.CutPrefix(kv, "DEVELOPER_DIR="); ok {
			return v, "user", nil
		}
	}
	if f.cltExists {
		return cltDir, "default", nil
	}
	if strings.Contains(f.selected, ".app/") || strings.HasSuffix(f.selected, ".app") {
		return "", "", fmt.Errorf("the developer directory that xcode-select has selected is %s, which the sandbox does not let a build read (a build script links with xcrun, which reads it), and the Command Line Tools (%s) are not installed, so the build would fail at the link step. Nothing was run. Install them with `xcode-select --install`, or give a directory that the build may read with -isolation-env DEVELOPER_DIR=<dir> (and -isolation-read <dir>)", f.selected, cltDir)
	}
	return "", "", nil
}
