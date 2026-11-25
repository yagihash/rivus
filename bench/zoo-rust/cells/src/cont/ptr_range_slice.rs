// @cell(F/cont/ptr-range-slice)
// @shape: a `&mut` taken as the value of a loop over a Vec, and a field written through it, is read from the Vec again
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-range-slice/vuln", vuln);
    m::case("F/cont/ptr-range-slice/fixed", fixed);
    m::case("F/cont/ptr-range-slice/decoy", decoy);
    m::case("F/cont/ptr-range-slice/vuln2", vuln2);
    m::case("F/cont/ptr-range-slice/vuln3", vuln3);
}

struct Boxed {
    v: String,
}

fn vuln() { // @vuln
    let mut xs = vec![Boxed { v: String::new() }];
    for p in xs.iter_mut() {
        p.v = m::src("cont-ptr-range-slice-vuln"); // @source(cont-ptr-range-slice-vuln)
    }
    m::sink_cmd(&xs[0].v); // @sink(cont-ptr-range-slice-vuln)
}

fn vuln2() { // @vuln
    let mut xs = vec![Boxed { v: String::new() }];
    for p in &mut xs {
        p.v = m::src("cont-ptr-range-slice-vuln2"); // @source(cont-ptr-range-slice-vuln2)
    }
    m::sink_cmd(&xs[0].v); // @sink(cont-ptr-range-slice-vuln2)
}

fn vuln3() { // @vuln
    let mut xs = vec![String::new()];
    for p in xs.iter_mut() {
        *p = m::src("cont-ptr-range-slice-vuln3"); // @source(cont-ptr-range-slice-vuln3)
    }
    m::sink_cmd(&xs[0]); // @sink(cont-ptr-range-slice-vuln3)
}

fn fixed() { // @fixed
    let mut xs = vec![Boxed { v: String::new() }];
    for p in xs.iter_mut() {
        p.v = m::clean_cmd(&m::src("cont-ptr-range-slice-fixed"));
    }
    m::sink_cmd(&xs[0].v); // @sink(none)
}

fn decoy() { // @decoy
    let mut xs = vec![Boxed { v: String::new() }];
    let ys = vec![Boxed { v: String::new() }];
    for p in xs.iter_mut() {
        p.v = m::src("cont-ptr-range-slice-decoy"); // @source(cont-ptr-range-slice-decoy)
    }
    m::sink_cmd(&ys[0].v); // @sink(none)
    m::sink_sql(&xs[0].v); // @sink(cont-ptr-range-slice-decoy)
}
