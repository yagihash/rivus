// @cell(F/dyn/xtalk-n-by-m)
// @shape: several handlers take the same parameter type (a source), and each has its own sink
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/xtalk-n-by-m/vuln", vuln);
    m::case("F/dyn/xtalk-n-by-m/fixed", fixed);
    m::case("F/dyn/xtalk-n-by-m/decoy", decoy);
}

fn handler(i: m::In) { // @source(dyn-xtalk-n-by-m-vuln)
    m::sink_cmd(&i.value); // @sink(dyn-xtalk-n-by-m-vuln)
}

fn vuln() { // @vuln
    handler(m::In::new("dyn-xtalk-n-by-m-vuln"));
}

fn handler_fixed(i: m::In) {
    m::sink_cmd(&m::clean_cmd(&i.value)); // @sink(none)
}

fn fixed() { // @fixed
    handler_fixed(m::In::new("dyn-xtalk-n-by-m-fixed"));
}

// three handlers of one parameter type: each source reaches its own sink and no other
fn handler_a(i: m::In) { // @source(dyn-xtalk-n-by-m-decoy)
    m::sink_cmd(&i.value); // @sink(dyn-xtalk-n-by-m-decoy)
}

fn handler_b(i: m::In) { // @source(dyn-xtalk-n-by-m-decoy-b)
    m::sink_sql(&i.value); // @sink(dyn-xtalk-n-by-m-decoy-b)
}

fn handler_c(_i: m::In) {
    m::sink_cmd("constant"); // @sink(none) @nosink(dyn-xtalk-n-by-m-decoy) @nosink(dyn-xtalk-n-by-m-decoy-b)
}

fn decoy() { // @decoy
    handler_a(m::In::new("dyn-xtalk-n-by-m-decoy"));
    handler_b(m::In::new("dyn-xtalk-n-by-m-decoy-b"));
    handler_c(m::In::new("dyn-xtalk-n-by-m-decoy-c"));
}
