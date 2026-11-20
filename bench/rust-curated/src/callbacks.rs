use crate::{sink, source};

// Closures that capture a value and are called where the engine does not see the call: a closure that
// a function returns, a closure in a closure, a closure that is never called (C-7,
// docs/DESIGN-capture.md). A closure that captures is not a `make_closure`: it is a struct of its
// environment (an `alloc` of the closure's type and a `field_store` for each captured value), and
// the closure is a function of that struct. What is found here is found by the call `Fn::call`,
// whose candidates are all the closures of that parameter list: so each closure here has two parameters of
// types of their own (two of `i16`, `i128`, `u128`, `f32`), as in fnself.rs, or it would be a candidate of
// the calls of the other files (a closure with no parameter, as the one of `thread::spawn`, is
// of that kind: it is not here for that reason).

fn make_sink(s: String) -> impl Fn(i16, i16) {
    move |_n, _m| {
        sink(&s); // @sink(returned)
    }
}

// the closure is made in a function, called in another
pub fn returned() {
    let a = source(); // @source(returned)
    let f = make_sink(a);
    f(0, 0);
}

fn run_i64(f: impl FnOnce(i128, i128)) {
    f(0, 0)
}

// the closure in the closure captures what the outer one captured
pub fn nested_cap() {
    let a = source(); // @source(nested-cap)
    run_i64(|_n, _k| {
        run_i64(|_m, _j| {
            sink(&a); // @sink(nested-cap)
        })
    });
}

// the closure is made and never called
pub fn never_called() {
    let a = source(); // @source(never-called)
    let _f = |_n: u128, _m: u128| {
        sink(&a); // @sink(none)
    };
    sink(&a); // @sink(never-called)
}

fn produce_f32(f: impl Fn(String, f32)) {
    f(String::from("const"), 1.0)
}

// the closure uses its parameter (what the helper gives), not the captured value
pub fn param_clean() {
    let a = source(); // @source(param-clean)
    produce_f32(|x: String, _n: f32| {
        sink(&x); // @sink(none)
    });
    sink(&a); // @sink(param-clean)
}
