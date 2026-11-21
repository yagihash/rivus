// @cell(F/dyn/sink-behind-own-interface)
// @shape: the project calls its own trait (a Querier, a Repository), the call site is the trait's method, and the sink is in an implementation
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/sink-behind-own-interface/vuln", vuln);
    m::case("F/dyn/sink-behind-own-interface/fixed", fixed);
    m::case("F/dyn/sink-behind-own-interface/decoy", decoy);
}

trait Querier {
    fn query(&self, sql: &str);
}

struct PgQuerier;

impl Querier for PgQuerier {
    fn query(&self, sql: &str) {
        m::sink_sql(sql); // @sink(dyn-sink-behind-own-interface-vuln)
    }
}

fn vuln() { // @vuln
    let q: Box<dyn Querier> = Box::new(PgQuerier);
    q.query(&m::src("dyn-sink-behind-own-interface-vuln")); // @source(dyn-sink-behind-own-interface-vuln)
}

trait QuerierFixed {
    fn query(&self, sql: &str);
}

struct PgQuerierFixed;

impl QuerierFixed for PgQuerierFixed {
    fn query(&self, sql: &str) {
        m::sink_sql(sql); // @sink(none)
    }
}

fn fixed() { // @fixed
    let q: Box<dyn QuerierFixed> = Box::new(PgQuerierFixed);
    q.query(&m::clean_sql(&m::src("dyn-sink-behind-own-interface-fixed")));
}

// the only implementation has no sink for what it is given
trait QuerierQuiet {
    fn query(&self, sql: &str);
}

struct QuietQuerier;

impl QuerierQuiet for QuietQuerier {
    fn query(&self, _sql: &str) {
        m::sink_sql("select 1"); // @sink(none) @nosink(dyn-sink-behind-own-interface-decoy)
    }
}

fn decoy() { // @decoy
    let q: Box<dyn QuerierQuiet> = Box::new(QuietQuerier);
    let v = m::src("dyn-sink-behind-own-interface-decoy"); // @source(dyn-sink-behind-own-interface-decoy)
    q.query(&v);
    m::sink_cmd(&v); // @sink(dyn-sink-behind-own-interface-decoy)
}
