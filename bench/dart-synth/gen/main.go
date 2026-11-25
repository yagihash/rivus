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

const rules = `ruleset: rivus/bench-dart-synth@1

sources:
  - symbol: package:synth/lib.dart#source
    labels: [user_input]

sinks:
  - symbol: package:synth/lib.dart#sink
    category: test
`

// packageConfig names the project, and, for size F, the dependency package. The dependency is under the hidden
// directory .dart_tool, so that it is not read as code of the project but as a dependency, when the code imports it
// (the same as a package of the pub cache: its functions are Dep).
func packageConfig(dep bool) string {
	d := ""
	if dep {
		d = `,
    {
      "name": "dep",
      "rootUri": "dep/",
      "packageUri": "lib/",
      "languageVersion": "3.10"
    }`
	}
	return `{
  "configVersion": 2,
  "packages": [
    {
      "name": "synth",
      "rootUri": "../",
      "packageUri": "lib/",
      "languageVersion": "3.10"
    }` + d + `
  ]
}
`
}

// depPackage writes the dependency of size F: n functions of each of three kinds that take a function value and give it
// on, the way the generated code of a real project meets a library. d: the argument is a function that returns a function
// (the project gives `(x) => (y) => f(y + x)`); d0 calls it, dI gives it on as it is to d(I-1) and, wrapped in a closure,
// to d(I/2): two paths to the bottom. w, p, b: the value is returned in a closure, as it is, or kept in the field of an object.
func depPackage(n int) map[string]string {
	var b strings.Builder
	b.WriteString("typedef Fn = String Function(String);\n\nString d0(String s, Fn Function(String) f) => f(s)(s);\n")
	for i := 1; i < n; i++ {
		fmt.Fprintf(&b, "\nString d%d(String s, Fn Function(String) f) {\n  var v = d%d(s, f);\n  v = d%d(v, (x) => f(x));\n  return v;\n}\n", i, i-1, i/2)
	}
	b.WriteString("\nString Function(String) w0(String Function(String) f) => (x) => f(x);\n")
	for i := 1; i < n; i++ {
		fmt.Fprintf(&b, "\nString Function(String) w%d(String Function(String) f) => w%d((x) => f(x));\n", i, i-1)
	}
	// p: the function value is given back as it is; b: it is kept in a field of the object that is given back
	b.WriteString("\nString Function(String) p0(String Function(String) f) => f;\n")
	for i := 1; i < n; i++ {
		fmt.Fprintf(&b, "\nString Function(String) p%d(String Function(String) f) => p%d(f);\n", i, i-1)
	}
	b.WriteString("\nclass Box {\n  final String Function(String) f;\n  Box(this.f);\n  String go(String s) => f(s);\n}\n\nBox b0(String Function(String) f) => Box(f);\n")
	for i := 1; i < n; i++ {
		fmt.Fprintf(&b, "\nBox b%d(String Function(String) f) => b%d((x) => f(x));\n", i, i-1)
	}
	return map[string]string{
		".dart_tool/dep/pubspec.yaml": "name: dep\nversion: 1.0.0\nenvironment:\n  sdk: ^3.10.0\n",
		".dart_tool/dep/lib/dep.dart": b.String(),
	}
}

// Render writes the plan as a Dart package.
func Render(pl *synthplan.Plan) map[string]string {
	files := map[string]string{
		"pubspec.yaml":                   "name: synth\npublish_to: none\nenvironment:\n  sdk: ^3.10.0\n",
		".dart_tool/package_config.json": packageConfig(pl.P.DepPct > 0),
		"rules.yaml":                     rules,
		"lib/lib.dart":                   "// The source and the sink of the benchmark.\nString source() => 'x';\nvoid sink(Object? s) {}\n",
		"lib/helpers.dart":               helpers,
	}
	if pl.P.DepPct > 0 {
		for k, v := range depPackage(pl.P.DepFuncs) {
			files[k] = v
		}
	}
	imports := ""
	if pl.P.DepPct > 0 {
		imports = "import 'package:dep/dep.dart' as dep;\n"
	}
	var all strings.Builder
	for m, ids := range pl.Modules() {
		var b strings.Builder
		b.WriteString("import 'helpers.dart';\nimport 'lib.dart';\n" + imports)
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
		case synthplan.DepRun:
			fmt.Fprintf(b, "  v = dep.d%d(v, (x) => (y) => f%d(y + x));\n", st.Dep, t)
		case synthplan.DepWrap:
			switch st.Dep % 3 {
			case 0:
				fmt.Fprintf(b, "  v = dep.w%d((x) => f%d(x))(v);\n", st.Dep, t)
			case 1:
				fmt.Fprintf(b, "  v = dep.p%d((x) => f%d(x))(v);\n", st.Dep, t)
			default:
				fmt.Fprintf(b, "  v = dep.b%d((x) => f%d(x)).go(v);\n", st.Dep, t)
			}
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
