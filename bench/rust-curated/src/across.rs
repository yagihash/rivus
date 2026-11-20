use crate::{sink, source};

// The helper is in another crate.

pub fn across_a() {
    let a = source(); // @source(across-a)
    let r = shared::with_in(a, |x| x);
    sink(&r); // @sink(across-a) @nosink(across-b)
}

pub fn across_b() {
    let b = source(); // @source(across-b)
    let r = shared::with_in(b, |x| x);
    sink(&r); // @sink(across-b) @nosink(across-a)
}

pub fn across_each() {
    let items = vec![source()]; // @source(across-each) @miss(silent)
    shared::each(items, |x| sink(&x)); // @sink(across-each) @noisy(42)
}

pub fn across_hub() {
    let mut h = shared::Hub::new();
    h.on(Box::new(|s| sink(s))); // @sink(across-hub) @noisy(5)
    h.emit(&source()); // @source(across-hub) @miss(silent)
}

pub fn across_capture() {
    let mut out = String::new();
    let t = source(); // @source(across-cap) @miss(silent)
    shared::each(vec![1], |_i| out = t.clone());
    sink(&out); // @sink(across-cap)
}
