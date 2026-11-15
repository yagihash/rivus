use crate::{sink, source};
use std::collections::HashMap;

// Traits, generics, iterators, loops and `?`: the extractor must read them all,
// and the flows through them must be found.

pub trait Handler {
    fn handle(&self, s: &str);
}

pub struct Printer;

impl Handler for Printer {
    fn handle(&self, s: &str) {
        sink(s); // @sink(dyn)
    }
}

pub struct Quiet;

impl Handler for Quiet {
    fn handle(&self, _s: &str) {}
}

pub fn run(h: &dyn Handler, s: String) {
    h.handle(&s);
}

pub fn via_dyn() {
    let s = source(); // @source(dyn)
    run(&Printer, s);
}

pub fn generic<T: Handler>(h: T, s: String) {
    h.handle(&s);
}

pub fn via_loop() {
    let items = vec![source(), String::from("b")]; // @source(loop)
    for item in &items {
        sink(item); // @sink(loop)
    }
}

pub fn via_iterator() {
    let items = vec![source()]; // @source(iter)
    let upper: Vec<String> = items.iter().map(|s| s.to_uppercase()).collect();
    sink(&upper[0]); // @sink(iter)
}

pub fn parse(input: &str) -> Result<String, std::num::ParseIntError> {
    let n: i32 = input.parse()?;
    Ok(n.to_string())
}

pub fn via_question() -> Result<(), std::num::ParseIntError> {
    let s = source(); // @source(question)
    let t = parse(&s)?;
    sink(&t); // @sink(question)
    Ok(())
}

pub fn via_map() {
    let mut m: HashMap<String, String> = HashMap::new();
    m.insert(String::from("k"), source()); // @source(map)
    if let Some(v) = m.get("k") {
        sink(v); // @sink(map)
    }
}

pub struct Wrapper {
    inner: String,
}

impl Wrapper {
    pub fn new(inner: String) -> Wrapper {
        Wrapper { inner }
    }

    pub fn get(&self) -> &str {
        &self.inner
    }
}

pub fn via_method() {
    let w = Wrapper::new(source()); // @source(method)
    sink(w.get()); // @sink(method)
}

pub fn via_while() {
    let mut s = String::new();
    let mut n = 0;
    while n < 3 {
        s = source(); // @source(while)
        n += 1;
    }
    sink(&s); // @sink(while)
}

pub fn via_match(k: u8) {
    let s = source(); // @source(match)
    let t = match k {
        0 => s,
        _ => String::from("safe"),
    };
    sink(&t); // @sink(match)
}
