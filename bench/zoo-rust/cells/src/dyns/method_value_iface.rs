// @cell(F/dyn/method-value-iface)
// @shape: the method of a trait is taken as a function value (<dyn Getter>::get) and called with a trait object; the decoy has two objects
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/method-value-iface/vuln", vuln);
    m::case("F/dyn/method-value-iface/fixed", fixed);
    m::case("F/dyn/method-value-iface/decoy", decoy);
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
    let b = Bx { v: m::src("dyn-method-value-iface-vuln") }; // @source(dyn-method-value-iface-vuln)
    let g = <dyn Getter>::get;
    m::sink_cmd(&g(&b)); // @sink(dyn-method-value-iface-vuln)
}

fn fixed() { // @fixed
    let b = Bx { v: m::clean_cmd(&m::src("dyn-method-value-iface-fixed")) };
    let g = <dyn Getter>::get;
    m::sink_cmd(&g(&b)); // @sink(none)
}

fn decoy() { // @decoy
    let a = Bx { v: m::src("dyn-method-value-iface-decoy") }; // @source(dyn-method-value-iface-decoy)
    let b = Bx { v: "constant".to_string() };
    let g = <dyn Getter>::get;
    m::sink_cmd(&g(&b)); // @sink(none) @nosink(dyn-method-value-iface-decoy)
    m::sink_sql(&g(&a)); // @sink(dyn-method-value-iface-decoy)
}
