// @cell(F/state/global-struct-copy)
// @shape: a function copies a static that is a struct, whole (clone), and reads a field of the copy; the decoy reads another field
use std::sync::Mutex;
use zoo_mark as m;

pub fn register() {
    m::case("F/state/global-struct-copy/vuln", vuln);
    m::case("F/state/global-struct-copy/fixed", fixed);
    m::case("F/state/global-struct-copy/decoy", decoy);
}

struct Conf {
    a: String,
    b: String,
}

impl Conf {
    fn copy(&self) -> Conf {
        Conf { a: self.a.clone(), b: self.b.clone() }
    }
}

static G_VULN: Mutex<Conf> = Mutex::new(Conf { a: String::new(), b: String::new() });

fn read_vuln() {
    let c = G_VULN.lock().unwrap().copy();
    m::sink_cmd(&c.a); // @sink(state-global-struct-copy-vuln)
}

fn vuln() { // @vuln
    G_VULN.lock().unwrap().a = m::src("state-global-struct-copy-vuln"); // @source(state-global-struct-copy-vuln)
    read_vuln();
}

static G_FIXED: Mutex<Conf> = Mutex::new(Conf { a: String::new(), b: String::new() });

fn read_fixed() {
    let c = G_FIXED.lock().unwrap().copy();
    m::sink_cmd(&c.a); // @sink(none)
}

fn fixed() { // @fixed
    G_FIXED.lock().unwrap().a = m::clean_cmd(&m::src("state-global-struct-copy-fixed"));
    read_fixed();
}

static G_DECOY: Mutex<Conf> = Mutex::new(Conf { a: String::new(), b: String::new() });

fn read_decoy() {
    let c = G_DECOY.lock().unwrap().copy();
    m::sink_cmd(&c.b); // @sink(none) @fp @nosink(state-global-struct-copy-decoy) @fp @ref(PARITY)
    m::sink_sql(&c.a); // @sink(state-global-struct-copy-decoy)
}

fn decoy() { // @decoy
    G_DECOY.lock().unwrap().a = m::src("state-global-struct-copy-decoy"); // @source(state-global-struct-copy-decoy)
    G_DECOY.lock().unwrap().b = "constant".to_string();
    read_decoy();
}
