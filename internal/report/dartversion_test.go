package report

import "testing"

func TestCheckDartVersion(t *testing.T) {
	cases := []struct {
		name, constraint, match, minimum string
	}{
		{"a range with the same minor", ">=3.13.0 <4.0.0", DartYes, "3.13.0"},
		{"a range with an older minor", ">=3.0.0 <4.0.0", DartYes, "3.0.0"},
		{"a caret, older", "^3.0.0", DartYes, "3.0.0"},
		{"a caret, the same minor, newer patch", "^3.13.9", DartYes, "3.13.9"},
		{"a caret, newer minor", "^3.14.0", DartNewer, "3.14.0"},
		{"a range, newer minor", ">=3.20.0 <4.0.0", DartNewer, "3.20.0"},
		{"newer major", "^4.0.0", DartNewer, "4.0.0"},
		{"older major", "^2.19.0", DartYes, "2.19.0"},
		{"strictly greater than an older one", ">3.0.0", DartYes, "3.0.0"},
		{"a bare version, newer", "3.99.0", DartNewer, "3.99.0"},
		{"two lower bounds, the higher counts", ">=3.0.0 >=3.50.0 <4.0.0", DartNewer, "3.50.0"},
		{"a pre-release", ">=3.14.0-0 <4.0.0", DartNewer, "3.14.0-0"},
		{"a two-part version", ">=3.14 <4.0.0", DartNewer, "3.14"},
		{"an upper bound that is newer says nothing", ">=3.0.0 <9.0.0", DartYes, "3.0.0"},
		{"only an upper bound", "<4.0.0", DartUnknown, ""},
		{"any", "any", DartUnknown, ""},
		{"empty (no constraint)", "", DartUnknown, ""},
		{"not a constraint", "next", DartUnknown, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := CheckDartVersion("3.13", c.constraint)
			if d.Match != c.match || d.Minimum != c.minimum || d.Target != c.constraint || d.Verified != "3.13" {
				t.Errorf("got %+v", d)
			}
		})
	}
	if d := CheckDartVersion("x", "^3.0.0"); d.Match != DartUnknown {
		t.Errorf("a verified version that cannot be read: %+v", d)
	}
}
