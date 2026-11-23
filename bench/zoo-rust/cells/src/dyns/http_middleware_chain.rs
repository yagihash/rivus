// @cell(F/dyn/http-middleware-chain)
// @shape: handlers are chained (the outer one calls next.serve()); the value is a field of the inner handler
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/http-middleware-chain/vuln", vuln);
    m::case("F/dyn/http-middleware-chain/fixed", fixed);
    m::case("F/dyn/http-middleware-chain/decoy", decoy);
}

trait Handler {
    fn serve(&self);
}

struct Logging {
    next: Box<dyn Handler>,
}

impl Handler for Logging {
    fn serve(&self) {
        self.next.serve()
    }
}

struct Inner {
    v: String,
}

impl Handler for Inner {
    fn serve(&self) {
        m::sink_cmd(&self.v); // @sink(dyn-http-middleware-chain-vuln) @noisy(1) @ref(DYN-IMPLS)
    }
}

fn vuln() { // @vuln
    let h = Logging { next: Box::new(Inner { v: m::src("dyn-http-middleware-chain-vuln") }) }; // @source(dyn-http-middleware-chain-vuln)
    h.serve();
}

struct LoggingFixed {
    next: Box<dyn Handler>,
}

impl Handler for LoggingFixed {
    fn serve(&self) {
        self.next.serve()
    }
}

struct InnerFixed {
    v: String,
}

impl Handler for InnerFixed {
    fn serve(&self) {
        m::sink_cmd(&self.v); // @sink(none) @fp @ref(DYN-IMPLS,PARITY)
    }
}

fn fixed() { // @fixed
    let h = LoggingFixed { next: Box::new(InnerFixed { v: m::clean_sql(&m::clean_cmd(&m::src("dyn-http-middleware-chain-fixed"))) }) };
    h.serve();
}

struct LoggingDecoy {
    next: Box<dyn Handler>,
}

impl Handler for LoggingDecoy {
    fn serve(&self) {
        self.next.serve()
    }
}

struct InnerDecoy {
    a: String,
    b: String,
}

impl Handler for InnerDecoy {
    fn serve(&self) {
        m::sink_cmd(&self.b); // @sink(none) @fp @nosink(dyn-http-middleware-chain-decoy) @fp @ref(DYN-IMPLS,PARITY)
        m::sink_sql(&self.a); // @sink(dyn-http-middleware-chain-decoy) @noisy(1) @ref(DYN-IMPLS)
    }
}

fn decoy() { // @decoy
    let h = LoggingDecoy { next: Box::new(InnerDecoy { a: m::src("dyn-http-middleware-chain-decoy"), b: "constant".to_string() }) }; // @source(dyn-http-middleware-chain-decoy)
    h.serve();
}
