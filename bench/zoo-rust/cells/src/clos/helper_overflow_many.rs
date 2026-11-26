// @cell(F/clos/helper-overflow-many)
// @shape: thirty-four different closures are passed to one helper that calls its function argument (more candidates than the 32 that rivus keeps, ADR-050)
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/helper-overflow-many/vuln", vuln);
    m::case("F/clos/helper-overflow-many/vuln2", vuln2);
    m::case("F/clos/helper-overflow-many/vuln3", vuln3);
    m::case("F/clos/helper-overflow-many/vuln4", vuln4);
    m::case("F/clos/helper-overflow-many/vuln5", vuln5);
    m::case("F/clos/helper-overflow-many/vuln6", vuln6);
    m::case("F/clos/helper-overflow-many/vuln7", vuln7);
    m::case("F/clos/helper-overflow-many/vuln8", vuln8);
    m::case("F/clos/helper-overflow-many/vuln9", vuln9);
    m::case("F/clos/helper-overflow-many/vuln10", vuln10);
    m::case("F/clos/helper-overflow-many/vuln11", vuln11);
    m::case("F/clos/helper-overflow-many/vuln12", vuln12);
    m::case("F/clos/helper-overflow-many/vuln13", vuln13);
    m::case("F/clos/helper-overflow-many/vuln14", vuln14);
    m::case("F/clos/helper-overflow-many/vuln15", vuln15);
    m::case("F/clos/helper-overflow-many/vuln16", vuln16);
    m::case("F/clos/helper-overflow-many/vuln17", vuln17);
    m::case("F/clos/helper-overflow-many/vuln18", vuln18);
    m::case("F/clos/helper-overflow-many/vuln19", vuln19);
    m::case("F/clos/helper-overflow-many/vuln20", vuln20);
    m::case("F/clos/helper-overflow-many/vuln21", vuln21);
    m::case("F/clos/helper-overflow-many/vuln22", vuln22);
    m::case("F/clos/helper-overflow-many/vuln23", vuln23);
    m::case("F/clos/helper-overflow-many/vuln24", vuln24);
    m::case("F/clos/helper-overflow-many/vuln25", vuln25);
    m::case("F/clos/helper-overflow-many/vuln26", vuln26);
    m::case("F/clos/helper-overflow-many/vuln27", vuln27);
    m::case("F/clos/helper-overflow-many/vuln28", vuln28);
    m::case("F/clos/helper-overflow-many/vuln29", vuln29);
    m::case("F/clos/helper-overflow-many/vuln30", vuln30);
    m::case("F/clos/helper-overflow-many/vuln31", vuln31);
    m::case("F/clos/helper-overflow-many/vuln32", vuln32);
    m::case("F/clos/helper-overflow-many/vuln33", vuln33);
    m::case("F/clos/helper-overflow-many/vuln34", vuln34);
    m::case("F/clos/helper-overflow-many/fixed", fixed);
    m::case("F/clos/helper-overflow-many/decoy", decoy);
}

fn transform(s: String, f: impl Fn(String) -> String) -> String {
    f(s)
}

fn vuln() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln"); // @source(clos-helper-overflow-many-vuln)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln)
        x + "1"
    });
}

fn vuln2() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln2"); // @source(clos-helper-overflow-many-vuln2)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln2)
        x + "2"
    });
}

fn vuln3() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln3"); // @source(clos-helper-overflow-many-vuln3)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln3)
        x + "3"
    });
}

fn vuln4() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln4"); // @source(clos-helper-overflow-many-vuln4)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln4)
        x + "4"
    });
}

fn vuln5() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln5"); // @source(clos-helper-overflow-many-vuln5)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln5)
        x + "5"
    });
}

fn vuln6() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln6"); // @source(clos-helper-overflow-many-vuln6)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln6)
        x + "6"
    });
}

fn vuln7() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln7"); // @source(clos-helper-overflow-many-vuln7)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln7)
        x + "7"
    });
}

fn vuln8() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln8"); // @source(clos-helper-overflow-many-vuln8)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln8)
        x + "8"
    });
}

