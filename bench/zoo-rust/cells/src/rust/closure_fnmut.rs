// @cell(F/rust/closure-fnmut)
// @shape: a closure that is an FnMut writes a captured String, and the variable is read after the calls (called by a helper that takes impl FnMut)
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/closure-fnmut/vuln", vuln);
    m::case("F/rust/closure-fnmut/fixed", fixed);
    m::case("F/rust/closure-fnmut/decoy", decoy);
    m::case("F/rust/closure-fnmut/vuln2", vuln2);
}

fn twice(mut f: impl FnMut(&str), s: &str) {
    f(s);
    f(s);
}

fn vuln() { // @vuln
    let mut acc = String::new();
    let mut f = |x: &str| acc.push_str(x);
    f(&m::src("rust-closure-fnmut-vuln")); // @source(rust-closure-fnmut-vuln) @miss(silent) @ref(ENV-PTR)
    m::sink_cmd(&acc); // @sink(rust-closure-fnmut-vuln)
}

fn vuln2() { // @vuln
    let mut acc = String::new();
    twice(|x| acc.push_str(x), &m::src("rust-closure-fnmut-vuln2")); // @source(rust-closure-fnmut-vuln2) @miss(silent) @ref(ENV-PTR)
    m::sink_cmd(&acc); // @sink(rust-closure-fnmut-vuln2)
}

fn fixed() { // @fixed
    let mut acc = String::new();
    twice(|x| acc.push_str(x), &m::clean_cmd(&m::src("rust-closure-fnmut-fixed")));
    m::sink_cmd(&acc); // @sink(none)
}

fn decoy() { // @decoy
    let mut a = String::new();
    let mut b = String::new();
    twice(|x| a.push_str(x), &m::src("rust-closure-fnmut-decoy")); // @source(rust-closure-fnmut-decoy) @miss(silent) @ref(ENV-PTR)
    twice(|x| b.push_str(x), "constant");
    m::sink_cmd(&b); // @sink(none) @nosink(rust-closure-fnmut-decoy)
    m::sink_sql(&a); // @sink(rust-closure-fnmut-decoy)
}
