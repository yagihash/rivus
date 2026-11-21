// @cell(F/rust/mut-out-param)
// @shape: a function writes its result through a &mut parameter (push_str into a String, push into a Vec) and the caller reads the variable
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/mut-out-param/vuln", vuln);
    m::case("F/rust/mut-out-param/fixed", fixed);
    m::case("F/rust/mut-out-param/decoy", decoy);
    m::case("F/rust/mut-out-param/vuln2", vuln2);
}

fn fill(out: &mut String, s: &str) {
    out.push_str(s);
}

fn vuln() { // @vuln
    let mut out = String::new();
    fill(&mut out, &m::src("rust-mut-out-param-vuln")); // @source(rust-mut-out-param-vuln)
    m::sink_cmd(&out); // @sink(rust-mut-out-param-vuln)
}

fn collect_into(out: &mut Vec<String>, s: String) {
    out.push(s);
}

fn vuln2() { // @vuln
    let mut out = Vec::new();
    collect_into(&mut out, m::src("rust-mut-out-param-vuln2")); // @source(rust-mut-out-param-vuln2)
    m::sink_cmd(&out[0]); // @sink(rust-mut-out-param-vuln2)
}

fn fixed() { // @fixed
    let mut out = String::new();
    fill(&mut out, &m::clean_cmd(&m::src("rust-mut-out-param-fixed")));
    m::sink_cmd(&out); // @sink(none)
}

fn decoy() { // @decoy
    let mut a = String::new();
    let mut b = String::new();
    fill(&mut a, &m::src("rust-mut-out-param-decoy")); // @source(rust-mut-out-param-decoy)
    fill(&mut b, "constant");
    m::sink_cmd(&b); // @sink(none) @nosink(rust-mut-out-param-decoy)
    m::sink_sql(&a); // @sink(rust-mut-out-param-decoy)
}
