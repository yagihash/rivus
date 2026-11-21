// @cell(F/dyn/sink-in-method-iface)
// @shape: a struct is held as a trait object, and the sink is in the method that the trait object calls
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/sink-in-method-iface/vuln", vuln);
    m::case("F/dyn/sink-in-method-iface/fixed", fixed);
    m::case("F/dyn/sink-in-method-iface/decoy", decoy);
}

trait Execer {
    fn exec(&self);
}

struct Runner {
    cmd: String,
}

impl Execer for Runner {
    fn exec(&self) {
        m::sink_cmd(&self.cmd); // @sink(dyn-sink-in-method-iface-vuln) @noisy(1) @ref(DYN-IMPLS)
    }
}

fn vuln() { // @vuln
    let e: Box<dyn Execer> = Box::new(Runner { cmd: m::src("dyn-sink-in-method-iface-vuln") }); // @source(dyn-sink-in-method-iface-vuln)
    e.exec();
}

struct RunnerFixed {
    cmd: String,
}

impl Execer for RunnerFixed {
    fn exec(&self) {
        m::sink_cmd(&self.cmd); // @sink(none) @fp @ref(DYN-IMPLS,PARITY)
    }
}

fn fixed() { // @fixed
    let e: Box<dyn Execer> = Box::new(RunnerFixed { cmd: m::clean_cmd(&m::src("dyn-sink-in-method-iface-fixed")) });
    e.exec();
}

struct RunnerDecoy {
    a: String,
    b: String,
}

impl Execer for RunnerDecoy {
    fn exec(&self) {
        m::sink_cmd(&self.b); // @sink(none) @fp @nosink(dyn-sink-in-method-iface-decoy) @fp @ref(DYN-IMPLS,PARITY)
        m::sink_sql(&self.a); // @sink(dyn-sink-in-method-iface-decoy) @noisy(1) @ref(DYN-IMPLS)
    }
}

fn decoy() { // @decoy
    let e: Box<dyn Execer> = Box::new(RunnerDecoy { a: m::src("dyn-sink-in-method-iface-decoy"), b: "constant".to_string() }); // @source(dyn-sink-in-method-iface-decoy)
    e.exec();
}
