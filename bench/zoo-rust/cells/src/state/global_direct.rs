// @cell(F/state/global-direct)
// @shape: a function writes a static and another function reads it
use std::sync::Mutex;
use zoo_mark as m;

pub fn register() {
    m::case("F/state/global-direct/vuln", vuln);
    m::case("F/state/global-direct/fixed", fixed);
    m::case("F/state/global-direct/decoy", decoy);
}

static G_VULN: Mutex<String> = Mutex::new(String::new());

fn read_vuln() {
    m::sink_cmd(&G_VULN.lock().unwrap()); // @sink(state-global-direct-vuln)
}

fn vuln() { // @vuln
    *G_VULN.lock().unwrap() = m::src("state-global-direct-vuln"); // @source(state-global-direct-vuln)
    read_vuln();
}

static G_FIXED: Mutex<String> = Mutex::new(String::new());

fn read_fixed() {
    m::sink_cmd(&G_FIXED.lock().unwrap()); // @sink(none)
}

fn fixed() { // @fixed
    *G_FIXED.lock().unwrap() = m::clean_cmd(&m::src("state-global-direct-fixed"));
    read_fixed();
}

static G_DECOY_A: Mutex<String> = Mutex::new(String::new());
static G_DECOY_B: Mutex<String> = Mutex::new(String::new());

fn read_decoy() {
    m::sink_cmd(&G_DECOY_B.lock().unwrap()); // @sink(none) @nosink(state-global-direct-decoy)
    m::sink_sql(&G_DECOY_A.lock().unwrap()); // @sink(state-global-direct-decoy)
}

fn decoy() { // @decoy
    *G_DECOY_A.lock().unwrap() = m::src("state-global-direct-decoy"); // @source(state-global-direct-decoy)
    *G_DECOY_B.lock().unwrap() = "constant".to_string();
    read_decoy();
}
