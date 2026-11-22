// @cell(F/conv/unsafe-string)
// @shape: a String / &str is made of the bytes of a value without a check (String::from_utf8_unchecked, str::from_utf8_unchecked of a slice made from raw parts): the result is made of the bytes
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/unsafe-string/vuln", vuln);
    m::case("F/conv/unsafe-string/fixed", fixed);
    m::case("F/conv/unsafe-string/decoy", decoy);
}

fn vuln() { // @vuln
    let b = m::src("conv-unsafe-string-vuln").into_bytes(); // @source(conv-unsafe-string-vuln)
    let s = unsafe { String::from_utf8_unchecked(b) };
    m::sink_cmd(&s); // @sink(conv-unsafe-string-vuln)
}

fn fixed() { // @fixed
    let b = m::clean_cmd(&m::src("conv-unsafe-string-fixed")).into_bytes();
    let s = unsafe { std::str::from_utf8_unchecked(std::slice::from_raw_parts(b.as_ptr(), b.len())) };
    m::sink_cmd(s); // @sink(none)
}

fn decoy() { // @decoy
    let a = m::src("conv-unsafe-string-decoy").into_bytes(); // @source(conv-unsafe-string-decoy)
    let c = b"constant".to_vec();
    let sc = unsafe { String::from_utf8_unchecked(c) };
    m::sink_cmd(&sc); // @sink(none) @nosink(conv-unsafe-string-decoy)
    let sa = unsafe { std::str::from_utf8_unchecked(std::slice::from_raw_parts(a.as_ptr(), a.len())) };
    m::sink_sql(sa); // @sink(conv-unsafe-string-decoy)
}
