// @cell(F/dart/dio-post)
// @shape: a value goes into the URL of Client::post (the method of the client, which builds a request and sends it): the SSRF sink of the built-in rows. The crate of the zoo sends nothing and records the URL
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/dio-post/vuln", vuln);
    m::case("F/dart/dio-post/fixed", fixed);
    m::case("F/dart/dio-post/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("dart-dio-post-vuln"); // @source(dart-dio-post-vuln)
    let _ = reqwest::blocking::Client::new().post(format!("http://example.invalid/?q={v}")).send(); // @sink(dart-dio-post-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_url(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("dart-dio-post-fixed"))))));
    let _ = reqwest::blocking::Client::new().post(format!("http://example.invalid/?q={v}")).send(); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dart-dio-post-decoy"); // @source(dart-dio-post-decoy)
    let _ = reqwest::blocking::Client::new().post("http://example.invalid/").body(&v).send(); // @sink(none) @nosink(dart-dio-post-decoy)
    m::sink_sql(&v); // @sink(dart-dio-post-decoy)
}
