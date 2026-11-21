// @cell(F/rust/arc-mutex)
// @shape: a value is written into an Arc<Mutex<String>> by a spawned thread (joined), and read by the spawner
use std::sync::{Arc, Mutex};
use std::thread;
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/arc-mutex/vuln", vuln);
    m::case("F/rust/arc-mutex/fixed", fixed);
    m::case("F/rust/arc-mutex/decoy", decoy);
}

fn vuln() { // @vuln
    let shared = Arc::new(Mutex::new(String::new()));
    let s2 = Arc::clone(&shared);
    let v = m::src("rust-arc-mutex-vuln"); // @source(rust-arc-mutex-vuln) @miss(unmodeled_library_call) @ref(CAP)
    thread::spawn(move || {
        *s2.lock().unwrap() = v;
    })
    .join()
    .unwrap();
    m::sink_cmd(&shared.lock().unwrap()); // @sink(rust-arc-mutex-vuln)
}

fn fixed() { // @fixed
    let shared = Arc::new(Mutex::new(String::new()));
    let s2 = Arc::clone(&shared);
    let v = m::clean_cmd(&m::src("rust-arc-mutex-fixed"));
    thread::spawn(move || {
        *s2.lock().unwrap() = v;
    })
    .join()
    .unwrap();
    m::sink_cmd(&shared.lock().unwrap()); // @sink(none)
}

fn decoy() { // @decoy
    let a = Arc::new(Mutex::new(String::new()));
    let b = Arc::new(Mutex::new(String::new()));
    let (a2, b2) = (Arc::clone(&a), Arc::clone(&b));
    let v = m::src("rust-arc-mutex-decoy"); // @source(rust-arc-mutex-decoy) @miss(unmodeled_library_call) @ref(CAP)
    thread::spawn(move || {
        *a2.lock().unwrap() = v;
        *b2.lock().unwrap() = "constant".to_string();
    })
    .join()
    .unwrap();
    m::sink_cmd(&b.lock().unwrap()); // @sink(none) @nosink(rust-arc-mutex-decoy)
    m::sink_sql(&a.lock().unwrap()); // @sink(rust-arc-mutex-decoy)
}
