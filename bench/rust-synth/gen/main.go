// Command gen writes a synthetic Rust crate whose shape (call depth, groups of
// functions that call each other, closures given to shared helpers) is set by
// parameters. The same parameters always write the same bytes. The crate has no
// dependency, so that `cargo check` runs without a network. See bench/rust-synth/README.md.
package main

import (
	"fmt"
	"strings"

	"github.com/yagihash/rivus/bench/synthplan"
)

func main() { synthplan.Main("bench/rust-synth/gen", Render) }

const helpers = `// The helpers every function uses: they call the closure that the caller gives.
pub fn via_a(s: String, f: impl Fn(String) -> String) -> String {
    f(s)
}

// the closure goes on to a second helper
pub fn via_relay(s: String, f: impl Fn(String) -> String) -> String {
    via_a(s, |x| f(x))
}

// the closure is kept in a field and called by a method
pub struct Runner<F: Fn(String) -> String> {
    f: F,
}

impl<F: Fn(String) -> String> Runner<F> {
    pub fn new(f: F) -> Runner<F> {
        Runner { f }
    }
    pub fn go(&self, s: String) -> String {
        (self.f)(s)
    }
}

// the callback is for its side effect
pub fn fire(s: String, mut f: impl FnMut(String)) {
    f(s);
}
`

// Render writes the plan as a Rust crate.
func Render(pl *synthplan.Plan) map[string]string {
	files := map[string]string{
		"Cargo.toml": "[package]\nname = \"synth\"\nversion = \"0.0.0\"\nedition = \"2021\"\n\n[workspace]\n",
		"rules.yaml": `ruleset: rivus/bench-rust-synth@1

sources:
  - symbol: synth::source
    labels: [user_input]

sinks:
  - symbol: synth::sink
    category: test
    accepts: [user_input]
`,
		"src/helpers.rs": helpers,
	}
	var lib strings.Builder
	lib.WriteString("pub mod helpers;\n")
	for m, ids := range pl.Modules() {
		var b strings.Builder
		b.WriteString("#![allow(unused_imports, unused_variables, unused_mut, unused_assignments)]\n")
		b.WriteString("use crate::helpers::*;\nuse crate::{sink, source};\n")
		for _, k := range pl.Imports(m) {
			fmt.Fprintf(&b, "use crate::m%d::*;\n", k)
		}
		for _, id := range ids {
			b.WriteString("\n")
			writeFunc(&b, pl, &pl.Funcs[id])
		}
		files[fmt.Sprintf("src/m%d.rs", m)] = b.String()
		fmt.Fprintf(&lib, "pub mod m%d;\n", m)
	}
	lib.WriteString("\npub fn source() -> String {\n    String::from(\"x\")\n}\n\npub fn sink(_s: &str) {}\n")
	files["src/lib.rs"] = lib.String()
	return files
}

func writeFunc(b *strings.Builder, pl *synthplan.Plan, f *synthplan.Func) {
	fmt.Fprintf(b, "pub fn f%d(s: String) -> String {\n    let mut v = s.clone();\n", f.ID)
	for i, st := range f.Stmts {
		t := st.Target
		switch st.Kind {
		case synthplan.Call:
			fmt.Fprintf(b, "    v = f%d(v);\n", t)
		case synthplan.Via:
			fmt.Fprintf(b, "    v = via_a(v, |x| f%d(x));\n", t)
		case synthplan.Relay:
			fmt.Fprintf(b, "    v = via_relay(v, |x| f%d(x));\n", t)
		case synthplan.Field:
			fmt.Fprintf(b, "    v = Runner::new(|x| f%d(x)).go(v);\n", t)
		case synthplan.Hold:
			fmt.Fprintf(b, "    v = via_a(v.clone(), |x| f%d(format!(\"{}{}\", x, v)));\n", t)
		case synthplan.Write:
			fmt.Fprintf(b, "    let mut w%d = String::new();\n    fire(v.clone(), |x| w%d = f%d(x));\n    v = w%d;\n", i, i, t, i)
		case synthplan.Source:
			b.WriteString("    v.push_str(&source());\n")
		case synthplan.Sink:
			b.WriteString("    sink(&v);\n")
		case synthplan.Back:
			fmt.Fprintf(b, "    if s.len() > 100000 {\n        v = f%d(v);\n    }\n", t)
		}
	}
	b.WriteString("    v\n}\n")
}
