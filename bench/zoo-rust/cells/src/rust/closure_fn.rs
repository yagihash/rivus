// @cell(F/rust/closure-fn)
// @shape: a closure that is an Fn (it captures by reference) is given to a helper that takes impl Fn and calls it
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/closure-fn/vuln", vuln);
    m::case("F/rust/closure-fn/fixed", fixed);
    m::case("F/rust/closure-fn/decoy", decoy);
    m::case("F/rust/closure-fn/vuln2", vuln2);
}

fn apply(f: impl Fn(&str) -> String, s: &str) -> String {
    f(s)
}

fn vuln() { // @vuln
    let r = apply(|x| x.to_string(), &m::src("rust-closure-fn-vuln")); // @source(rust-closure-fn-vuln)
    m::sink_cmd(&r); // @sink(rust-closure-fn-vuln)
}

fn vuln2() { // @vuln
    let v = m::src("rust-closure-fn-vuln2"); // @source(rust-closure-fn-vuln2)
    let r = apply(|_| v.clone(), "constant");
    m::sink_cmd(&r); // @sink(rust-closure-fn-vuln2)
}

fn fixed() { // @fixed
    let r = apply(|x| x.to_string(), &m::clean_cmd(&m::src("rust-closure-fn-fixed")));
    m::sink_cmd(&r); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("rust-closure-fn-decoy"); // @source(rust-closure-fn-decoy)
    let a = apply(|x| x.to_string(), "constant");
    let b = apply(|x| x.to_string(), &v);
    m::sink_cmd(&a); // @sink(none) @nosink(rust-closure-fn-decoy)
    m::sink_sql(&b); // @sink(rust-closure-fn-decoy)
}
