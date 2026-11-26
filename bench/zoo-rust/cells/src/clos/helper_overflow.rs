// @cell(F/clos/helper-overflow)
// @shape: ten different closures are passed to one helper that calls its function argument (more than eight candidates)
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/helper-overflow/vuln", vuln);
    m::case("F/clos/helper-overflow/vuln2", vuln2);
    m::case("F/clos/helper-overflow/vuln3", vuln3);
    m::case("F/clos/helper-overflow/vuln4", vuln4);
    m::case("F/clos/helper-overflow/vuln5", vuln5);
    m::case("F/clos/helper-overflow/vuln6", vuln6);
    m::case("F/clos/helper-overflow/vuln7", vuln7);
    m::case("F/clos/helper-overflow/vuln8", vuln8);
    m::case("F/clos/helper-overflow/vuln9", vuln9);
    m::case("F/clos/helper-overflow/vuln10", vuln10);
    m::case("F/clos/helper-overflow/fixed", fixed);
    m::case("F/clos/helper-overflow/decoy", decoy);
}

fn transform(s: String, f: impl Fn(String) -> String) -> String {
    f(s)
}

fn vuln() { // @vuln
    let a = m::src("clos-helper-overflow-vuln"); // @source(clos-helper-overflow-vuln)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln)
        x + "1"
    });
}

fn vuln2() { // @vuln
    let a = m::src("clos-helper-overflow-vuln2"); // @source(clos-helper-overflow-vuln2)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln2)
        x + "2"
    });
}

fn vuln3() { // @vuln
    let a = m::src("clos-helper-overflow-vuln3"); // @source(clos-helper-overflow-vuln3)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln3)
        x + "3"
    });
}

fn vuln4() { // @vuln
    let a = m::src("clos-helper-overflow-vuln4"); // @source(clos-helper-overflow-vuln4)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln4)
        x + "4"
    });
}

fn vuln5() { // @vuln
    let a = m::src("clos-helper-overflow-vuln5"); // @source(clos-helper-overflow-vuln5)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln5)
        x + "5"
    });
}

fn vuln6() { // @vuln
    let a = m::src("clos-helper-overflow-vuln6"); // @source(clos-helper-overflow-vuln6)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln6)
        x + "6"
    });
}

fn vuln7() { // @vuln
    let a = m::src("clos-helper-overflow-vuln7"); // @source(clos-helper-overflow-vuln7)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln7)
        x + "7"
    });
}

fn vuln8() { // @vuln
    let a = m::src("clos-helper-overflow-vuln8"); // @source(clos-helper-overflow-vuln8)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln8)
        x + "8"
    });
}

fn vuln9() { // @vuln
    let a = m::src("clos-helper-overflow-vuln9"); // @source(clos-helper-overflow-vuln9)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln9)
        x + "9"
    });
}

fn vuln10() { // @vuln
    let a = m::src("clos-helper-overflow-vuln10"); // @source(clos-helper-overflow-vuln10)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-vuln10)
        x + "10"
    });
}

fn fixed() { // @fixed
    let a = m::clean_cmd(&m::src("clos-helper-overflow-fixed"));
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(none)
        x + "f"
    });
}

fn decoy() { // @decoy
    let a = m::src("clos-helper-overflow-decoy"); // @source(clos-helper-overflow-decoy)
    transform("constant".to_string(), |x| {
        m::sink_cmd(&x); // @sink(none) @nosink(clos-helper-overflow-decoy)
        x + "d"
    });
    m::sink_sql(&a); // @sink(clos-helper-overflow-decoy)
}
