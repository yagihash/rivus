// @cell(F/clos/callback-of-callback)
// @shape: a closure takes a function as its parameter and calls it (g(a)): the callback of a callback
use zoo_mark as m;

// The value has a type of its own, so that the calls of a function value of this cell do not meet those of the other cells.
struct Cb(String);

pub fn register() {
    m::case("F/clos/callback-of-callback/vuln", vuln);
    m::case("F/clos/callback-of-callback/fixed", fixed);
    m::case("F/clos/callback-of-callback/decoy", decoy);
}

fn via_inner(s: &str, outer: impl Fn(&dyn Fn(Cb) -> Cb) -> Cb) -> Cb {
    outer(&|x: Cb| Cb(x.0 + s))
}

fn vuln() { // @vuln
    let a = m::src("clos-callback-of-callback-vuln"); // @source(clos-callback-of-callback-vuln)
    let out = via_inner("const", |g| g(Cb(a.clone())));
    m::sink_cmd(&out.0); // @sink(clos-callback-of-callback-vuln)
}

fn fixed() { // @fixed
    let a = m::clean_cmd(&m::src("clos-callback-of-callback-fixed"));
    let out = via_inner("const", |g| g(Cb(a.clone())));
    m::sink_cmd(&out.0); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("clos-callback-of-callback-decoy"); // @source(clos-callback-of-callback-decoy)
    let out = via_inner("const", |g| g(Cb("constant".to_string())));
    m::sink_cmd(&out.0); // @sink(none) @nosink(clos-callback-of-callback-decoy)
    let out2 = via_inner("const", |g| g(Cb(a.clone())));
    m::sink_sql(&out2.0); // @sink(clos-callback-of-callback-decoy)
}
