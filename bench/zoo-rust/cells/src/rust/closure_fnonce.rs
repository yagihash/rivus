// @cell(F/rust/closure-fnonce)
// @shape: a move closure that is an FnOnce returns the value it captured, and a helper calls it once
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/closure-fnonce/vuln", vuln);
    m::case("F/rust/closure-fnonce/fixed", fixed);
    m::case("F/rust/closure-fnonce/decoy", decoy);
}

fn call_once<F: FnOnce() -> String>(f: F) -> String {
    f()
}

fn vuln() { // @vuln
    let v = m::src("rust-closure-fnonce-vuln"); // @source(rust-closure-fnonce-vuln)
    let r = call_once(move || v);
    m::sink_cmd(&r); // @sink(rust-closure-fnonce-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("rust-closure-fnonce-fixed"));
    let r = call_once(move || v);
    m::sink_cmd(&r); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("rust-closure-fnonce-decoy"); // @source(rust-closure-fnonce-decoy)
    let k = "constant".to_string();
    let a = call_once(move || k);
    let b = call_once(move || v);
    m::sink_cmd(&a); // @sink(none) @nosink(rust-closure-fnonce-decoy)
    m::sink_sql(&b); // @sink(rust-closure-fnonce-decoy)
}
