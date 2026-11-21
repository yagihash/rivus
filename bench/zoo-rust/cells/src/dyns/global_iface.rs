// @cell(F/dyn/global-iface)
// @shape: a static holds a trait object that holds the value, and another function reads it
use std::sync::Mutex;
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/global-iface/vuln", vuln);
    m::case("F/dyn/global-iface/fixed", fixed);
    m::case("F/dyn/global-iface/decoy", decoy);
}

trait Getter {
    fn get(&self) -> String;
}

struct Bx {
    v: String,
}

impl Getter for Bx {
    fn get(&self) -> String {
        self.v.clone()
    }
}

type Slot = Mutex<Option<Box<dyn Getter + Send>>>;

static G_VULN: Slot = Mutex::new(None);

fn read_vuln() {
    m::sink_cmd(&G_VULN.lock().unwrap().as_ref().unwrap().get()); // @sink(dyn-global-iface-vuln)
}

fn vuln() { // @vuln
    *G_VULN.lock().unwrap() = Some(Box::new(Bx { v: m::src("dyn-global-iface-vuln") })); // @source(dyn-global-iface-vuln)
    read_vuln();
}

static G_FIXED: Slot = Mutex::new(None);

fn read_fixed() {
    m::sink_cmd(&G_FIXED.lock().unwrap().as_ref().unwrap().get()); // @sink(none)
}

fn fixed() { // @fixed
    *G_FIXED.lock().unwrap() = Some(Box::new(Bx { v: m::clean_cmd(&m::src("dyn-global-iface-fixed")) }));
    read_fixed();
}

static G_DECOY_A: Slot = Mutex::new(None);
static G_DECOY_B: Slot = Mutex::new(None);

fn read_decoy() {
    m::sink_cmd(&G_DECOY_B.lock().unwrap().as_ref().unwrap().get()); // @sink(none) @nosink(dyn-global-iface-decoy)
    m::sink_sql(&G_DECOY_A.lock().unwrap().as_ref().unwrap().get()); // @sink(dyn-global-iface-decoy)
}

fn decoy() { // @decoy
    *G_DECOY_A.lock().unwrap() = Some(Box::new(Bx { v: m::src("dyn-global-iface-decoy") })); // @source(dyn-global-iface-decoy)
    *G_DECOY_B.lock().unwrap() = Some(Box::new(Bx { v: "constant".to_string() }));
    read_decoy();
}
