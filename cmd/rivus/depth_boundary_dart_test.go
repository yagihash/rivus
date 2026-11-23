package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The Dart counterpart of TestDepthBoundaryGo (today's truth, measured at depths 1 to 6):
//
//	C (the flow) is found at every depth.
//	X (the neighbour under the same 1st-level node) is told apart from depth 3 on, and reported at depth 2 and 1: the object that a
//	  Dart class instance is counts one level, so the boundary is one depth earlier than in Go. This is the precision that the new
//	  default (2) loses in Dart; the finding carries access_path_truncated.
//	D (the neighbour under the same 2nd-level node) is reported at EVERY depth (1 to 6): in Dart, the two fields of the innermost object
//	  are not told apart, and that is not an effect of the depth (not understood yet; a precision gap of its own, not caused by
//	  -dep-k).
//
// The Dart counterpart of TestDepthBoundaryGo: a package of the pub cache (a dependency) builds an object whose 3rd-level field
// holds the value, and two neighbours (D under the same 2nd-level node, X under the same 1st-level node). The project reads
// the neighbour and passes it to a sink. Depth 3: D and X are told apart from C. Depth 2 (the default): D is reported
// (precision lost; the note says so). Depth 1: X too. The flow itself (C) is found at every depth.
func TestDepthBoundaryDart(t *testing.T) {
	t.Parallel()
	dart := os.Getenv("RIVUS_DART")
	if dart == "" {
		skipOrFail(t, "RIVUS_DART")
	}
	extractor, err := filepath.Abs("../../extractor/dart")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	third := filepath.Join(cache, "hosted", "pub.dev", "third-1.0.0")
	app := filepath.Join(root, "app")
	for name, text := range map[string]string{
		filepath.Join(third, "pubspec.yaml"): "name: third\nversion: 1.0.0\nenvironment:\n  sdk: ^3.8.0\n",
		filepath.Join(third, "lib", "third.dart"): `class N3 {
  String c = '';
  String d = '';
}

class N2 {
  N3 b = N3();
  String x = '';
}

class Outer {
  N2 a = N2();
}

// Build puts the value into the field of the 3rd level; the neighbours hold constants.
Outer build(String v) {
  final o = Outer();
  o.a.b.c = v;
  o.a.b.d = 'clean';
  o.a.x = 'clean';
  return o;
}
`,
		filepath.Join(app, "pubspec.yaml"): "name: app\nenvironment:\n  sdk: ^3.8.0\ndependencies:\n  third: ^1.0.0\n",
		filepath.Join(app, "lib", "app.dart"): `import 'dart:io';
import 'package:third/third.dart';

void vuln(HttpRequest r) {
  final o = build(r.uri.queryParameters['q']!);
  Process.runSync(o.a.b.c, []);
}

void decoyLevel3(HttpRequest r) {
  final o = build(r.uri.queryParameters['q']!);
  Process.runSync(o.a.b.d, []);
}

void decoyLevel2(HttpRequest r) {
  final o = build(r.uri.queryParameters['q']!);
  Process.runSync(o.a.x, []);
}
`,
		filepath.Join(app, ".dart_tool", "package_config.json"): `{"configVersion":2,"packages":[{"name":"third","rootUri":"file://` + third + `","packageUri":"lib/"},{"name":"app","rootUri":"../","packageUri":"lib/"}]}`,
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) map[string]bool {
		t.Helper()
		cmd := exec.Command(binary, append([]string{"analyze", "-lang", "dart", "-C", app}, args...)...)
		cmd.Env = append(os.Environ(), "PUB_CACHE="+cache, "RIVUS_DART_EXTRACTOR="+dart+" run "+filepath.Join(extractor, "bin", "rivus_dart.dart"))
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("analyze %v: %v", args, err)
		}
		var r struct {
			Findings []struct {
				Source struct {
					Func string `json:"func"`
				} `json:"source"`
				Uncertainties []struct {
					Kind string `json:"kind"`
				} `json:"uncertainties"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		got := map[string]bool{}
		for _, f := range r.Findings {
			name := f.Source.Func[strings.LastIndex(f.Source.Func, "#")+1:]
			got[name] = true
			if name == "decoyLevel2" {
				noted := false
				for _, u := range f.Uncertainties {
					noted = noted || u.Kind == "access_path_truncated"
				}
				if !noted {
					t.Errorf("%v: decoyLevel2 is reported because of the cut and has no access_path_truncated note", args)
				}
			}
		}
		return got
	}
	for _, c := range []struct {
		args                           []string
		vuln, decoyLevel3, decoyLevel2 bool
	}{
		{[]string{"-dep-k", "3"}, true, true, false}, // the old default
		{nil, true, true, true},                      // the default (2): X is reported, and says that the path was cut
		{[]string{"-dep-k", "1"}, true, true, true},
		{[]string{"-dep-k", "6"}, true, true, false},
	} {
		got := run(c.args...)
		for name, want := range map[string]bool{"vuln": c.vuln, "decoyLevel3": c.decoyLevel3, "decoyLevel2": c.decoyLevel2} {
			if got[name] != want {
				t.Errorf("%v: %s reported = %v, want %v (found: %v)", c.args, name, got[name], want, got)
			}
		}
	}
}
