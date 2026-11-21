// @cell(F/rust/deref)
// @shape: a wrapper that implements Deref (and DerefMut) is used as its target: deref coercion into a &str parameter, and a method of the target that writes through DerefMut
use std::ops::{Deref, DerefMut};
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/deref/vuln", vuln);
    m::case("F/rust/deref/fixed", fixed);
    m::case("F/rust/deref/decoy", decoy);
    m::case("F/rust/deref/vuln2", vuln2);
}

struct W(String);

impl Deref for W {
    type Target = String;
    fn deref(&self) -> &String {
        &self.0
    }
}

impl DerefMut for W {
    fn deref_mut(&mut self) -> &mut String {
        &mut self.0
    }
}

fn vuln() { // @vuln
    let w = W(m::src("rust-deref-vuln")); // @source(rust-deref-vuln)
    m::sink_cmd(&w); // @sink(rust-deref-vuln)
}

fn vuln2() { // @vuln
    let mut w = W(String::new());
    w.push_str(&m::src("rust-deref-vuln2")); // @source(rust-deref-vuln2)
    m::sink_cmd(&w); // @sink(rust-deref-vuln2)
}

fn fixed() { // @fixed
    let w = W(m::clean_cmd(&m::src("rust-deref-fixed")));
    m::sink_cmd(&w); // @sink(none)
}

fn decoy() { // @decoy
    let a = W(m::src("rust-deref-decoy")); // @source(rust-deref-decoy)
    let b = W("constant".to_string());
    m::sink_cmd(&b); // @sink(none) @nosink(rust-deref-decoy)
    m::sink_sql(&a); // @sink(rust-deref-decoy)
}
