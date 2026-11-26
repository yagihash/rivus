// @cell(F/dart/dio-get)
// @shape: a value goes into the URL of reqwest::blocking::get (the function of the crate): the SSRF sink of the built-in rows. The crate of the zoo sends nothing and records the URL
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/dio-get/vuln", vuln);
    m::case("F/dart/dio-get/fixed", fixed);
    m::case("F/dart/dio-get/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("dart-dio-get-vuln"); // @source(dart-dio-get-vuln)
    let _ = reqwest::blocking::get(format!("http://example.invalid/?q={v}")); // @sink(dart-dio-get-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_url(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("dart-dio-get-fixed"))))));
    let _ = reqwest::blocking::get(format!("http://example.invalid/?q={v}")); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dart-dio-get-decoy"); // @source(dart-dio-get-decoy)
    let _ = reqwest::blocking::Client::new().post("http://example.invalid/").body(&v).send(); // @sink(none) @nosink(dart-dio-get-decoy)
    m::sink_sql(&v); // @sink(dart-dio-get-decoy)
}
