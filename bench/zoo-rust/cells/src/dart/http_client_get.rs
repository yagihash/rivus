// @cell(F/dart/http-client-get)
// @shape: a value goes into the URL of Client::get of a client that is a variable: the SSRF sink of the built-in rows. The crate of the zoo sends nothing and records the URL
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/http-client-get/vuln", vuln);
    m::case("F/dart/http-client-get/fixed", fixed);
    m::case("F/dart/http-client-get/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("dart-http-client-get-vuln"); // @source(dart-http-client-get-vuln)
    let c = reqwest::blocking::Client::new();
    let _ = c.get(format!("http://example.invalid/?q={v}")).send(); // @sink(dart-http-client-get-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_url(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("dart-http-client-get-fixed"))))));
    let c = reqwest::blocking::Client::new();
    let _ = c.get(format!("http://example.invalid/?q={v}")).send(); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dart-http-client-get-decoy"); // @source(dart-http-client-get-decoy)
    let c = reqwest::blocking::Client::new();
    let _ = c.get("http://example.invalid/").header("x", &v).send(); // @sink(none) @nosink(dart-http-client-get-decoy)
    m::sink_sql(&v); // @sink(dart-http-client-get-decoy)
}
