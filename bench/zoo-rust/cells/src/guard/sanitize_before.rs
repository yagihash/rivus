// @cell(F/guard/sanitize-before)
// @shape: the value is sanitized, and then used (the vulnerable case uses it before; the decoy sanitizes a copy and the original goes on)
use zoo_mark as m;

pub fn register() {
    m::case("F/guard/sanitize-before/vuln", vuln);
    m::case("F/guard/sanitize-before/fixed", fixed);
    m::case("F/guard/sanitize-before/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("guard-sanitize-before-vuln"); // @source(guard-sanitize-before-vuln)
    m::sink_cmd(&v); // @sink(guard-sanitize-before-vuln)
    let v = m::clean_cmd(&v);
    m::sink_cmd(&v); // @sink(none)
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("guard-sanitize-before-fixed"));
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("guard-sanitize-before-decoy"); // @source(guard-sanitize-before-decoy)
    let b = m::clean_cmd(&a);
    m::sink_cmd(&b); // @sink(none) @nosink(guard-sanitize-before-decoy)
    m::sink_sql(&a); // @sink(guard-sanitize-before-decoy)
}
