// @cell(F/dyn/func-map)
// @shape: a function value looked up in a map by a constant key is called; the map has another entry with another sink
use std::collections::HashMap;
use zoo_mark as m;

// Each case has its own argument type (a newtype of the marker), so that the cases of the cell do not meet at a call of
// Fn::call, whose candidates are the closures of the same argument types (the same type in one case is the point of the decoy).
struct Tv(String);
struct Tf(String);
struct Td(String);

pub fn register() {
    m::case("F/dyn/func-map/vuln", vuln);
    m::case("F/dyn/func-map/fixed", fixed);
    m::case("F/dyn/func-map/decoy", decoy);
}

fn vuln() { // @vuln
    let v = Tv(m::src("dyn-func-map-vuln")); // @source(dyn-func-map-vuln) @miss(unmodeled_library_call) @ref(FUNC-VALUE-MEMORY,PARITY)
    let mut t: HashMap<&str, Box<dyn Fn(&Tv)>> = HashMap::new();
    t.insert(
        "cmd",
        Box::new(|x| {
            m::sink_cmd(&x.0); // @sink(dyn-func-map-vuln)
        }),
    );
    (t["cmd"])(&v);
}

fn fixed() { // @fixed
    let v = Tf(m::clean_cmd(&m::src("dyn-func-map-fixed")));
    let mut t: HashMap<&str, Box<dyn Fn(&Tf)>> = HashMap::new();
    t.insert(
        "cmd",
        Box::new(|x| {
            m::sink_cmd(&x.0); // @sink(none)
        }),
    );
    (t["cmd"])(&v);
}

fn decoy() { // @decoy
    let v = Td(m::src("dyn-func-map-decoy")); // @source(dyn-func-map-decoy) @miss(unmodeled_library_call) @ref(FUNC-VALUE-MEMORY,PARITY)
    let mut t: HashMap<&str, Box<dyn Fn(&Td)>> = HashMap::new();
    t.insert(
        "cmd",
        Box::new(|x| {
            m::sink_cmd(&x.0); // @sink(none) @nosink(dyn-func-map-decoy)
        }),
    );
    t.insert(
        "sql",
        Box::new(|x| {
            m::sink_sql(&x.0); // @sink(dyn-func-map-decoy)
        }),
    );
    (t["cmd"])(&Td("constant".to_string()));
    (t["sql"])(&v);
}
