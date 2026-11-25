// @cell(F/cont/ptr-get-mut)
// @shape: a `&mut` taken out of a Vec by `get_mut`, `first_mut` or `last_mut`, and a field written through it, is read from the Vec again
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-get-mut/vuln", vuln);
    m::case("F/cont/ptr-get-mut/vuln2", vuln2);
    m::case("F/cont/ptr-get-mut/vuln3", vuln3);
    m::case("F/cont/ptr-get-mut/fixed", fixed);
    m::case("F/cont/ptr-get-mut/decoy", decoy);
}

struct Boxed {
    v: String,
}

fn vuln() { // @vuln
    let mut xs = vec![Boxed { v: String::new() }];
    let p = xs.get_mut(0).unwrap();
    p.v = m::src("cont-ptr-get-mut-vuln"); // @source(cont-ptr-get-mut-vuln)
    m::sink_cmd(&xs[0].v); // @sink(cont-ptr-get-mut-vuln)
}

fn vuln2() { // @vuln
    let mut xs = vec![Boxed { v: String::new() }];
    let p = xs.first_mut().unwrap();
    p.v = m::src("cont-ptr-get-mut-vuln2"); // @source(cont-ptr-get-mut-vuln2)
    m::sink_cmd(&xs[0].v); // @sink(cont-ptr-get-mut-vuln2)
}

fn vuln3() { // @vuln
    let mut xs = vec![Boxed { v: String::new() }];
    let p = xs.last_mut().unwrap();
    p.v = m::src("cont-ptr-get-mut-vuln3"); // @source(cont-ptr-get-mut-vuln3)
    m::sink_cmd(&xs[0].v); // @sink(cont-ptr-get-mut-vuln3)
}

fn fixed() { // @fixed
    let mut xs = vec![Boxed { v: String::new() }];
    let p = xs.get_mut(0).unwrap();
    p.v = m::clean_cmd(&m::src("cont-ptr-get-mut-fixed"));
    m::sink_cmd(&xs[0].v); // @sink(none)
}

fn decoy() { // @decoy
    let mut xs = vec![Boxed { v: String::new() }];
    let ys = vec![Boxed { v: String::new() }];
    let p = xs.get_mut(0).unwrap();
    p.v = m::src("cont-ptr-get-mut-decoy"); // @source(cont-ptr-get-mut-decoy)
    m::sink_cmd(&ys[0].v); // @sink(none)
    m::sink_sql(&xs[0].v); // @sink(cont-ptr-get-mut-decoy)
}
