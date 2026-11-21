use crate::{sink, source};
use std::future::Future;

// The candidates of a call of a closure (`Fn::call`) narrowed to the closures that flow to the receiver
// (narrow.rs), when the receiver is held by a coroutine: an `async` block that captures it, or an `async fn` that
// takes it. The shapes are those of an async runtime that spawns the future (`tokio::spawn(async move { f(x) })`),
// with a `spawn` of our own that takes the future, so that no runtime is needed. Each form has types of its own, so
// the closures of the other forms are not its candidates.

fn spawn<F: Future<Output = ()>>(_f: F) {}

// (1) an `async move` block that captures a clone of the closure, and calls it: the receiver is a field of the
// coroutine. Two callers; the closure of the other one is not the receiver (`@fp` goes)
struct Qa(String);

fn run_block(f: impl Fn(&Qa) + Clone + Send + 'static, s: Qa) {
    let g = f.clone();
    spawn(async move { g(&s) });
}

pub fn block_real() {
    let a = Qa(source()); // @source(async-block)
    run_block(
        |x| {
            sink(&x.0); // @sink(async-block)
        },
        a,
    );
}

pub fn block_other() {
    run_block(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        Qa(String::from("constant")),
    );
}

pub fn block_extra() {
    let g = |x: &Qa| {
        sink(&x.0); // @sink(none) @fp
    };
    g(&Qa(String::from("constant")));
}

// (2) an `async fn` that takes the closure and calls it
struct Qb(String);

async fn run_async_fn(f: impl Fn(&Qb), s: &Qb) {
    f(s)
}

pub fn async_fn_real() {
    let a = Qb(source()); // @source(async-fn)
    spawn(run_async_fn(
        |x| {
            sink(&x.0); // @sink(async-fn)
        },
        &a,
    ));
}

pub fn async_fn_other() {
    spawn(run_async_fn(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        &Qb(String::from("constant")),
    ));
}

pub fn async_fn_extra() {
    let g = |x: &Qb| {
        sink(&x.0); // @sink(none) @fp
    };
    g(&Qb(String::from("constant")));
}

// (3) the shape of a helper that calls one function on each of several items, in a task each
// (`for_each_live_node`): an `async fn` with a closure parameter, cloned into an `async move` block in a loop
struct Qc(String);

async fn for_each_item(items: Vec<Qc>, f: impl Fn(&Qc) + Clone + Send + 'static) {
    for item in items {
        let g = f.clone();
        spawn(async move { g(&item) });
    }
}

pub fn each_real() {
    let a = Qc(source()); // @source(async-each)
    spawn(for_each_item(
        vec![a],
        |x| {
            sink(&x.0); // @sink(async-each)
        },
    ));
}

pub fn each_other() {
    spawn(for_each_item(
        vec![Qc(String::from("constant"))],
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
    ));
}

pub fn each_extra() {
    let g = |x: &Qc| {
        sink(&x.0); // @sink(none) @fp
    };
    g(&Qc(String::from("constant")));
}

// (4) two origins, one of them cannot be followed (a closure that captured a value, returned by a function: the
// result of a call): every candidate stays, and the flow through the one that is returned is found
struct Qd(String);

fn make_unknown_qd(tag: String) -> impl Fn(&Qd) + Clone + Send + 'static {
    move |x| {
        sink(&x.0); // @sink(async-unknown)
        sink(&tag);
    }
}

fn run_block_qd(f: impl Fn(&Qd) + Clone + Send + 'static, s: Qd) {
    let g = f.clone();
    spawn(async move { g(&s) });
}

pub fn unknown_known() {
    run_block_qd(
        |x| {
            sink(&x.0); // @sink(none) @fp
        },
        Qd(String::from("constant")),
    );
}

pub fn unknown_unknown() {
    let a = Qd(source()); // @source(async-unknown)
    run_block_qd(make_unknown_qd(String::from("tag")), a);
}
