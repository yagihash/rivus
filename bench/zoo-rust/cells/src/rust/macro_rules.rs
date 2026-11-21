// @cell(F/rust/macro-rules)
// @shape: the value goes through a macro_rules! macro (one that returns its argument, one that expands into the sink call)
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/macro-rules/vuln", vuln);
    m::case("F/rust/macro-rules/fixed", fixed);
    m::case("F/rust/macro-rules/decoy", decoy);
    m::case("F/rust/macro-rules/vuln2", vuln2);
}

macro_rules! pass {
    ($e:expr) => {
        $e
    };
}

fn vuln() { // @vuln
    let v = pass!(m::src("rust-macro-rules-vuln")); // @source(rust-macro-rules-vuln)
    m::sink_cmd(&v); // @sink(rust-macro-rules-vuln)
}

macro_rules! emit {
    ($e:expr) => {
        m::sink_cmd(&$e) // @noisy(1) @ref(R-35)
    };
}

fn vuln2() { // @vuln
    let v = m::src("rust-macro-rules-vuln2"); // @source(rust-macro-rules-vuln2) @miss(unmodeled_library_call) @ref(R-35)
    emit!(v); // @sink(rust-macro-rules-vuln2)
}

fn fixed() { // @fixed
    let v = pass!(m::clean_cmd(&m::src("rust-macro-rules-fixed")));
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let a = pass!(m::src("rust-macro-rules-decoy")); // @source(rust-macro-rules-decoy)
    let b = pass!("constant".to_string());
    m::sink_cmd(&b); // @sink(none) @nosink(rust-macro-rules-decoy)
    m::sink_sql(&a); // @sink(rust-macro-rules-decoy)
}
