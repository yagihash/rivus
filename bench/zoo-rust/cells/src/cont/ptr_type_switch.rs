// @cell(F/cont/ptr-type-switch)
// @shape: a `&mut` bound by a `match` on a `&mut enum`, and a field written through it, is read from the enum again
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-type-switch/vuln", vuln);
    m::case("F/cont/ptr-type-switch/fixed", fixed);
    m::case("F/cont/ptr-type-switch/decoy", decoy);
}

struct Boxed {
    v: String,
}

enum Any {
    B(Boxed),
}

fn read(x: &Any) -> &str {
    match x {
        Any::B(b) => &b.v,
    }
}

fn vuln() { // @vuln
    let mut x = Any::B(Boxed { v: String::new() });
    match &mut x {
        Any::B(p) => p.v = m::src("cont-ptr-type-switch-vuln"), // @source(cont-ptr-type-switch-vuln)
    }
    m::sink_cmd(read(&x)); // @sink(cont-ptr-type-switch-vuln)
}

fn fixed() { // @fixed
    let mut x = Any::B(Boxed { v: String::new() });
    match &mut x {
        Any::B(p) => p.v = m::clean_cmd(&m::src("cont-ptr-type-switch-fixed")),
    }
    m::sink_cmd(read(&x)); // @sink(none)
}

fn decoy() { // @decoy
    let mut x = Any::B(Boxed { v: String::new() });
    let y = Any::B(Boxed { v: String::new() });
    match &mut x {
        Any::B(p) => p.v = m::src("cont-ptr-type-switch-decoy"), // @source(cont-ptr-type-switch-decoy)
    }
    m::sink_cmd(read(&y)); // @sink(none)
    m::sink_sql(read(&x)); // @sink(cont-ptr-type-switch-decoy)
}
