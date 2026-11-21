// @cell(F/dyn/method-value)
// @shape: a method is taken as a function value (Printer::run) and called with the receiver and the argument; the decoy swaps what is tainted
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/method-value/vuln", vuln);
    m::case("F/dyn/method-value/fixed", fixed);
    m::case("F/dyn/method-value/decoy", decoy);
}

struct Printer {
    prefix: String,
}

impl Printer {
    fn run(&self, arg: &str) {
        m::sink_cmd(arg); // @sink(dyn-method-value-vuln)
    }
}

fn vuln() { // @vuln
    let p = Printer { prefix: "constant".to_string() };
    let f = Printer::run;
    f(&p, &m::src("dyn-method-value-vuln")); // @source(dyn-method-value-vuln)
}

struct PrinterFixed {
    prefix: String,
}

impl PrinterFixed {
    fn run(&self, arg: &str) {
        m::sink_cmd(arg); // @sink(none)
    }
}

fn fixed() { // @fixed
    let p = PrinterFixed { prefix: "constant".to_string() };
    let f = PrinterFixed::run;
    f(&p, &m::clean_cmd(&m::src("dyn-method-value-fixed")));
}

struct PrinterDecoy {
    prefix: String,
}

impl PrinterDecoy {
    fn run(&self, arg: &str) {
        m::sink_cmd(arg); // @sink(none) @nosink(dyn-method-value-decoy)
        m::sink_sql(&self.prefix); // @sink(dyn-method-value-decoy)
    }
}

fn decoy() { // @decoy
    let p = PrinterDecoy { prefix: m::src("dyn-method-value-decoy") }; // @source(dyn-method-value-decoy)
    let f = PrinterDecoy::run;
    f(&p, "constant");
}
