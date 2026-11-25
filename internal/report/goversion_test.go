package report

import "testing"

func TestCheckGoVersion(t *testing.T) {
	const app = "example.com/app"
	cases := []struct {
		name    string
		main    string
		mods    map[string]string
		match   string
		newestM string
		newestV string
		target  string
	}{
		{"same minor", app, map[string]string{app: "1.27"}, GoYes, "", "", "1.27"},
		{"same minor, newer patch", app, map[string]string{app: "1.27.9"}, GoYes, "", "", "1.27.9"},
		{"older minor", app, map[string]string{app: "1.20"}, GoYes, "", "", "1.20"},
		{"target newer", app, map[string]string{app: "1.99", "example.com/d": "1.20"}, GoNewer, app, "1.99", "1.99"},
		{"only a dependency newer", app, map[string]string{app: "1.27", "example.com/d": "1.28.2"}, GoNewer, "example.com/d", "1.28.2", "1.27"},
		{"the newest of two dependencies", app, map[string]string{app: "1.27", "example.com/a": "1.28", "example.com/b": "1.30"}, GoNewer, "example.com/b", "1.30", "1.27"},
		{"no go line in the target", app, map[string]string{"example.com/d": "1.99"}, GoUnknown, "", "", ""},
		{"no modules (no go.mod)", "", nil, GoUnknown, "", "", ""},
		{"a major that is newer", app, map[string]string{app: "2.0"}, GoNewer, app, "2.0", "2.0"},
		{"a release candidate of the same minor", app, map[string]string{app: "1.27rc1"}, GoYes, "", "", "1.27rc1"},
		{"not a version", app, map[string]string{app: "next"}, GoUnknown, "", "", "next"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := CheckGoVersion("1.27", c.main, c.mods)
			if g.Match != c.match || g.NewestModule != c.newestM || g.NewestVersion != c.newestV || g.Target != c.target || g.Verified != "1.27" {
				t.Errorf("got %+v", g)
			}
		})
	}
}
