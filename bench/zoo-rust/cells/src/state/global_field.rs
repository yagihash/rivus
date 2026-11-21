// @cell(F/state/global-field)
// @shape: a function writes one field of a static that is a struct, and another function reads it; the decoy reads another field
use std::sync::Mutex;
use zoo_mark as m;

pub fn register() {
    m::case("F/state/global-field/vuln", vuln);
    m::case("F/state/global-field/fixed", fixed);
    m::case("F/state/global-field/decoy", decoy);
}

struct Conf {
    a: String,
    b: String,
}

static G_VULN: Mutex<Conf> = Mutex::new(Conf { a: String::new(), b: String::new() });

fn read_vuln() {
    m::sink_cmd(&G_VULN.lock().unwrap().a); // @sink(state-global-field-vuln)
}

fn vuln() { // @vuln
    G_VULN.lock().unwrap().a = m::src("state-global-field-vuln"); // @source(state-global-field-vuln)
    read_vuln();
}

static G_FIXED: Mutex<Conf> = Mutex::new(Conf { a: String::new(), b: String::new() });

fn read_fixed() {
    m::sink_cmd(&G_FIXED.lock().unwrap().a); // @sink(none)
}

fn fixed() { // @fixed
    G_FIXED.lock().unwrap().a = m::clean_cmd(&m::src("state-global-field-fixed"));
    read_fixed();
}

static G_DECOY: Mutex<Conf> = Mutex::new(Conf { a: String::new(), b: String::new() });

fn read_decoy_b() {
    m::sink_cmd(&G_DECOY.lock().unwrap().b); // @sink(none) @fp @nosink(state-global-field-decoy) @fp @ref(PARITY)
}

fn read_decoy_a() {
    m::sink_sql(&G_DECOY.lock().unwrap().a); // @sink(state-global-field-decoy)
}

fn decoy() { // @decoy
    G_DECOY.lock().unwrap().a = m::src("state-global-field-decoy"); // @source(state-global-field-decoy)
    G_DECOY.lock().unwrap().b = "constant".to_string();
    read_decoy_b();
    read_decoy_a();
}
