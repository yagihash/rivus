use crate::{sink, source};
use std::collections::HashMap;
use std::sync::{LazyLock, RwLock};

// A value that one function stores in a global table and another reads back:
// the alert targets of a server (the request makes one, a timer calls it).

pub struct Target {
    pub endpoint: String,
}

pub struct Configs {
    map: RwLock<HashMap<String, Target>>,
}

impl Configs {
    pub fn update(&self, t: Target) {
        self.map.write().unwrap().insert(t.endpoint.clone(), t);
    }

    pub fn get(&self, id: &str) -> Option<String> {
        self.map.read().unwrap().get(id).map(|t| t.endpoint.clone())
    }
}

pub static TARGETS: LazyLock<Configs> = LazyLock::new(|| Configs { map: RwLock::new(HashMap::new()) });

pub fn register() {
    let s = source(); // @source(glob-table)
    TARGETS.update(Target { endpoint: s });
}

pub fn fire() {
    if let Some(e) = TARGETS.get("x") {
        sink(&e); // @sink(glob-table)
    }
}

// the same, with simpler containers: which link of the chain is missing

pub struct One {
    m: std::sync::Mutex<String>,
}
pub static ONE: LazyLock<One> = LazyLock::new(|| One { m: std::sync::Mutex::new(String::new()) });

pub fn one_set() {
    let s = source(); // @source(glob-one)
    *ONE.m.lock().unwrap() = s;
}

pub fn one_get() {
    let e = ONE.m.lock().unwrap().clone();
    sink(&e); // @sink(glob-one)
}

pub struct Many {
    v: RwLock<Vec<String>>,
}
pub static MANY: LazyLock<Many> = LazyLock::new(|| Many { v: RwLock::new(Vec::new()) });

pub fn many_set() {
    let s = source(); // @source(glob-vec)
    MANY.v.write().unwrap().push(s);
}

pub fn many_get() {
    let e = MANY.v.read().unwrap()[0].clone();
    sink(&e); // @sink(glob-vec)
}
