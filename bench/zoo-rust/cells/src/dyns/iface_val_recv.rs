// @cell(F/dyn/iface-val-recv)
// @shape: a value is held by a struct whose method takes self by value (self: Box<Self>), and is read through a trait object
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/iface-val-recv/vuln", vuln);
    m::case("F/dyn/iface-val-recv/fixed", fixed);
    m::case("F/dyn/iface-val-recv/decoy", decoy);
}

trait Getter {
    fn get(self: Box<Self>) -> String;
}

struct Bx {
    v: String,
}

impl Getter for Bx {
    fn get(self: Box<Self>) -> String {
        self.v
    }
}

fn vuln() { // @vuln
    let g: Box<dyn Getter> = Box::new(Bx { v: m::src("dyn-iface-val-recv-vuln") }); // @source(dyn-iface-val-recv-vuln)
    m::sink_cmd(&g.get()); // @sink(dyn-iface-val-recv-vuln)
}

fn fixed() { // @fixed
    let g: Box<dyn Getter> = Box::new(Bx { v: m::clean_cmd(&m::src("dyn-iface-val-recv-fixed")) });
    m::sink_cmd(&g.get()); // @sink(none)
}

fn decoy() { // @decoy
    let a: Box<dyn Getter> = Box::new(Bx { v: m::src("dyn-iface-val-recv-decoy") }); // @source(dyn-iface-val-recv-decoy)
    let b: Box<dyn Getter> = Box::new(Bx { v: "constant".to_string() });
    m::sink_cmd(&b.get()); // @sink(none) @nosink(dyn-iface-val-recv-decoy)
    m::sink_sql(&a.get()); // @sink(dyn-iface-val-recv-decoy)
}
