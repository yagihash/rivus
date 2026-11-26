// @cell(F/dart/http-ioclient-send)
// @shape: a value goes into the request that is sent by Client::execute of a variable typed with the concrete type reqwest::blocking::Client: the request is made by Client::get on the same line, the sink of the built-in rows. The crate of the zoo sends nothing and records the URL
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/http-ioclient-send/vuln", vuln);
    m::case("F/dart/http-ioclient-send/fixed", fixed);
    m::case("F/dart/http-ioclient-send/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("dart-http-ioclient-send-vuln"); // @source(dart-http-ioclient-send-vuln)
    let c: reqwest::blocking::Client = reqwest::blocking::Client::new();
    { let r = c.get(format!("http://example.invalid/?q={v}")).build().unwrap(); let _ = c.execute(r); } // @sink(dart-http-ioclient-send-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_url(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("dart-http-ioclient-send-fixed"))))));
    let c: reqwest::blocking::Client = reqwest::blocking::Client::new();
    { let r = c.get(format!("http://example.invalid/?q={v}")).build().unwrap(); let _ = c.execute(r); } // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dart-http-ioclient-send-decoy"); // @source(dart-http-ioclient-send-decoy)
    let c: reqwest::blocking::Client = reqwest::blocking::Client::new();
    { let r = c.get("http://example.invalid/").build().unwrap(); let _ = c.execute(r); } // @sink(none) @nosink(dart-http-ioclient-send-decoy)
    m::sink_sql(&v); // @sink(dart-http-ioclient-send-decoy)
}
