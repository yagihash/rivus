// A helper in another crate: the closure is given from the crate that uses it.
pub fn with_in<T, R>(value: T, f: impl FnOnce(T) -> R) -> R {
    f(value)
}

pub fn each<T>(items: Vec<T>, mut f: impl FnMut(T)) {
    for i in items {
        f(i);
    }
}

pub struct Hub {
    subs: Vec<Box<dyn Fn(&str)>>,
}

impl Hub {
    pub fn new() -> Hub {
        Hub { subs: Vec::new() }
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
