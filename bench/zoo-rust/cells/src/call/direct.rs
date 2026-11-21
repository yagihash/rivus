// @cell(F/call/direct)
// @shape: a value of a source goes to a sink in the same function
use zoo_mark as m;

pub fn register() {
    m::case("F/call/direct/vuln", vuln);
    m::case("F/call/direct/fixed", fixed);
    m::case("F/call/direct/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("call-direct-vuln"); // @source(call-direct-vuln)
    m::sink_cmd(&v); // @sink(call-direct-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("call-direct-fixed"));
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("call-direct-decoy"); // @source(call-direct-decoy)
    m::sink_cmd("constant"); // @sink(none) @nosink(call-direct-decoy)
    m::sink_cmd(&t); // @sink(call-direct-decoy)
}
