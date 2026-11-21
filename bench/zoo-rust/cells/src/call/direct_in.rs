// @cell(F/call/direct-in)
// @shape: the source is the type of a parameter of the handler (m::In), and the handler passes its text to a sink
use zoo_mark as m;

pub fn register() {
    m::case("F/call/direct-in/vuln", vuln);
    m::case("F/call/direct-in/fixed", fixed);
    m::case("F/call/direct-in/decoy", decoy);
}

fn handle(i: m::In) { // @source(call-direct-in-vuln)
    m::sink_cmd(&i.value); // @sink(call-direct-in-vuln)
}

fn vuln() { // @vuln
    handle(m::In::new("call-direct-in-vuln"));
}

fn handle_fixed(i: m::In) {
    m::sink_cmd(&m::clean_cmd(&i.value)); // @sink(none)
}

fn fixed() { // @fixed
    handle_fixed(m::In::new("call-direct-in-fixed"));
}

fn handle_decoy(i: m::In, other: &str) { // @source(call-direct-in-decoy)
    m::sink_cmd(other); // @sink(none) @nosink(call-direct-in-decoy)
    m::sink_cmd(&i.value); // @sink(call-direct-in-decoy)
}

fn decoy() { // @decoy
    handle_decoy(m::In::new("call-direct-in-decoy"), "constant");
}
