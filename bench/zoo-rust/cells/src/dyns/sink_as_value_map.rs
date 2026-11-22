// @cell(F/dyn/sink-as-value-map)
// @shape: a function that a rule names as a sink is made a function value; the function value is stored in a map and called by a constant key
// (the map keeps the type of the function item: a function pointer of a #[track_caller] function loses the line of the call)
use std::collections::HashMap;
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/sink-as-value-map/vuln", vuln);
    m::case("F/dyn/sink-as-value-map/fixed", fixed);
    m::case("F/dyn/sink-as-value-map/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("dyn-sink-as-value-map-vuln"); // @source(dyn-sink-as-value-map-vuln)
    let mut t = HashMap::new();
    t.insert("cmd", m::sink_cmd);
    (t["cmd"])(&v); // @sink(dyn-sink-as-value-map-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("dyn-sink-as-value-map-fixed"));
    let mut t = HashMap::new();
    t.insert("cmd", m::sink_cmd);
    (t["cmd"])(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dyn-sink-as-value-map-decoy"); // @source(dyn-sink-as-value-map-decoy)
    let mut tc = HashMap::new();
    tc.insert("cmd", m::sink_cmd);
    let mut ts = HashMap::new();
    ts.insert("sql", m::sink_sql);
    (tc["cmd"])("constant"); // @sink(none) @nosink(dyn-sink-as-value-map-decoy)
    (ts["sql"])(&v); // @sink(dyn-sink-as-value-map-decoy)
}
