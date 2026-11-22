package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// The version of a release is versionStamp (graphkey.go: `go build -ldflags "-X main.versionStamp=1.2.3"`; it is also
// part of the key of an index). A build from a checkout leaves it empty, and rivus says what the build information of Go
// knows (the module version, or the revision and whether the tree was modified).

// versionString is what `rivus version` prints (one line, so that a script and a formula's test can read it).
func versionString() string {
	v := versionStamp
	extra := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		var rev, modified string
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				modified = s.Value
			}
		}
		if rev != "" {
			if len(rev) > 12 {
				rev = rev[:12]
			}
			extra = " (" + rev
			if modified == "true" {
				extra += ", modified"
			}
			extra += ")"
		}
	}
	if v == "" {
		v = "dev"
	}
	return fmt.Sprintf("rivus %s%s %s/%s %s", strings.TrimPrefix(v, "v"), extra, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
