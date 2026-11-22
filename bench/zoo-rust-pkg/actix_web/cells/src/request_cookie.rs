// @cell(F/pkg/actix-web/request-cookie)
// @shape: a handler takes the HttpRequest; a cookie that it reads goes to a sink
use zoo_mark as m;
use actix_web::cookie::Cookie;
use actix_web::test::TestRequest;
use actix_web::HttpRequest;

pub fn register() {
    m::case("F/pkg/actix-web/request-cookie/vuln", vuln);
    m::case("F/pkg/actix-web/request-cookie/fixed", fixed);
    m::case("F/pkg/actix-web/request-cookie/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-request-cookie-vuln").value;
    handle_vuln(TestRequest::default().cookie(Cookie::new("sid", v)).to_http_request());
}

fn handle_vuln(req: HttpRequest) { // @source(pkg-actix-web-request-cookie-vuln)
    m::sink_cmd(req.cookie("sid").unwrap().value()); // @sink(pkg-actix-web-request-cookie-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-request-cookie-fixed").value;
    handle_fixed(TestRequest::default().cookie(Cookie::new("sid", v)).to_http_request());
}

fn handle_fixed(req: HttpRequest) {
    let v = m::clean_cmd(req.cookie("sid").unwrap().value());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-request-cookie-decoy").value;
    handle_decoy(TestRequest::default().cookie(Cookie::new("sid", v)).to_http_request());
}


fn handle_decoy(req: HttpRequest) { // @source(pkg-actix-web-request-cookie-decoy)
    let t = req.cookie("sid").unwrap().value().to_string();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-request-cookie-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-request-cookie-decoy)
}