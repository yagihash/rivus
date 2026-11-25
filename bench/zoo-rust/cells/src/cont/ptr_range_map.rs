// @cell(F/cont/ptr-range-map)
// @shape: a `&mut` taken as the value of a loop over a map (`values_mut`, `iter_mut`, `&mut` of the map), and a field written through it, is read from the map again
use std::collections::HashMap;
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-range-map/vuln", vuln);
    m::case("F/cont/ptr-range-map/fixed", fixed);
    m::case("F/cont/ptr-range-map/decoy", decoy);
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
    for p in t.values_mut() {
        p.v = m::src("cont-ptr-range-map-vuln"); // @source(cont-ptr-range-map-vuln)
    }
    m::sink_cmd(&t["a"].v); // @sink(cont-ptr-range-map-vuln)
}

fn fixed() { // @fixed
    let mut t = table();
    for p in t.values_mut() {
        p.v = m::clean_cmd(&m::src("cont-ptr-range-map-fixed"));
    }
    for (_, p) in t.iter_mut() {
        p.v = m::clean_cmd(&m::src("cont-ptr-range-map-fixed2"));
    }
    for (_, p) in &mut t {
        p.v = m::clean_cmd(&m::src("cont-ptr-range-map-fixed3"));
    }
    m::sink_cmd(&t["a"].v); // @sink(none)
}

fn decoy() { // @decoy
    let mut t = table();
    let u = table();
    for p in t.values_mut() {
        p.v = m::src("cont-ptr-range-map-decoy"); // @source(cont-ptr-range-map-decoy)
    }
    m::sink_cmd(&u["a"].v); // @sink(none)
    m::sink_sql(&t["a"].v); // @sink(cont-ptr-range-map-decoy)
}
