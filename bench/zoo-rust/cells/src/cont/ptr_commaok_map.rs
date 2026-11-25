// @cell(F/cont/ptr-commaok-map)
// @shape: a `&mut` taken out of a map by `get_mut`, and a field written through it, is read from the map again
use std::collections::HashMap;
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-commaok-map/vuln", vuln);
    m::case("F/cont/ptr-commaok-map/fixed", fixed);
    m::case("F/cont/ptr-commaok-map/decoy", decoy);
}

struct Boxed {
    v: String,
}

fn table() -> HashMap<String, Boxed> {
    let mut t = HashMap::new();
    t.insert("a".to_string(), Boxed { v: String::new() });
    t
}

fn vuln() { // @vuln
    let mut t = table();
    let p = t.get_mut("a").unwrap();
    p.v = m::src("cont-ptr-commaok-map-vuln"); // @source(cont-ptr-commaok-map-vuln)
    m::sink_cmd(&t["a"].v); // @sink(cont-ptr-commaok-map-vuln)
}

fn fixed() { // @fixed
    let mut t = table();
    let p = t.get_mut("a").unwrap();
    p.v = m::clean_cmd(&m::src("cont-ptr-commaok-map-fixed"));
    m::sink_cmd(&t["a"].v); // @sink(none)
}

fn decoy() { // @decoy
    let mut t = table();
    let u = table();
    let p = t.get_mut("a").unwrap();
    p.v = m::src("cont-ptr-commaok-map-decoy"); // @source(cont-ptr-commaok-map-decoy)
    m::sink_cmd(&u["a"].v); // @sink(none)
    m::sink_sql(&t["a"].v); // @sink(cont-ptr-commaok-map-decoy)
}
