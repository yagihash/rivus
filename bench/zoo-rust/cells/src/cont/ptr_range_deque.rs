// @cell(F/cont/ptr-range-deque)
// @shape: a `&mut` taken as the value of a loop over a VecDeque (`iter_mut`, `&mut` of the deque), and a field written through it, is read from the deque again
use std::collections::VecDeque;
use zoo_mark as m;

pub fn register() {
    m::case("F/cont/ptr-range-deque/vuln", vuln);
    m::case("F/cont/ptr-range-deque/vuln2", vuln2);
    m::case("F/cont/ptr-range-deque/fixed", fixed);
    m::case("F/cont/ptr-range-deque/decoy", decoy);
}

struct Boxed {
    v: String,
}

fn queue() -> VecDeque<Boxed> {
    let mut d = VecDeque::new();
    d.push_back(Boxed { v: String::new() });
    d
}

fn vuln() { // @vuln
    let mut d = queue();
    for p in d.iter_mut() {
        p.v = m::src("cont-ptr-range-deque-vuln"); // @source(cont-ptr-range-deque-vuln)
    }
    m::sink_cmd(&d[0].v); // @sink(cont-ptr-range-deque-vuln)
}

fn vuln2() { // @vuln
    let mut d = queue();
    for p in &mut d {
        p.v = m::src("cont-ptr-range-deque-vuln2"); // @source(cont-ptr-range-deque-vuln2)
    }
    m::sink_cmd(&d[0].v); // @sink(cont-ptr-range-deque-vuln2)
}

fn fixed() { // @fixed
    let mut d = queue();
    for p in d.iter_mut() {
        p.v = m::clean_cmd(&m::src("cont-ptr-range-deque-fixed"));
    }
    m::sink_cmd(&d[0].v); // @sink(none)
}

fn decoy() { // @decoy
    let mut d = queue();
    let e = queue();
    for p in d.iter_mut() {
        p.v = m::src("cont-ptr-range-deque-decoy"); // @source(cont-ptr-range-deque-decoy)
    }
    m::sink_cmd(&e[0].v); // @sink(none)
    m::sink_sql(&d[0].v); // @sink(cont-ptr-range-deque-decoy)
}
