// @cell(F/dyn/sink-in-method)
// @shape: a constructor stores the value in a struct, and the sink is in a method of the struct
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/sink-in-method/vuln", vuln);
    m::case("F/dyn/sink-in-method/fixed", fixed);
    m::case("F/dyn/sink-in-method/decoy", decoy);
}

struct Runner {
    cmd: String,
}

impl Runner {
    fn new(c: String) -> Runner {
        Runner { cmd: c }
    }
    fn exec(&self) {
        m::sink_cmd(&self.cmd); // @sink(dyn-sink-in-method-vuln)
    }
}

fn vuln() { // @vuln
    Runner::new(m::src("dyn-sink-in-method-vuln")).exec(); // @source(dyn-sink-in-method-vuln)
}

struct RunnerFixed {
    cmd: String,
}

impl RunnerFixed {
    fn exec(&self) {
        m::sink_cmd(&self.cmd); // @sink(none)
    }
}

fn fixed() { // @fixed
    RunnerFixed { cmd: m::clean_cmd(&m::src("dyn-sink-in-method-fixed")) }.exec();
}

struct RunnerDecoy {
    a: String,
    b: String,
}

impl RunnerDecoy {
    fn exec(&self) {
        m::sink_cmd(&self.b); // @sink(none) @nosink(dyn-sink-in-method-decoy)
        m::sink_sql(&self.a); // @sink(dyn-sink-in-method-decoy)
    }
}

fn decoy() { // @decoy
    let r = RunnerDecoy { a: m::src("dyn-sink-in-method-decoy"), b: "constant".to_string() }; // @source(dyn-sink-in-method-decoy)
    r.exec();
}
