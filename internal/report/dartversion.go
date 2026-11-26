package report

import "regexp"

// dartTerm is one term of an SDK constraint of a pubspec.yaml: an operator (none, "=", "^", ">=", ">", "<=", "<") and a version.
var dartTerm = regexp.MustCompile(`(\^|>=|>|<=|<|=)?\s*v?(\d+\.\d+(?:\.\d+)?(?:[-+][0-9A-Za-z.+-]*)?)`)

// DartSDKMinimum is the lowest SDK that an SDK constraint of a pubspec.yaml allows ("^3.0.0" and ">=3.0.0 <4.0.0" say 3.0.0; a bare
// version is that version). The language version of a package is this one, so it is what is compared with the verified one; an upper bound
// says nothing about the language. When several terms give a lower bound, the highest counts. False for "any", an empty constraint, a
// constraint that has only an upper bound, and text that is not a constraint.
func DartSDKMinimum(constraint string) (string, bool) {
	min, minMaj, minMin, found := "", 0, 0, false
	for _, m := range dartTerm.FindAllStringSubmatch(constraint, -1) {
		if m[1] == "<" || m[1] == "<=" {
			continue
		}
		maj, mnr, ok := goMinorOf(m[2])
		if !ok {
			continue
		}
		if !found || maj > minMaj || (maj == minMaj && mnr > minMin) {
			min, minMaj, minMin, found = m[2], maj, mnr, true
		}
	}
	return min, found
}

// CheckDartVersion compares the lowest SDK that the sdk constraint of the analysed package allows with verified (the minor of the Dart SDK that
// this rivus was verified with), to the minor. "newer" when the package needs a newer language than the one the extractor was checked against;
// "unknown" when there is no constraint, or none that gives a lower bound. Only the analysed package is read: the pubspec.yaml of its
// dependencies is not (unlike Go, where the loader gives the go line of each module).
func CheckDartVersion(verified, constraint string) *DartVersion {
	d := &DartVersion{Target: constraint, Verified: verified, Match: DartUnknown}
	vmaj, vmin, ok := goMinorOf(verified)
	if !ok {
		return d
	}
	min, ok := DartSDKMinimum(constraint)
	if !ok {
		return d
	}
	d.Minimum = min
	maj, mnr, _ := goMinorOf(min)
	d.Match = DartYes
	if maj > vmaj || (maj == vmaj && mnr > vmin) {
		d.Match = DartNewer
	}
	return d
}
