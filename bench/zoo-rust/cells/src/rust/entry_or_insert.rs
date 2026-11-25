// @cell(F/rust/entry-or-insert)
// @shape: a value is written through the `&mut` that `entry(..).or_insert(..)` or `or_insert_with(..)` gives, and is read from the map again
use std::collections::HashMap;
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/entry-or-insert/vuln", vuln);
    m::case("F/rust/entry-or-insert/vuln2", vuln2);
    m::case("F/rust/entry-or-insert/fixed", fixed);
    m::case("F/rust/entry-or-insert/decoy", decoy);
}

fn vuln() { // @vuln
    let mut t: HashMap<String, String> = HashMap::new();
    *t.entry("a".to_string()).or_insert(String::new()) = m::src("rust-entry-or-insert-vuln"); // @source(rust-entry-or-insert-vuln)
    m::sink_cmd(&t["a"]); // @sink(rust-entry-or-insert-vuln)
}

fn vuln2() { // @vuln
    let mut t: HashMap<String, String> = HashMap::new();
    t.entry("a".to_string()).or_insert_with(String::new).push_str(&m::src("rust-entry-or-insert-vuln2")); // @source(rust-entry-or-insert-vuln2)
    m::sink_cmd(&t["a"]); // @sink(rust-entry-or-insert-vuln2)
}

fn fixed() { // @fixed
    let mut t: HashMap<String, String> = HashMap::new();
    *t.entry("a".to_string()).or_insert(String::new()) = m::clean_cmd(&m::src("rust-entry-or-insert-fixed"));
    m::sink_cmd(&t["a"]); // @sink(none)
}

fn decoy() { // @decoy
    let mut t: HashMap<String, String> = HashMap::new();
    let mut u: HashMap<String, String> = HashMap::new();
    u.insert("a".to_string(), String::new());
    *t.entry("a".to_string()).or_insert(String::new()) = m::src("rust-entry-or-insert-decoy"); // @source(rust-entry-or-insert-decoy)
    m::sink_cmd(&u["a"]); // @sink(none)
    m::sink_sql(&t["a"]); // @sink(rust-entry-or-insert-decoy)
}
