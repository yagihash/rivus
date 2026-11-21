// @cell(F/dyn/sanitizer-own-impl)
// @shape: the project implements a trait whose method the rules call a sanitizer, and the implementation reaches a sink itself
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/sanitizer-own-impl/vuln", vuln);
    m::case("F/dyn/sanitizer-own-impl/fixed", fixed);
    m::case("F/dyn/sanitizer-own-impl/decoy", decoy);
}

/// `clean` is declared a sanitizer by rules.yaml: what it returns is clean.
trait Cleaner {
    fn clean(&self, s: &str) -> String;
}

struct ShellCleaner;

impl Cleaner for ShellCleaner {
    fn clean(&self, s: &str) -> String {
        m::sink_cmd(s); // @sink(dyn-sanitizer-own-impl-vuln) @noisy(2) @ref(DYN-IMPLS,PARITY)
        m::clean_cmd(s)
    }
}

fn vuln() { // @vuln
    let c: Box<dyn Cleaner> = Box::new(ShellCleaner);
    let _ = c.clean(&m::src("dyn-sanitizer-own-impl-vuln")); // @source(dyn-sanitizer-own-impl-vuln) @ref(PARITY)
}

struct FixedCleaner;

impl Cleaner for FixedCleaner {
    fn clean(&self, s: &str) -> String {
        let clean = m::clean_cmd(s);
        m::sink_cmd(&clean); // @sink(none)
        clean
    }
}

fn fixed() { // @fixed
    let c: Box<dyn Cleaner> = Box::new(FixedCleaner);
    let _ = c.clean(&m::src("dyn-sanitizer-own-impl-fixed"));
}

struct QuietCleaner;

impl Cleaner for QuietCleaner {
    fn clean(&self, s: &str) -> String {
        m::sink_cmd(s); // @sink(none) @fp @nosink(dyn-sanitizer-own-impl-decoy) @fp @ref(DYN-IMPLS,PARITY)
        m::clean_cmd(s)
    }
}

struct LoudCleaner;

impl Cleaner for LoudCleaner {
    fn clean(&self, s: &str) -> String {
        m::sink_sql(s); // @sink(dyn-sanitizer-own-impl-decoy) @noisy(2) @ref(DYN-IMPLS,PARITY)
        m::clean_sql(s)
    }
}

fn decoy() { // @decoy
    let quiet: Box<dyn Cleaner> = Box::new(QuietCleaner);
    let loud: Box<dyn Cleaner> = Box::new(LoudCleaner);
    let _ = quiet.clean("constant");
    let _ = loud.clean(&m::src("dyn-sanitizer-own-impl-decoy")); // @source(dyn-sanitizer-own-impl-decoy)
}