fn vuln9() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln9"); // @source(clos-helper-overflow-many-vuln9)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln9)
        x + "9"
    });
}

fn vuln10() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln10"); // @source(clos-helper-overflow-many-vuln10)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln10)
        x + "10"
    });
}

fn vuln11() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln11"); // @source(clos-helper-overflow-many-vuln11)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln11)
        x + "11"
    });
}

fn vuln12() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln12"); // @source(clos-helper-overflow-many-vuln12)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln12)
        x + "12"
    });
}

fn vuln13() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln13"); // @source(clos-helper-overflow-many-vuln13)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln13)
        x + "13"
    });
}

fn vuln14() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln14"); // @source(clos-helper-overflow-many-vuln14)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln14)
        x + "14"
    });
}

fn vuln15() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln15"); // @source(clos-helper-overflow-many-vuln15)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln15)
        x + "15"
    });
}

fn vuln16() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln16"); // @source(clos-helper-overflow-many-vuln16)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln16)
        x + "16"
    });
}

fn vuln17() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln17"); // @source(clos-helper-overflow-many-vuln17)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln17)
        x + "17"
    });
}

fn vuln18() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln18"); // @source(clos-helper-overflow-many-vuln18)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln18)
        x + "18"
    });
}

fn vuln19() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln19"); // @source(clos-helper-overflow-many-vuln19)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln19)
        x + "19"
    });
}

fn vuln20() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln20"); // @source(clos-helper-overflow-many-vuln20)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln20)
        x + "20"
    });
}

fn vuln21() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln21"); // @source(clos-helper-overflow-many-vuln21)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln21)
        x + "21"
    });
}

fn vuln22() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln22"); // @source(clos-helper-overflow-many-vuln22)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln22)
        x + "22"
    });
}

fn vuln23() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln23"); // @source(clos-helper-overflow-many-vuln23)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln23)
        x + "23"
    });
}

fn vuln24() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln24"); // @source(clos-helper-overflow-many-vuln24)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln24)
        x + "24"
    });
}

fn vuln25() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln25"); // @source(clos-helper-overflow-many-vuln25)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln25)
        x + "25"
    });
}

fn vuln26() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln26"); // @source(clos-helper-overflow-many-vuln26)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln26)
        x + "26"
    });
}

fn vuln27() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln27"); // @source(clos-helper-overflow-many-vuln27)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln27)
        x + "27"
    });
}

fn vuln28() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln28"); // @source(clos-helper-overflow-many-vuln28)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln28)
        x + "28"
    });
}

fn vuln29() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln29"); // @source(clos-helper-overflow-many-vuln29)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln29)
        x + "29"
    });
}

fn vuln30() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln30"); // @source(clos-helper-overflow-many-vuln30)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln30)
        x + "30"
    });
}

fn vuln31() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln31"); // @source(clos-helper-overflow-many-vuln31)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln31)
        x + "31"
    });
}

fn vuln32() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln32"); // @source(clos-helper-overflow-many-vuln32)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln32)
        x + "32"
    });
}

fn vuln33() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln33"); // @source(clos-helper-overflow-many-vuln33)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln33)
        x + "33"
    });
}

fn vuln34() { // @vuln
    let a = m::src("clos-helper-overflow-many-vuln34"); // @source(clos-helper-overflow-many-vuln34)
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(clos-helper-overflow-many-vuln34)
        x + "34"
    });
}

fn fixed() { // @fixed
    let a = m::clean_cmd(&m::src("clos-helper-overflow-many-fixed"));
    transform(a, |x| {
        m::sink_cmd(&x); // @sink(none)
        x + "f"
    });
}

fn decoy() { // @decoy
    let a = m::src("clos-helper-overflow-many-decoy"); // @source(clos-helper-overflow-many-decoy)
    transform("constant".to_string(), |x| {
        m::sink_cmd(&x); // @sink(none) @nosink(clos-helper-overflow-many-decoy)
        x + "d"
    });
    m::sink_sql(&a); // @sink(clos-helper-overflow-many-decoy)
}
