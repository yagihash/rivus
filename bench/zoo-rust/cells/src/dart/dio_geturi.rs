// @cell(F/dart/dio-geturi)
// @shape: a value goes into the URL object made by Url::parse that Client::get takes: the SSRF sink of the built-in rows. The crate of the zoo sends nothing and records the URL
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/dio-geturi/vuln", vuln);
    m::case("F/dart/dio-geturi/fixed", fixed);
    m::case("F/dart/dio-geturi/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("dart-dio-geturi-vuln"); // @source(dart-dio-geturi-vuln)
    let c = reqwest::blocking::Client::new();
    let u = reqwest::Url::parse(&format!("http://example.invalid/?q={v}")).unwrap();
    let _ = c.get(u).send(); // @sink(dart-dio-geturi-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_url(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("dart-dio-geturi-fixed"))))));
    let c = reqwest::blocking::Client::new();
    let u = reqwest::Url::parse(&format!("http://example.invalid/?q={v}")).unwrap();
    let _ = c.get(u).send(); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dart-dio-geturi-decoy"); // @source(dart-dio-geturi-decoy)
    let c = reqwest::blocking::Client::new();
    let u = reqwest::Url::parse("http://example.invalid/").unwrap();
    let _ = c.get(u).header("x", &v).send(); // @sink(none) @nosink(dart-dio-geturi-decoy)
    m::sink_sql(&v); // @sink(dart-dio-geturi-decoy)
}
