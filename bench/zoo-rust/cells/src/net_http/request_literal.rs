// @cell(F/net-http/request-literal)
// @shape: a value goes into the URL object made by Url::parse, the request is made by Request::new (not by Client::get) and is sent by Client::execute. The sink of the built-in rows is the argument of Client::get, so Request::new is not a sink. The crate of the zoo sends nothing and records the URL
use zoo_mark as m;

pub fn register() {
    m::case("F/net-http/request-literal/vuln", vuln);
    m::case("F/net-http/request-literal/fixed", fixed);
    m::case("F/net-http/request-literal/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("net-http-request-literal-vuln"); // @source(net-http-request-literal-vuln) @miss(silent) @ref(#238)
    let c = reqwest::blocking::Client::new();
    let u = reqwest::Url::parse(&format!("http://example.invalid/?q={v}")).unwrap();
    let _ = c.execute(reqwest::Request::new(reqwest::Method::GET, u)); // @sink(net-http-request-literal-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_url(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("net-http-request-literal-fixed"))))));
    let c = reqwest::blocking::Client::new();
    let u = reqwest::Url::parse(&format!("http://example.invalid/?q={v}")).unwrap();
    let _ = c.execute(reqwest::Request::new(reqwest::Method::GET, u)); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("net-http-request-literal-decoy"); // @source(net-http-request-literal-decoy)
    let c = reqwest::blocking::Client::new();
    let u = reqwest::Url::parse("http://example.invalid/").unwrap();
    let _ = c.execute(reqwest::Request::new(reqwest::Method::GET, u).header("x", &v)); // @sink(none) @nosink(net-http-request-literal-decoy)
    m::sink_sql(&v); // @sink(net-http-request-literal-decoy)
}
