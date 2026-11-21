use crate::helpers::*;
use crate::{sink, source};

// A closure that writes a variable it captured, and what comes out of it.

pub fn write_a() {
    let mut out = String::new();
    let t = source(); // @source(write-a)
    run(|| {
        out = t;
    });
    sink(&out); // @sink(write-a) @nosink(write-b)
}

pub fn write_b() {
    let mut out = String::new();
    let t = source(); // @source(write-b)
    run(|| {
        out = t;
    });
    sink(&out); // @sink(write-b) @nosink(write-a)
}

// the closure writes a constant: nothing comes out of it
pub fn write_const() {
    let mut out = String::new();
    let t = source(); // @source(write-const)
    run(|| {
        out = String::from("fixed");
    });
    sink(&out); // @sink(none)
    sink(&t); // @sink(write-const)
}

// the callback writes what the helper gave it into the captured variable
pub fn write_param() {
    let mut out = String::new();
    let t = source(); // @source(write-param)
    fire_and_forget(t, |x| {
        out = x;
    });
    sink(&out); // @sink(write-param)
}

// a list that the closure pushes to
pub fn push_to_list() {
    let mut items: Vec<String> = Vec::new();
    let t = source(); // @source(push-list)
    run(|| items.push(t));
    sink(&items[0]); // @sink(push-list)
}

// a relay through the parameter: a helper that gives the closure to a second helper
fn relay_param(s: String, f: impl Fn(String) -> String) -> String {
    transform(s, f)
}

pub fn relay_param_a() {
    let a = source(); // @source(relay-p-a)
    sink(&relay_param(a, |x| x)); // @sink(relay-p-a) @nosink(relay-p-b)
}

pub fn relay_param_b() {
    let b = source(); // @source(relay-p-b)
    sink(&relay_param(b, |x| x)); // @sink(relay-p-b) @nosink(relay-p-a)
}

// a relay through the capture: the closure given to the second helper
// captures the callback of the first
fn relay_capture(f: impl Fn()) {
    run(|| f());
}

pub fn relay_cap_a() {
    let a = source(); // @source(relay-c-a)
    relay_capture(|| sink(&a)); // @sink(relay-c-a) @nosink(relay-c-b)
}

pub fn relay_cap_b() {
    let b = source(); // @source(relay-c-b)
    relay_capture(|| sink(&b)); // @sink(relay-c-b) @nosink(relay-c-a)
}

// both: the parameter, then the capture
fn relay_both(s: String, f: impl Fn(String) -> String) -> String {
    transform(s, |x| f(x))
}

pub fn relay_both_a() {
    let a = source(); // @source(relay-b-a)
    sink(&relay_both(a, |x| x)); // @sink(relay-b-a) @nosink(relay-b-b)
}

pub fn relay_both_b() {
    let b = source(); // @source(relay-b-b)
    sink(&relay_both(b, |x| x)); // @sink(relay-b-b) @nosink(relay-b-a)
}

// a closure returned by a function holds a captured value
pub fn holder_a() {
    let a = source(); // @source(holder-a)
    let get = holder(a);
    sink(&get()); // @sink(holder-a) @nosink(holder-b)
}

pub fn holder_b() {
    let b = source(); // @source(holder-b)
    let get = holder(b);
    sink(&get()); // @sink(holder-b) @nosink(holder-a)
}

pub fn holder_boxed_a() {
    let a = source(); // @source(holderbox-a)
    let get = holder_boxed(a);
    sink(&get()); // @sink(holderbox-a) @nosink(holderbox-b)
}

pub fn holder_boxed_b() {
    let b = source(); // @source(holderbox-b)
    let get = holder_boxed(b);
    sink(&get()); // @sink(holderbox-b) @nosink(holderbox-a)
}

// a closure called in the function that made it (no helper)
pub fn local_closure() {
    let a = source(); // @source(local)
    let f = || a.clone();
    sink(&f()); // @sink(local)
}

// a closure that captures by reference and the variable is changed after it is made
pub fn late_write() {
    let mut v = String::from("const");
    let a = source(); // @source(late)
    let mut set = |x: String| v = x;
    set(a);
    sink(&v); // @sink(late)
}

// A closure that writes one of two variables it captured by `&mut`: the other one is not changed, and a
// closure that captured another variable in the same position (the first field of its environment) does not
// see the write.
pub fn write_one_of_two() {
    let mut out = String::new();
    let mut other = String::new();
    let t = source(); // @source(write-one-of-two)
    run(|| {
        out = t;
        other.push_str("const");
    });
    sink(&out); // @sink(write-one-of-two)
    sink(&other); // @sink(none)
}

pub fn write_one_of_two_reader() {
    let kept = String::from("const");
    run(|| sink(&kept)); // @sink(none)
}

// a struct that holds a `&mut` and writes through it, with no closure (the same pointer in a field)
struct Slot<'a> {
    out: &'a mut String,
    t: String,
}

pub fn struct_mut_field() {
    let mut out = String::new();
    let t = source(); // @source(struct-mut-field)
    let e = Slot { out: &mut out, t };
    *e.out = e.t;
    sink(&out); // @sink(struct-mut-field)
}

fn write_slot(e: Slot) {
    *e.out = e.t;
}

pub fn struct_mut_field_helper() {
    let mut out = String::new();
    let t = source(); // @source(struct-mut-field-helper)
    write_slot(Slot { out: &mut out, t });
    sink(&out); // @sink(struct-mut-field-helper)
}

pub fn struct_mut_field_const() {
    let mut out = String::new();
    let t = source(); // @source(struct-mut-field-const)
    write_slot(Slot { out: &mut out, t: String::from("const") });
    sink(&out); // @sink(none)
    sink(&t); // @sink(struct-mut-field-const)
}
