use crate::{sink, source};

// `Fn` and `FnMut` are called through `&self` (`Fn::call(&f, ..)`, `FnMut::call_mut(&mut f, ..)`): the
// receiver of the call is the callee's own parameter, held in a cell, not the parameter itself. The call
// is matched to every closure of the type, and what the callers gave is told apart by the guards of the
// dispatch (RUST-13; they work only when the receiver is a parameter), so two callers of one helper
// should not see each other's closures. This is the acceptance of ADR-047 (draft 2): every `@fp` here is
// to go. Each form has a signature of its own (`u8`, `u16`, `u32`, `u64`), so that the closures of the
// other files are not its candidates and do not change the counts.

// the result of the callback is returned
fn call_fn(s: String, f: impl Fn(String, u8) -> String) -> String {
    f(s, 1)
}

// the callback is given the value and holds the sink
fn fn_sink_with(s: String, f: impl Fn(String, u16)) {
    f(s, 1);
}

// the same with FnMut (`mut f`: the call takes `&mut f`)
fn call_fn_mut(s: String, mut f: impl FnMut(String, u32) -> String) -> String {
    f(s, 1)
}

fn fn_mut_sink_with(s: String, mut f: impl FnMut(String, u64)) {
    f(s, 1);
}

// the closure returns a constant: the result is clean (the identity of the other caller is not this one's)
pub fn fn_ret_clean() {
    let a = source(); // @source(fnself-clean)
    let r = call_fn(a.clone(), |_x, _n| String::from("fixed"));
    sink(&r); // @sink(none)
    sink(&a); // @sink(fnself-clean)
}

pub fn fn_ret_id() {
    let b = source(); // @source(fnself-id)
    let r = call_fn(b, |x, _n| x);
    sink(&r); // @sink(fnself-id) @nosink(fnself-clean)
}

pub fn fn_sink_a() {
    let a = source(); // @source(fnself-a)
    fn_sink_with(a, |x, _n| sink(&x)); // @sink(fnself-a) @nosink(fnself-b)
}

pub fn fn_sink_b() {
    let b = source(); // @source(fnself-b)
    fn_sink_with(b, |x, _n| sink(&x)); // @sink(fnself-b) @nosink(fnself-a)
}

pub fn fn_mut_ret_clean() {
    let a = source(); // @source(fnmutself-clean)
    let r = call_fn_mut(a.clone(), |_x, _n| String::from("fixed"));
    sink(&r); // @sink(none)
    sink(&a); // @sink(fnmutself-clean)
}

pub fn fn_mut_ret_id() {
    let b = source(); // @source(fnmutself-id)
    let r = call_fn_mut(b, |x, _n| x);
    sink(&r); // @sink(fnmutself-id) @nosink(fnmutself-clean)
}

pub fn fn_mut_sink_a() {
    let a = source(); // @source(fnmutself-a)
    fn_mut_sink_with(a, |x, _n| sink(&x)); // @sink(fnmutself-a) @nosink(fnmutself-b)
}

pub fn fn_mut_sink_b() {
    let b = source(); // @source(fnmutself-b)
    fn_mut_sink_with(b, |x, _n| sink(&x)); // @sink(fnmutself-b) @nosink(fnmutself-a)
}

// the callback is a reference (`&impl Fn`): the receiver of the call is the parameter itself, which is the
// shape `dispatchGuards` accepts, and it is reported wrongly all the same: the candidates here are closures,
// which are not methods (`!fn.Recv`), so `makeDispatchGuards` gives no guard. So RUST-13 is not only that the
// receiver is a cell. Signature of its own (`i8`).
fn call_fn_ref(s: String, f: &impl Fn(String, i8) -> String) -> String {
    f(s, 1)
}

pub fn fn_ref_clean() {
    let a = source(); // @source(fnref-clean)
    let r = call_fn_ref(a.clone(), &|_x, _n| String::from("fixed"));
    sink(&r); // @sink(none) @fp
    sink(&a); // @sink(fnref-clean)
}

pub fn fn_ref_id() {
    let b = source(); // @source(fnref-id)
    let r = call_fn_ref(b, &|x, _n| x);
    sink(&r); // @sink(fnref-id) @nosink(fnref-clean)
}
