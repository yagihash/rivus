// @cell(F/cont/map-key-vs-value)
// @shape: a value is stored under one key of a HashMap, and the decoy reads the other key
use std::collections::HashMap;
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/map-key-vs-value/vuln", vuln);
    m::case("F/cont/map-key-vs-value/fixed", fixed);
    m::case("F/cont/map-key-vs-value/decoy", decoy);
}

fn vuln() { // @vuln
    let mut h = HashMap::new();
    h.insert("a", m::src("cont-map-key-vs-value-vuln")); // @source(cont-map-key-vs-value-vuln)
    h.insert("b", "constant".to_string());
    m::sink_cmd(&h["a"]); // @sink(cont-map-key-vs-value-vuln)
}

fn fixed() { // @fixed
    let mut h = HashMap::new();
    h.insert("a", m::clean_cmd(&m::src("cont-map-key-vs-value-fixed")));
    h.insert("b", "constant".to_string());
    m::sink_cmd(&h["a"]); // @sink(none)
}

fn decoy() { // @decoy
    let mut h = HashMap::new();
    h.insert("a", m::src("cont-map-key-vs-value-decoy")); // @source(cont-map-key-vs-value-decoy)
    h.insert("b", "constant".to_string());
    m::sink_cmd(&h["b"]); // @sink(none) @fp @nosink(cont-map-key-vs-value-decoy) @fp @ref(KEYED-MAP,PARITY)
    m::sink_sql(&h["a"]); // @sink(cont-map-key-vs-value-decoy)
}
