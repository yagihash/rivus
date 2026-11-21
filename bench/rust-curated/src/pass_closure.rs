use crate::helpers::*;
use crate::{sink, source};

// A closure given to a helper, by two callers: the value of one must not reach
// the sink of the other.

pub fn ret_a() {
    let a = source(); // @source(ret-a)
    let r = transform(a, |x| x);
    sink(&r); // @sink(ret-a) @nosink(ret-b)
}

pub fn ret_b() {
    let b = source(); // @source(ret-b)
    let r = transform(b, |x| x);
    sink(&r); // @sink(ret-b) @nosink(ret-a)
}

// the closure returns a constant of its own; the helper's result is clean
pub fn ret_clean() {
    let c = source(); // @source(ret-clean)
    let r = transform(c.clone(), |_x| String::from("fixed"));
    sink(&r); // @sink(none) @fp
    sink(&c); // @sink(ret-clean)
}

// the callback is given the value: it reaches the sink inside the closure
pub fn arg_a() {
    let a = source(); // @source(arg-a)
    fire_and_forget(a, |x| sink(&x)); // @sink(arg-a) @nosink(arg-b) @noisy(28)
}

pub fn arg_b() {
    let b = source(); // @source(arg-b)
    fire_and_forget(b, |x| sink(&x)); // @sink(arg-b) @nosink(arg-a) @noisy(28)
}

// the helper calls the callback with a constant; the caller's source stays in the caller
pub fn prod_a() {
    let a = source(); // @source(prod-a)
    produce(|x| {
        sink(&x); // @sink(none) @fp
    });
    sink(&a); // @sink(prod-a)
}

// the closure captures the value, and the helper calls it
pub fn run_a() {
    let a = source(); // @source(run-a)
    run(|| sink(&a)); // @sink(run-a) @nosink(run-b)
}

pub fn run_b() {
    let b = source(); // @source(run-b)
    run(|| sink(&b)); // @sink(run-b) @nosink(run-a)
}

// the closure takes what it captures (move)
pub fn run_move() {
    let a = source(); // @source(run-move)
    run(move || sink(&a)); // @sink(run-move)
}

// a block body, and the value is changed on the way
pub fn block_body() {
    let a = source(); // @source(block)
    let r = transform(a, |x| {
        let y = x.trim().to_string();
        y
    });
    sink(&r); // @sink(block)
}

// the closure returns the value on one branch only
pub fn branchy(ok: bool) {
    let a = source(); // @source(branchy)
    let r = transform(a, |x| if ok { x } else { String::from("no") });
    sink(&r); // @sink(branchy)
}

// FnMut with a counter, called twice: the second call gets the result of the first
pub fn twice_a() {
    let a = source(); // @source(twice-a)
    let mut n = 0;
    let r = twice(a, |x| {
        n += 1;
        x
    });
    sink(&r); // @sink(twice-a) @nosink(twice-b)
}

pub fn twice_b() {
    let b = source(); // @source(twice-b)
    let r = twice(b, |x| x);
    sink(&r); // @sink(twice-b) @nosink(twice-a)
}

// the callback is a `&dyn Fn` and a `Box<dyn Fn>`
pub fn dyn_ref_a() {
    let a = source(); // @source(dynref-a)
    let r = call_dyn(a, &|x| x);
    sink(&r); // @sink(dynref-a) @nosink(dynref-b)
}

pub fn dyn_ref_b() {
    let b = source(); // @source(dynref-b)
    let r = call_dyn(b, &|x| x);
    sink(&r); // @sink(dynref-b) @nosink(dynref-a)
}

pub fn boxed_a() {
    let a = source(); // @source(boxed-a)
    let r = call_boxed(a, Box::new(|x| x));
    sink(&r); // @sink(boxed-a) @nosink(boxed-b)
}

pub fn boxed_b() {
    let b = source(); // @source(boxed-b)
    let r = call_boxed(b, Box::new(|x| x));
    sink(&r); // @sink(boxed-b) @nosink(boxed-a)
}

// the sink is the callback's
pub fn sink_with_a() {
    let a = source(); // @source(sinkwith-a)
    sink_with(a, |s| sink(s)); // @sink(sinkwith-a) @nosink(sinkwith-b)
}

pub fn sink_with_b() {
    let b = source(); // @source(sinkwith-b)
    sink_with(b, |s| sink(s)); // @sink(sinkwith-b) @nosink(sinkwith-a)
}
