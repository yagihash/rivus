// @cell(F/dyn/generics-ptr)
// @shape: a generic struct instantiated with String (G<String>) implements a generic trait, and is read through a &dyn trait object
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/generics-ptr/vuln", vuln);
    m::case("F/dyn/generics-ptr/fixed", fixed);
    m::case("F/dyn/generics-ptr/decoy", decoy);
}

trait GetterOf<T> {
    fn get(&self) -> T;
}

struct G<T> {
    v: T,
}

impl<T: Clone> GetterOf<T> for G<T> {
    fn get(&self) -> T {
        self.v.clone()
    }
}

fn vuln() { // @vuln
    let g = G { v: m::src("dyn-generics-ptr-vuln") }; // @source(dyn-generics-ptr-vuln)
    let i: &dyn GetterOf<String> = &g;
    m::sink_cmd(&i.get()); // @sink(dyn-generics-ptr-vuln)
}

fn fixed() { // @fixed
    let g = G { v: m::clean_cmd(&m::src("dyn-generics-ptr-fixed")) };
    let i: &dyn GetterOf<String> = &g;
    m::sink_cmd(&i.get()); // @sink(none)
}

fn decoy() { // @decoy
    let ga = G { v: m::src("dyn-generics-ptr-decoy") }; // @source(dyn-generics-ptr-decoy)
    let gb = G { v: "constant".to_string() };
    let a: &dyn GetterOf<String> = &ga;
    let b: &dyn GetterOf<String> = &gb;
    m::sink_cmd(&b.get()); // @sink(none) @nosink(dyn-generics-ptr-decoy)
    m::sink_sql(&a.get()); // @sink(dyn-generics-ptr-decoy)
}
