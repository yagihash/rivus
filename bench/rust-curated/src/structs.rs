use crate::helpers::*;
use crate::{sink, source};

// Callbacks kept in structs and called by methods, and methods that are
// given the receiver's own method.

pub struct Handler {
    on: Box<dyn Fn(String) -> String>,
}

impl Handler {
    pub fn new(on: Box<dyn Fn(String) -> String>) -> Handler {
        Handler { on }
    }
    pub fn handle(&self, s: String) -> String {
        (self.on)(s)
    }
}

pub fn field_function_a() {
    let a = source(); // @source(field-a)
    let h = Handler::new(Box::new(|x| x));
    sink(&h.handle(a)); // @sink(field-a) @nosink(field-b)
}

pub fn field_function_b() {
    let b = source(); // @source(field-b)
    let h = Handler::new(Box::new(|x| x));
    sink(&h.handle(b)); // @sink(field-b) @nosink(field-a)
}

// the generic struct of the helpers
pub fn runner_a() {
    let a = source(); // @source(runner-a)
    let r = Runner::new(|x| x);
    sink(&r.go(a)); // @sink(runner-a) @nosink(runner-b)
}

pub fn runner_b() {
    let b = source(); // @source(runner-b)
    let r = Runner::new(|x| x);
    sink(&r.go(b)); // @sink(runner-b) @nosink(runner-a)
}

// the closure captures the value, and the struct keeps it
pub fn runner_capture() {
    let a = source(); // @source(runner-cap)
    let r = Runner::new(move |_x| a.clone());
    sink(&r.go(String::from("const"))); // @sink(runner-cap)
}

// an emitter: callbacks are registered, and called later
pub struct Emitter {
    subs: Vec<Box<dyn Fn(&str)>>,
}

impl Emitter {
    pub fn new() -> Emitter {
        Emitter { subs: Vec::new() }
    }
    pub fn on(&mut self, f: Box<dyn Fn(&str)>) {
        self.subs.push(f);
    }
    pub fn emit(&self, s: &str) {
        for f in &self.subs {
            f(s);
        }
    }
}

pub fn emitter_flow() {
    let mut e = Emitter::new();
    e.on(Box::new(|s| sink(s))); // @sink(emit)
    e.emit(&source()); // @source(emit) @miss(silent)
}

// two emitters: the subscriber of one is not called by the other
pub fn emitter_two() {
    let mut e1 = Emitter::new();
    let mut e2 = Emitter::new();
    e1.on(Box::new(|s| sink(s))); // @sink(emit-two)
    e2.on(Box::new(|s| sink(s))); // @sink(none)
    e1.emit(&source()); // @source(emit-two) @miss(silent)
    e2.emit("const");
}

// a method that gives its own method to a helper (a closure over &self)
pub struct Pipeline {
    prefix: String,
}

impl Pipeline {
    pub fn new(prefix: String) -> Pipeline {
        Pipeline { prefix }
    }
    fn stamp(&self, s: String) -> String {
        format!("{}{}", self.prefix, s)
    }
    pub fn run(&self, s: String) -> String {
        transform(s, |x| self.stamp(x))
    }
    pub fn run_each(&self, items: Vec<String>) -> Vec<String> {
        items.into_iter().map(|x| self.stamp(x)).collect()
    }
}

pub fn method_in_closure() {
    let a = source(); // @source(method-closure)
    sink(&Pipeline::new(String::from("p")).run(a)); // @sink(method-closure)
}

pub fn method_in_closure_prefix() {
    let p = source(); // @source(method-prefix)
    sink(&Pipeline::new(p).run(String::from("const"))); // @sink(method-prefix)
}

pub fn method_in_map() {
    let a = source(); // @source(method-map)
    let out = Pipeline::new(String::from("p")).run_each(vec![a]);
    sink(&out[0]); // @sink(method-map)
}

// a trait with a callback parameter, and two implementations
pub trait Stage {
    fn apply(&self, s: String, f: &dyn Fn(String) -> String) -> String;
}

pub struct Plain;
pub struct Fixed;

impl Stage for Plain {
    fn apply(&self, s: String, f: &dyn Fn(String) -> String) -> String {
        f(s)
    }
}

impl Stage for Fixed {
    fn apply(&self, _s: String, f: &dyn Fn(String) -> String) -> String {
        f(String::from("fixed"))
    }
}

pub fn trait_callback_plain() {
    let a = source(); // @source(stage-plain)
    sink(&Plain.apply(a, &|x| x)); // @sink(stage-plain)
}

pub fn trait_callback_fixed() {
    let a = source(); // @source(stage-fixed)
    sink(&Fixed.apply(a.clone(), &|x| x)); // @sink(none)
    sink(&a); // @sink(stage-fixed)
}

pub fn trait_object_callback(s: &dyn Stage) {
    let a = source(); // @source(stage-dyn)
    sink(&s.apply(a, &|x| x)); // @sink(stage-dyn)
}
