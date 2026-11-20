use crate::helpers::*;
use crate::{sink, source};

// Closures inside closures, generic helpers, callbacks that take callbacks.

pub fn nested_a() {
    let a = source(); // @source(nest-a)
    let r = transform(String::from("const"), |_x| transform(a.clone(), |y| y));
    sink(&r); // @sink(nest-a) @nosink(nest-b)
}

pub fn nested_b() {
    let b = source(); // @source(nest-b)
    let r = transform(String::from("const"), |_x| transform(b.clone(), |y| y));
    sink(&r); // @sink(nest-b) @nosink(nest-a)
}

// the inner closure uses the parameter of the outer one
pub fn nested_param() {
    let a = source(); // @source(nest-param)
    let r = transform(a, |x| transform(x, |y| y));
    sink(&r); // @sink(nest-param)
}

// three levels
pub fn nested_three() {
    let a = source(); // @source(nest-three)
    let r = transform(a, |x| transform(x, |y| transform(y, |z| z)));
    sink(&r); // @sink(nest-three)
}

// the callback gets another callback
pub fn takes_callback_a() {
    let a = source(); // @source(takes-a)
    let r = via_inner(String::from("const"), |g| g(a.clone()));
    sink(&r); // @sink(takes-a) @nosink(takes-b)
}

pub fn takes_callback_b() {
    let b = source(); // @source(takes-b)
    let r = via_inner(String::from("const"), |g| g(b.clone()));
    sink(&r); // @sink(takes-b) @nosink(takes-a)
}

// the generic helper, two instantiations
pub fn generic_a() {
    let a = source(); // @source(gen-a)
    sink(&apply_to(a, |x| x)); // @sink(gen-a) @nosink(gen-b)
}

pub fn generic_b() {
    let b = source(); // @source(gen-b)
    sink(&apply_to(b, |x| x)); // @sink(gen-b) @nosink(gen-a)
}

pub fn generic_vec() {
    let t = source(); // @source(gen-vec)
    let v = apply_to(vec![t], |x| x);
    sink(&v[0]); // @sink(gen-vec)
}

pub fn generic_int() {
    let t = source(); // @source(gen-int)
    let n = apply_to(t.len(), |x| x + 1);
    sink(&n.to_string()); // @sink(none)
    sink(&t); // @sink(gen-int)
}

// the closure is chosen by a condition
pub fn choose(flag: bool) {
    let a = source(); // @source(choose)
    let f: Box<dyn Fn(String) -> String> = if flag {
        Box::new(|x| x)
    } else {
        Box::new(|_x| String::from("fixed"))
    };
    sink(&f(a)); // @sink(choose)
}

// the closure is stored in a variable first, and given to two helpers
pub fn shared_closure() {
    let a = source(); // @source(shared)
    let id = |x: String| x;
    sink(&transform(a, id)); // @sink(shared)
    sink(&transform(String::from("const"), id)); // @sink(none)
}

// an immediately invoked closure
pub fn iife() {
    let a = source(); // @source(iife)
    let r = (|| a.clone())();
    sink(&r); // @sink(iife)
}

// currying: a closure that returns a closure
pub fn curried() {
    let a = source(); // @source(curry)
    let add = |p: String| move |q: String| p.clone() + &q;
    sink(&add(a)(String::from("x"))); // @sink(curry)
}

// composition of two functions
fn compose(f: impl Fn(String) -> String, g: impl Fn(String) -> String) -> impl Fn(String) -> String {
    move |x| g(f(x))
}

pub fn composed_a() {
    let a = source(); // @source(compose-a)
    sink(&compose(|x| x, |y| y)(a)); // @sink(compose-a) @nosink(compose-b)
}

pub fn composed_b() {
    let b = source(); // @source(compose-b)
    sink(&compose(|x| x, |y| y)(b)); // @sink(compose-b) @nosink(compose-a)
}

// a recursive function that takes a callback
fn recur(s: String, n: u32, f: &dyn Fn(String) -> String) -> String {
    if n == 0 {
        f(s)
    } else {
        recur(f(s), n - 1, f)
    }
}

pub fn recursive_callback() {
    let a = source(); // @source(recur)
    sink(&recur(a, 3, &|x| x)); // @sink(recur)
}

// a callback called inside a closure given to another helper
pub fn callback_in_helper() {
    let a = source(); // @source(cb-in-helper)
    let r = transform(a, |x| call_dyn(x, &|y| y));
    sink(&r); // @sink(cb-in-helper)
}
