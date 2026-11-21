// @cell(F/state/save-restore)
// @shape: a function saves a field of a static, sets it, calls the work and restores it; the decoy reads another field
use std::sync::Mutex;
use zoo_mark as m;

pub fn register() {
    m::case("F/state/save-restore/vuln", vuln);
    m::case("F/state/save-restore/fixed", fixed);
    m::case("F/state/save-restore/decoy", decoy);
}

struct State {
    a: String,
    b: String,
}

static S_VULN: Mutex<State> = Mutex::new(State { a: String::new(), b: String::new() });

fn read_vuln() {
    m::sink_cmd(&S_VULN.lock().unwrap().a); // @sink(state-save-restore-vuln)
}

fn vuln() { // @vuln
    let old = S_VULN.lock().unwrap().a.clone();
    S_VULN.lock().unwrap().a = m::src("state-save-restore-vuln"); // @source(state-save-restore-vuln)
    read_vuln();
    S_VULN.lock().unwrap().a = old;
}

static S_FIXED: Mutex<State> = Mutex::new(State { a: String::new(), b: String::new() });

fn read_fixed() {
    m::sink_cmd(&S_FIXED.lock().unwrap().a); // @sink(none)
}

fn fixed() { // @fixed
    let old = S_FIXED.lock().unwrap().a.clone();
    S_FIXED.lock().unwrap().a = m::clean_cmd(&m::src("state-save-restore-fixed"));
    read_fixed();
    S_FIXED.lock().unwrap().a = old;
}

static S_DECOY: Mutex<State> = Mutex::new(State { a: String::new(), b: String::new() });

fn read_decoy_b() {
    m::sink_cmd(&S_DECOY.lock().unwrap().b); // @sink(none) @fp @nosink(state-save-restore-decoy) @fp @ref(PARITY)
}

fn read_decoy_a() {
    m::sink_sql(&S_DECOY.lock().unwrap().a); // @sink(state-save-restore-decoy)
}

fn decoy() { // @decoy
    let old = S_DECOY.lock().unwrap().a.clone();
    S_DECOY.lock().unwrap().a = m::src("state-save-restore-decoy"); // @source(state-save-restore-decoy)
    S_DECOY.lock().unwrap().b = "constant".to_string();
    read_decoy_b();
    read_decoy_a();
    S_DECOY.lock().unwrap().a = old;
}
