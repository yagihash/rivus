// @cell(F/dyn/iface-ptr-recv-read)
// @shape: a value is held by a struct with a &self method, and is read through a &dyn trait object
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/iface-ptr-recv-read/vuln", vuln);
    m::case("F/dyn/iface-ptr-recv-read/fixed", fixed);
    m::case("F/dyn/iface-ptr-recv-read/decoy", decoy);
}

trait Getter {
    fn get(&self) -> String;
}

struct Bx {
    v: String,
}

impl Getter for Bx {
    fn get(&self) -> String {
        self.v.clone()
    }
}

fn vuln() { // @vuln
    let b = Bx { v: m::src("dyn-iface-ptr-recv-read-vuln") }; // @source(dyn-iface-ptr-recv-read-vuln)
    let g: &dyn Getter = &b;
    m::sink_cmd(&g.get()); // @sink(dyn-iface-ptr-recv-read-vuln)
}

fn fixed() { // @fixed
    let b = Bx { v: m::clean_cmd(&m::src("dyn-iface-ptr-recv-read-fixed")) };
    let g: &dyn Getter = &b;
    m::sink_cmd(&g.get()); // @sink(none)
}

fn decoy() { // @decoy
    let ba = Bx { v: m::src("dyn-iface-ptr-recv-read-decoy") }; // @source(dyn-iface-ptr-recv-read-decoy)
    let bb = Bx { v: "constant".to_string() };
    let a: &dyn Getter = &ba;
    let b: &dyn Getter = &bb;
    m::sink_cmd(&b.get()); // @sink(none) @nosink(dyn-iface-ptr-recv-read-decoy)
    m::sink_sql(&a.get()); // @sink(dyn-iface-ptr-recv-read-decoy)
}
