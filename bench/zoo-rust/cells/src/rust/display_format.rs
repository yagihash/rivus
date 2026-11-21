// @cell(F/rust/display-format)
// @shape: the value goes through format! (positional and inline arguments) and through a Display impl (to_string())
use std::fmt;
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/display-format/vuln", vuln);
    m::case("F/rust/display-format/fixed", fixed);
    m::case("F/rust/display-format/decoy", decoy);
    m::case("F/rust/display-format/vuln2", vuln2);
    m::case("F/rust/display-format/vuln3", vuln3);
}

fn vuln() { // @vuln
    let v = m::src("rust-display-format-vuln"); // @source(rust-display-format-vuln)
    let s = format!("run {}", v);
    m::sink_cmd(&s); // @sink(rust-display-format-vuln)
}

fn vuln2() { // @vuln
    let v = m::src("rust-display-format-vuln2"); // @source(rust-display-format-vuln2)
    let s = format!("run {v}");
    m::sink_cmd(&s); // @sink(rust-display-format-vuln2)
}

struct Wrapper(String);

impl fmt::Display for Wrapper {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "x{}", self.0)
    }
}

fn vuln3() { // @vuln
    let w = Wrapper(m::src("rust-display-format-vuln3")); // @source(rust-display-format-vuln3)
    m::sink_cmd(&w.to_string()); // @sink(rust-display-format-vuln3)
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("rust-display-format-fixed"));
    let s = format!("run {}", v);
    m::sink_cmd(&s); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("rust-display-format-decoy"); // @source(rust-display-format-decoy)
    let a = format!("run {}", "constant");
    let b = format!("run {}", v);
    m::sink_cmd(&a); // @sink(none) @nosink(rust-display-format-decoy)
    m::sink_sql(&b); // @sink(rust-display-format-decoy)
}
