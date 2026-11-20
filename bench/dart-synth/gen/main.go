// Command gen writes a synthetic Dart package whose shape (call depth, groups of
// functions that call each other, closures given to shared helpers) is set by
// parameters. The same parameters always write the same bytes. It needs no
// dependency: pubspec.yaml and .dart_tool/package_config.json are written too, so
// that the extractor can resolve the package without pub and without a network.
// See bench/dart-synth/README.md.
package main

import (
	"fmt"
	"strings"

	"github.com/yagihash/rivus/bench/synthplan"
)

func main() { synthplan.Main("bench/dart-synth/gen", Render) }

const helpers = `// The helpers every function uses: they call the closure that the caller gives.
String viaA(String s, String Function(String) f) => f(s);

// the closure goes on to a second helper
String viaRelay(String s, String Function(String) f) => viaA(s, (x) => f(x));

// the closure is kept in a field and called by a method
class Runner {
  final String Function(String) f;
  Runner(this.f);
  String go(String s) => f(s);
}

// the callback is for its side effect
void fire(String s, void Function(String) f) {
  f(s);
}
`

// Render writes the plan as a Dart package.
func Render(pl *synthplan.Plan) map[string]string {
	files := map[string]string{
		"pubspec.yaml": "name: synth\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n",
		".dart_tool/package_config.json": `{
  "configVersion": 2,
  "packages": [
    {
      "name": "synth",
      "rootUri": "../",
      "packageUri": "lib/",
      "languageVersion": "3.10"
    }
  ]
}
`,
		"rules.yaml": `ruleset: rivus/bench-dart-synth@1

sources:
  - symbol: package:synth/lib.dart#source
    labels: [user_input]

sinks:
  - symbol: package:synth/lib.dart#sink
    category: test
`,
		"lib/lib.dart":     "// The source and the sink of the benchmark.\nString source() => 'x';\nvoid sink(Object? s) {}\n",
		"lib/helpers.dart": helpers,
	}
	var all strings.Builder
	for m, ids := range pl.Modules() {
		var b strings.Builder
		b.WriteString("import 'helpers.dart';\nimport 'lib.dart';\n")
		for _, k := range pl.Imports(m) {
			fmt.Fprintf(&b, "import 'm%d.dart';\n", k)
		}
		for _, id := range ids {
			b.WriteString("\n")
			writeFunc(&b, pl, &pl.Funcs[id])
		}
		files[fmt.Sprintf("lib/m%d.dart", m)] = b.String()
		fmt.Fprintf(&all, "export 'm%d.dart';\n", m)
	}
	files["lib/all.dart"] = all.String()
	return files
}

func writeFunc(b *strings.Builder, pl *synthplan.Plan, f *synthplan.Func) {
	fmt.Fprintf(b, "String f%d(String s) {\n  var v = s;\n", f.ID)
	for i, st := range f.Stmts {
		t := st.Target
		switch st.Kind {
		case synthplan.Call:
			fmt.Fprintf(b, "  v = f%d(v);\n", t)
		case synthplan.Via:
			fmt.Fprintf(b, "  v = viaA(v, (x) => f%d(x));\n", t)
		case synthplan.Relay:
			fmt.Fprintf(b, "  v = viaRelay(v, (x) => f%d(x));\n", t)
		case synthplan.Field:
			fmt.Fprintf(b, "  v = Runner((x) => f%d(x)).go(v);\n", t)
		case synthplan.Hold:
			fmt.Fprintf(b, "  v = viaA(v, (x) => f%d(x + v));\n", t)
		case synthplan.Write:
			fmt.Fprintf(b, "  var w%d = '';\n  fire(v, (x) {\n    w%d = f%d(x);\n  });\n  v = w%d;\n", i, i, t, i)
		case synthplan.Source:
			b.WriteString("  v += source();\n")
		case synthplan.Sink:
			b.WriteString("  sink(v);\n")
		case synthplan.Back:
			fmt.Fprintf(b, "  if (s.length > 100000) {\n    v = f%d(v);\n  }\n", t)
		}
	}
	b.WriteString("  return v;\n}\n")
}
