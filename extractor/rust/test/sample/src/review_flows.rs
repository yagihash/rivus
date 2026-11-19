use crate::{sink, source};

// What a review of the extractor asked for (REVIEW-2026-11-19): flows that were
// lost without a trace. Each case is a form that the other samples do not have.

// --- RUST-1: the end of a scope calls Drop::drop ----------------------------

pub struct Logger {
    buf: String,
}

impl Drop for Logger {
    fn drop(&mut self) {
        sink(&self.buf); // @sink(rv-drop)
    }
}

pub fn drop_flow() {
    let s = source(); // @source(rv-drop)
    let mut l = Logger { buf: String::new() };
    l.buf.push_str(&s);
}

// --- RUST-2: a closure that is given to a generic function ------------------

fn apply<F: FnOnce(String)>(f: F, v: String) {
    f(v)
}

pub fn closure_in_crate_generic() {
    let s = source(); // @source(rv-closure)
    apply(|x| sink(&x), s); // @sink(rv-closure)
}

pub fn closure_in_std_for_each() {
    let s = source(); // @source(rv-for-each)
    vec![s].iter().for_each(|x| sink(x)); // @sink(rv-for-each)
}

// --- RUST-3: a call through dyn finds the implementations in the crate ------

pub struct Feed;

impl Iterator for Feed {
    type Item = String;
    fn next(&mut self) -> Option<String> {
        Some(source()) // @source(rv-dyn-iter)
    }
}

pub fn drain(it: &mut dyn Iterator<Item = String>) {
    while let Some(s) = it.next() {
        sink(&s); // @sink(rv-dyn-iter)
    }
}

pub fn run_drain() {
    drain(&mut Feed);
}

pub trait Source2 {
    fn get(&self) -> String;
    fn twice(&self) -> String {
        let s = self.get();
        format!("{}{}", s, s)
    }
}

pub struct Impl2;

impl Source2 for Impl2 {
    fn get(&self) -> String {
        source() // @source(rv-default-method)
    }
}

pub fn through_default_method(x: &dyn Source2) {
    sink(&x.twice()); // @sink(rv-default-method)
}

pub fn run_default_method() {
    through_default_method(&Impl2);
}

// --- RUST-7: a union holds the field that was written, not the first one ----

pub union Either {
    n: u64,
    s: std::mem::ManuallyDrop<String>,
}

pub fn union_flow() {
    let s = source(); // @source(rv-union)
    let u = Either { s: std::mem::ManuallyDrop::new(s) };
    unsafe {
        sink(&u.s); // @sink(rv-union)
    }
}
