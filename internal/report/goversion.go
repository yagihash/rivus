package report

import (
	"regexp"
	"strconv"
)

var goMinor = regexp.MustCompile(`^(\d+)\.(\d+)`)

// goMinorOf reads the language version of a go line ("1.27", "1.27.4", "1.27rc1") as major and minor.
func goMinorOf(v string) (major, minor int, ok bool) {
	m := goMinor.FindStringSubmatch(v)
	if m == nil {
		return 0, 0, false
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	return major, minor, true
}

// CheckGoVersion compares the go lines of the modules that were read (mods, by module path; the target is mainModule) with verified, to the
// minor. The newest of all the modules decides: a dependency written for a newer Go counts as the target does. A target without a go line is
// "unknown". Of equal go lines, the target is named, then the smaller module path.
func CheckGoVersion(verified, mainModule string, mods map[string]string) *GoVersion {
	g := &GoVersion{Target: mods[mainModule], Verified: verified, Match: GoUnknown}
	if _, _, ok := goMinorOf(g.Target); !ok {
		return g
	}
	vmaj, vmin, ok := goMinorOf(verified)
	if !ok {
		return g
	}
	g.Match = GoYes
	newest, newestMaj, newestMin := "", vmaj, vmin
	for path, v := range mods {
		maj, min, ok := goMinorOf(v)
		if !ok || maj < newestMaj || (maj == newestMaj && min <= newestMin) {
			if ok && newest != "" && maj == newestMaj && min == newestMin && prefer(path, newest, mainModule) {
				newest = path
			}
			continue
		}
		newest, newestMaj, newestMin = path, maj, min
	}
	if newest != "" {
		g.Match, g.NewestModule, g.NewestVersion = GoNewer, newest, mods[newest]
	}
	return g
}

// prefer says whether path is named before other when their go lines are equal.
func prefer(path, other, mainModule string) bool {
	if path == mainModule {
		return true
	}
	return other != mainModule && path < other
}
