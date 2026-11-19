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

// --- RUST-8: a foreign function is a call that leaves the language ----------

extern "C" {
    fn atoi(s: *const u8) -> i32;
}

pub fn foreign_call() -> i32 {
    let s = source();
    unsafe { atoi(s.as_ptr()) }
}

// --- RUST-9: a call with a const generic argument that is still open --------

pub struct W<const N: usize> {
    v: String,
}

fn pass_w<const N: usize>(w: W<N>) -> String {
    w.v
}

fn outer_w<const N: usize>(w: W<N>) {
    sink(&pass_w(w)); // @sink(rv-const-generic)
}

pub fn run_const_generic() {
    let s = source(); // @source(rv-const-generic)
    outer_w(W::<3> { v: s });
}

// An array that is made and read in one function, and one that is passed on.
pub fn array_local() {
    let s = source(); // @source(rv-array-local)
    let a = [s];
    sink(&a[0]); // @sink(rv-array-local)
}

fn first_of(a: [String; 1]) -> String {
    a[0].clone()
}

pub fn array_passed() {
    let s = source(); // @source(rv-array-passed)
    sink(&first_of([s])); // @sink(rv-array-passed)
}

// --- #40: a call of generic code is linked to the function that holds the code

// (a) a generic function and the method of a generic type, in a dependency
pub fn dep_generic() {
    let s = source(); // @source(rv-dep-generic)
    sink(&depcrate::pass(s)); // @sink(rv-dep-generic)
}

pub fn dep_generic_method() {
    let s = source(); // @source(rv-dep-generic-method)
    sink(&depcrate::Holder(s).into_inner()); // @sink(rv-dep-generic-method)
}

// (b) an impl of a trait for a generic type overrides the default body
pub trait Tr {
    fn m(&self, _s: String) {}
}

pub struct Wr<T>(pub T);

impl<T> Tr for Wr<T> {
    fn m(&self, s: String) {
        sink(&s); // @sink(rv-generic-impl)
    }
}

pub fn generic_impl() {
    let s = source(); // @source(rv-generic-impl)
    Wr(1u8).m(s);
}

// (c) a generic function used as a value
fn ident<T>(x: T) -> T {
    x
}

pub fn generic_as_value() {
    let s = source(); // @source(rv-generic-value)
    let f = ident::<String>;
    sink(&f(s)); // @sink(rv-generic-value)
}
