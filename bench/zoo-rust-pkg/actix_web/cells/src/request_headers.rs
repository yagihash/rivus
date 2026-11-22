// @cell(F/pkg/actix-web/request-headers)
// @shape: a handler takes the HttpRequest; a header that it reads (HeaderMap::get, HeaderValue::to_str) goes to a sink
use zoo_mark as m;
use actix_web::test::TestRequest;
use actix_web::HttpRequest;

pub fn register() {
    m::case("F/pkg/actix-web/request-headers/vuln", vuln);
    m::case("F/pkg/actix-web/request-headers/fixed", fixed);
    m::case("F/pkg/actix-web/request-headers/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-request-headers-vuln").value;
    handle_vuln(TestRequest::default().insert_header(("X-Input", v.as_str())).to_http_request());
}

fn handle_vuln(req: HttpRequest) { // @source(pkg-actix-web-request-headers-vuln)
    m::sink_cmd(req.headers().get("X-Input").unwrap().to_str().unwrap()); // @sink(pkg-actix-web-request-headers-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-request-headers-fixed").value;
    handle_fixed(TestRequest::default().insert_header(("X-Input", v.as_str())).to_http_request());
}

fn handle_fixed(req: HttpRequest) {
    let v = m::clean_cmd(req.headers().get("X-Input").unwrap().to_str().unwrap());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-request-headers-decoy").value;
    handle_decoy(TestRequest::default().insert_header(("X-Input", v.as_str())).to_http_request());
}


fn handle_decoy(req: HttpRequest) { // @source(pkg-actix-web-request-headers-decoy)
    let t = req.headers().get("X-Input").unwrap().to_str().unwrap().to_string();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-request-headers-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-request-headers-decoy)
}