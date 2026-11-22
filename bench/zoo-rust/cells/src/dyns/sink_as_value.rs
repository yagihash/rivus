// @cell(F/dyn/sink-as-value)
// @shape: a function that a rule names as a sink is made a function value (let f = m::sink_cmd) and the value is called
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/sink-as-value/vuln", vuln);
    m::case("F/dyn/sink-as-value/fixed", fixed);
    m::case("F/dyn/sink-as-value/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("dyn-sink-as-value-vuln"); // @source(dyn-sink-as-value-vuln)
    let f = m::sink_cmd;
    f(&v); // @sink(dyn-sink-as-value-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("dyn-sink-as-value-fixed"));
    let f = m::sink_cmd;
    f(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dyn-sink-as-value-decoy"); // @source(dyn-sink-as-value-decoy)
    let f = m::sink_cmd;
    let g = m::sink_sql;
    f("constant"); // @sink(none) @nosink(dyn-sink-as-value-decoy)
    g(&v); // @sink(dyn-sink-as-value-decoy)
}
