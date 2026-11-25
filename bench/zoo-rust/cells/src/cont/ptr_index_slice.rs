// @cell(F/cont/ptr-index-slice)
// @shape: a `&mut` taken by the index of a Vec, and a field written through it, is read from the Vec again
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-index-slice/vuln", vuln);
    m::case("F/cont/ptr-index-slice/fixed", fixed);
    m::case("F/cont/ptr-index-slice/decoy", decoy);
}

struct Boxed {
    v: String,
}

fn vuln() { // @vuln
    let mut xs = vec![Boxed { v: String::new() }];
    let p = &mut xs[0];
    p.v = m::src("cont-ptr-index-slice-vuln"); // @source(cont-ptr-index-slice-vuln)
    m::sink_cmd(&xs[0].v); // @sink(cont-ptr-index-slice-vuln)
}

fn fixed() { // @fixed
    let mut xs = vec![Boxed { v: String::new() }];
    let p = &mut xs[0];
    p.v = m::clean_cmd(&m::src("cont-ptr-index-slice-fixed"));
    m::sink_cmd(&xs[0].v); // @sink(none)
}

fn decoy() { // @decoy
    let mut xs = vec![Boxed { v: String::new() }];
    let ys = vec![Boxed { v: String::new() }];
    let p = &mut xs[0];
    p.v = m::src("cont-ptr-index-slice-decoy"); // @source(cont-ptr-index-slice-decoy)
    m::sink_cmd(&ys[0].v); // @sink(none)
    m::sink_sql(&xs[0].v); // @sink(cont-ptr-index-slice-decoy)
}
