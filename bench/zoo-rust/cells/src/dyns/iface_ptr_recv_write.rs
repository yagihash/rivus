// @cell(F/dyn/iface-ptr-recv-write)
// @shape: a &mut self method writes a field through a trait object (i.set(src)), and the original struct is read afterwards
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/iface-ptr-recv-write/vuln", vuln);
    m::case("F/dyn/iface-ptr-recv-write/fixed", fixed);
    m::case("F/dyn/iface-ptr-recv-write/decoy", decoy);
}

trait Setter {
    fn set(&mut self, s: String);
}

struct Holder {
    v: String,
}

impl Setter for Holder {
    fn set(&mut self, s: String) {
        self.v = s;
    }
}

fn vuln() { // @vuln
    let mut h = Holder { v: String::new() };
    let i: &mut dyn Setter = &mut h;
    i.set(m::src("dyn-iface-ptr-recv-write-vuln")); // @source(dyn-iface-ptr-recv-write-vuln)
    m::sink_cmd(&h.v); // @sink(dyn-iface-ptr-recv-write-vuln)
}

fn fixed() { // @fixed
    let mut h = Holder { v: String::new() };
    let i: &mut dyn Setter = &mut h;
    i.set(m::clean_cmd(&m::src("dyn-iface-ptr-recv-write-fixed")));
    m::sink_cmd(&h.v); // @sink(none)
}

fn decoy() { // @decoy
    let mut h = Holder { v: String::new() };
    let other = Holder { v: "constant".to_string() };
    let i: &mut dyn Setter = &mut h;
    i.set(m::src("dyn-iface-ptr-recv-write-decoy")); // @source(dyn-iface-ptr-recv-write-decoy)
    m::sink_cmd(&other.v); // @sink(none) @nosink(dyn-iface-ptr-recv-write-decoy)
    m::sink_sql(&h.v); // @sink(dyn-iface-ptr-recv-write-decoy)
}
