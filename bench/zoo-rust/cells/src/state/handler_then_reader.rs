// @cell(F/state/handler-then-reader)
// @shape: one entry point (a handler) writes a static from its parameter, and another entry point reads it
use std::sync::Mutex;
use zoo_mark as m;

pub fn register() {
    m::case("F/state/handler-then-reader/vuln", vuln);
    m::case("F/state/handler-then-reader/fixed", fixed);
    m::case("F/state/handler-then-reader/decoy", decoy);
}

static LAST_VULN: Mutex<String> = Mutex::new(String::new());

fn write_vuln(i: m::In) { // @source(state-handler-then-reader-vuln)
    *LAST_VULN.lock().unwrap() = i.value;
}

fn read_vuln() {
    m::sink_cmd(&LAST_VULN.lock().unwrap()); // @sink(state-handler-then-reader-vuln)
}

fn vuln() { // @vuln
    write_vuln(m::In::new("state-handler-then-reader-vuln"));
    read_vuln();
}

static LAST_FIXED: Mutex<String> = Mutex::new(String::new());

fn write_fixed(i: m::In) {
    *LAST_FIXED.lock().unwrap() = m::clean_cmd(&i.value);
}

fn read_fixed() {
    m::sink_cmd(&LAST_FIXED.lock().unwrap()); // @sink(none)
}

fn fixed() { // @fixed
    write_fixed(m::In::new("state-handler-then-reader-fixed"));
    read_fixed();
}

static LAST_DECOY_A: Mutex<String> = Mutex::new(String::new());
static LAST_DECOY_B: Mutex<String> = Mutex::new(String::new());

fn write_decoy_a(i: m::In) { // @source(state-handler-then-reader-decoy)
    *LAST_DECOY_A.lock().unwrap() = i.value;
}

fn write_decoy_b(s: &str) {
    *LAST_DECOY_B.lock().unwrap() = s.to_string();
}

fn read_decoy() {
    m::sink_cmd(&LAST_DECOY_B.lock().unwrap()); // @sink(none) @nosink(state-handler-then-reader-decoy)
    m::sink_sql(&LAST_DECOY_A.lock().unwrap()); // @sink(state-handler-then-reader-decoy)
}

fn decoy() { // @decoy
    write_decoy_a(m::In::new("state-handler-then-reader-decoy"));
    write_decoy_b("constant");
    read_decoy();
}
