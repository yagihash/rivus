// @cell(F/cont/ptr-range-array)
// @shape: a `&mut` taken as the value of a loop over a `&mut` slice or a `&mut` array, and a field written through it, is read from the owner again
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-range-array/vuln", vuln);
    m::case("F/cont/ptr-range-array/vuln2", vuln2);
    m::case("F/cont/ptr-range-array/fixed", fixed);
    m::case("F/cont/ptr-range-array/decoy", decoy);
}

struct Boxed {
    v: String,
}

fn vuln() { // @vuln
    let mut xs = vec![Boxed { v: String::new() }];
    let part: &mut [Boxed] = &mut xs[..];
    for p in part {
        p.v = m::src("cont-ptr-range-array-vuln"); // @source(cont-ptr-range-array-vuln)
    }
    m::sink_cmd(&xs[0].v); // @sink(cont-ptr-range-array-vuln)
}

fn fill(xs: &mut [Boxed]) {
    for p in xs {
        p.v = m::src("cont-ptr-range-array-vuln2"); // @source(cont-ptr-range-array-vuln2)
    }
}

fn vuln2() { // @vuln
    let mut xs = vec![Boxed { v: String::new() }];
    fill(&mut xs);
    m::sink_cmd(&xs[0].v); // @sink(cont-ptr-range-array-vuln2)
}

fn fixed() { // @fixed
    let mut xs = vec![Boxed { v: String::new() }];
    let part: &mut [Boxed] = &mut xs[..];
    for p in part {
        p.v = m::clean_cmd(&m::src("cont-ptr-range-array-fixed"));
    }
    m::sink_cmd(&xs[0].v); // @sink(none)
    let mut zs = [String::new()];
    for p in &mut zs {
        *p = m::clean_cmd(&m::src("cont-ptr-range-array-fixed2"));
    }
    m::sink_cmd(&zs[0]); // @sink(none)
}

fn decoy() { // @decoy
    let mut xs = vec![Boxed { v: String::new() }];
    let ys = vec![Boxed { v: String::new() }];
    let part: &mut [Boxed] = &mut xs[..];
    for p in part {
        p.v = m::src("cont-ptr-range-array-decoy"); // @source(cont-ptr-range-array-decoy)
    }
    m::sink_cmd(&ys[0].v); // @sink(none)
    m::sink_sql(&xs[0].v); // @sink(cont-ptr-range-array-decoy)
}
