// @cell(F/conv/buffer-writestring)
// @shape: write_str writes the value into a String (std::fmt::Write), and the String is the sink (the callee has no body in the analysis: a rule for it)
use std::fmt::Write;
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/buffer-writestring/vuln", vuln);
    m::case("F/conv/buffer-writestring/fixed", fixed);
    m::case("F/conv/buffer-writestring/decoy", decoy);
}

fn vuln() { // @vuln
    let mut b = String::new();
    b.write_str(&m::src("conv-buffer-writestring-vuln")).unwrap(); // @source(conv-buffer-writestring-vuln) @miss(unmodeled_library_call) @ref(CORE-K)
    m::sink_cmd(&b); // @sink(conv-buffer-writestring-vuln)
}

fn fixed() { // @fixed
    let mut b = String::new();
    b.write_str(&m::clean_cmd(&m::src("conv-buffer-writestring-fixed"))).unwrap();
    m::sink_cmd(&b); // @sink(none)
}

fn decoy() { // @decoy
    let mut a = String::new();
    let mut b = String::new();
    a.write_str(&m::src("conv-buffer-writestring-decoy")).unwrap(); // @source(conv-buffer-writestring-decoy) @miss(unmodeled_library_call) @ref(CORE-K)
    b.write_str("constant").unwrap();
    m::sink_cmd(&b); // @sink(none) @nosink(conv-buffer-writestring-decoy)
    m::sink_sql(&a); // @sink(conv-buffer-writestring-decoy)
}
