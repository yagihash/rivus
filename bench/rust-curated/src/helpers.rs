// Helpers that take a function, shared by the callers of the other files.
use crate::sink;

// the callback gets the value, and what it returns is returned
pub fn transform(s: String, f: impl Fn(String) -> String) -> String {
    f(s)
}

// the same with a bound written as a where clause and a function pointer
pub fn transform_ptr(s: String, f: fn(String) -> String) -> String {
    f(s)
}

// the callback is called with a constant: what it holds is the caller's
pub fn produce(f: impl Fn(String)) {
    f(String::from("const"))
}

// the callback is called for the side effect
pub fn run(f: impl FnOnce()) {
    f()
}

// the callback is FnMut and called twice
pub fn twice(s: String, mut f: impl FnMut(String) -> String) -> String {
    let once = f(s);
    f(once)
}

// the helper is generic
pub fn apply_to<T>(value: T, f: impl Fn(T) -> T) -> T {
    f(value)
}

// the callback gets another callback
pub fn via_inner(s: String, outer: impl Fn(&dyn Fn(String) -> String) -> String) -> String {
    let tag = s.clone();
    outer(&move |x| format!("{}{}", x, tag))
}

// the callback is a trait object
pub fn call_dyn(s: String, f: &dyn Fn(String) -> String) -> String {
    f(s)
}

pub fn call_boxed(s: String, f: Box<dyn Fn(String) -> String>) -> String {
    f(s)
}

// a helper that calls the callback and drops the result
pub fn fire_and_forget(s: String, f: impl FnOnce(String)) {
    f(s);
}

// a helper that returns a closure that holds its argument
pub fn holder(s: String) -> impl Fn() -> String {
    move || s.clone()
}

pub fn holder_boxed(s: String) -> Box<dyn Fn() -> String> {
    Box::new(move || s.clone())
}

// the callback is kept in a field of the helper
pub struct Runner<F: Fn(String) -> String> {
    f: F,
}

impl<F: Fn(String) -> String> Runner<F> {
    pub fn new(f: F) -> Runner<F> {
        Runner { f }
    }
    pub fn go(&self, s: String) -> String {
        (self.f)(s)
    }
}

// a helper that calls the sink for the caller
pub fn sink_with(s: String, f: impl Fn(&str)) {
    f(&s);
}

pub fn into_sink(s: &str) {
    sink(s); // @sink(into-sink)
}
