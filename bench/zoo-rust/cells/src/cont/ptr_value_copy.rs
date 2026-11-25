// @cell(F/cont/ptr-value-copy)
// @shape: a value copied out of a container (`clone()`) is not an alias: a field written in the copy is not written back to the original
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-value-copy/vuln", vuln);
    m::case("F/cont/ptr-value-copy/fixed", fixed);
    m::case("F/cont/ptr-value-copy/decoy", decoy);
}

#[derive(Clone)]
struct Boxed {
    v: String,
}

// the copy itself is tainted and read: that is a finding
fn vuln() { // @vuln
    let xs = vec![Boxed { v: String::new() }];
    let mut c = xs[0].clone();
    c.v = m::src("cont-ptr-value-copy-vuln"); // @source(cont-ptr-value-copy-vuln)
    m::sink_cmd(&c.v); // @sink(cont-ptr-value-copy-vuln)
}

fn fixed() { // @fixed
    let xs = vec![Boxed { v: String::new() }];
    let mut c = xs[0].clone();
    c.v = m::clean_cmd(&m::src("cont-ptr-value-copy-fixed"));
    m::sink_cmd(&c.v); // @sink(none)
}

// the write goes to the copy only: the original is clean
fn decoy() { // @decoy
    let xs = vec![Boxed { v: String::new() }];
    let mut c = xs[0].clone();
    c.v = m::src("cont-ptr-value-copy-decoy"); // @source(cont-ptr-value-copy-decoy)
    m::sink_cmd(&xs[0].v); // @sink(none)
    m::sink_html(&c.v); // @sink(cont-ptr-value-copy-decoy)
}
