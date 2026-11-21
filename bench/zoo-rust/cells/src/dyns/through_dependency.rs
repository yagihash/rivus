// @cell(F/dyn/through-dependency)
// @shape: a value written through fmt::Write goes through a dependency (zoo_dep::wrap, a crate of its own whose body is not read) to a writer of the project, and reaches a sink in its write_str
use std::fmt::{self, Write};
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/through-dependency/vuln", vuln);
    m::case("F/dyn/through-dependency/fixed", fixed);
    m::case("F/dyn/through-dependency/decoy", decoy);
}

struct ShellWriter;

impl Write for ShellWriter {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        m::sink_cmd(s); // @sink(dyn-through-dependency-vuln) @noisy(2) @ref(DYN-IMPLS,PARITY)
        Ok(())
    }
}

fn vuln() { // @vuln
    let mut out = zoo_dep::wrap(ShellWriter);
    out.write_str(&m::src("dyn-through-dependency-vuln")).unwrap(); // @source(dyn-through-dependency-vuln) @ref(PARITY)
}

struct FixedWriter;

impl Write for FixedWriter {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        m::sink_cmd(&m::clean_cmd(s)); // @sink(none)
        Ok(())
    }
}

fn fixed() { // @fixed
    let mut out = zoo_dep::wrap(FixedWriter);
    out.write_str(&m::src("dyn-through-dependency-fixed")).unwrap();
}

struct QuietWriter;

impl Write for QuietWriter {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        m::sink_cmd(s); // @sink(none) @fp @nosink(dyn-through-dependency-decoy) @fp @ref(DYN-IMPLS,PARITY)
        Ok(())
    }
}

struct LoudWriter;

impl Write for LoudWriter {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        m::sink_sql(s); // @sink(dyn-through-dependency-decoy) @noisy(2) @ref(DYN-IMPLS,PARITY)
        Ok(())
    }
}

fn decoy() { // @decoy
    let mut quiet = zoo_dep::wrap(QuietWriter);
    let mut loud = zoo_dep::wrap(LoudWriter);
    quiet.write_str("constant").unwrap();
    loud.write_str(&m::src("dyn-through-dependency-decoy")).unwrap(); // @source(dyn-through-dependency-decoy)
}
