use std::fs;
use std::process::Command;

// The models of the standard library: the sources and the sinks are found
// without a rule of the sample.

pub fn from_args() {
    let a = std::env::args().nth(1).unwrap(); // @source(std-args)
    let _ = Command::new(a).output(); // @sink(std-args)
}

pub fn from_var() -> String {
    let v = std::env::args().nth(2).unwrap(); // @source(std-var)
    fs::read_to_string(&v).unwrap() // @sink(std-var)
}

pub fn formatted() {
    let a = std::env::args().nth(1).unwrap(); // @source(std-format)
    let cmd = format!("ls {}", a);
    let _ = Command::new(cmd).output(); // @sink(std-format)
}

pub fn literal() {
    let _ = Command::new("ls").output(); // @sink(none)
}

pub fn pushed() {
    let mut s = String::from("ls ");
    s.push_str(&std::env::args().nth(1).unwrap()); // @source(std-push)
    let _ = Command::new(&s).output(); // @sink(std-push)
}

pub fn rwlock_value() {
    let a = std::env::args().nth(1).unwrap(); // @source(std-rwlock)
    let l = std::sync::RwLock::new(a);
    let s = l.read().unwrap().clone();
    let _ = Command::new(&s).output(); // @sink(std-rwlock)
}
