// @cell(F/dyn/iface-write-passed)
// @shape: a Vec<u8> is passed to a function as a &mut dyn Write, written to there with write!, and read from the owner
use std::io::Write;
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/iface-write-passed/vuln", vuln);
    m::case("F/dyn/iface-write-passed/fixed", fixed);
    m::case("F/dyn/iface-write-passed/decoy", decoy);
}

fn fill(w: &mut dyn Write, s: &str) {
    write!(w, "{}", s).unwrap();
}

fn vuln() { // @vuln
    let mut b: Vec<u8> = Vec::new();
    fill(&mut b, &m::src("dyn-iface-write-passed-vuln")); // @source(dyn-iface-write-passed-vuln)
    m::sink_cmd_bytes(&b); // @sink(dyn-iface-write-passed-vuln)
}

fn fixed() { // @fixed
    let mut b: Vec<u8> = Vec::new();
    fill(&mut b, &m::src("dyn-iface-write-passed-fixed"));
    m::sink_cmd(&m::clean_cmd(&String::from_utf8_lossy(&b))); // @sink(none)
}

fn decoy() { // @decoy
    let mut b: Vec<u8> = Vec::new();
    let mut other: Vec<u8> = Vec::new();
    fill(&mut b, &m::src("dyn-iface-write-passed-decoy")); // @source(dyn-iface-write-passed-decoy)
    m::sink_cmd_bytes(&other); // @sink(none) @nosink(dyn-iface-write-passed-decoy)
    m::sink_sql(&String::from_utf8_lossy(&b)); // @sink(dyn-iface-write-passed-decoy)
    other.clear();
}
