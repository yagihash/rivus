// @cell(F/rust/vec-index-write)
// @shape: a value is written at the index of a Vec (the element, a field of the element, a push_str into the element) and read from the Vec again
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/vec-index-write/vuln", vuln);
    m::case("F/rust/vec-index-write/fixed", fixed);
    m::case("F/rust/vec-index-write/decoy", decoy);
    m::case("F/rust/vec-index-write/vuln2", vuln2);
    m::case("F/rust/vec-index-write/vuln3", vuln3);
}

struct Boxed {
    v: String,
}

fn vuln() { // @vuln
    let mut xs = vec![String::new()];
    xs[0] = m::src("rust-vec-index-write-vuln"); // @source(rust-vec-index-write-vuln)
    m::sink_cmd(&xs[0]); // @sink(rust-vec-index-write-vuln)
}

fn vuln2() { // @vuln
    let mut xs = vec![Boxed { v: String::new() }];
    xs[0].v = m::src("rust-vec-index-write-vuln2"); // @source(rust-vec-index-write-vuln2)
    m::sink_cmd(&xs[0].v); // @sink(rust-vec-index-write-vuln2)
}

fn vuln3() { // @vuln
    let mut xs = vec![String::new()];
    xs[0].push_str(&m::src("rust-vec-index-write-vuln3")); // @source(rust-vec-index-write-vuln3)
    m::sink_cmd(&xs[0]); // @sink(rust-vec-index-write-vuln3)
}

fn fixed() { // @fixed
    let mut xs = vec![String::new()];
    xs[0] = m::clean_cmd(&m::src("rust-vec-index-write-fixed"));
    m::sink_cmd(&xs[0]); // @sink(none)
}

fn decoy() { // @decoy
    let mut xs = vec![String::new()];
    let ys = vec![String::from("constant")];
    xs[0] = m::src("rust-vec-index-write-decoy"); // @source(rust-vec-index-write-decoy)
    m::sink_cmd(&ys[0]); // @sink(none)
    m::sink_sql(&xs[0]); // @sink(rust-vec-index-write-decoy)
}
