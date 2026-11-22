// @cell(F/pkg/actix-web/request-ref)
// @shape: a handler takes the HttpRequest by reference; the query string that it reads goes to a sink
use zoo_mark as m;
use actix_web::test::TestRequest;
use actix_web::HttpRequest;

pub fn register() {
    m::case("F/pkg/actix-web/request-ref/vuln", vuln);
    m::case("F/pkg/actix-web/request-ref/fixed", fixed);
    m::case("F/pkg/actix-web/request-ref/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-request-ref-vuln").value;
    let req = TestRequest::default().uri(&format!("/?{}", v)).to_http_request();
    handle_vuln(&req);
}

fn handle_vuln(req: &HttpRequest) { // @source(pkg-actix-web-request-ref-vuln)
    m::sink_cmd(req.query_string()); // @sink(pkg-actix-web-request-ref-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-request-ref-fixed").value;
    let req = TestRequest::default().uri(&format!("/?{}", v)).to_http_request();
    handle_fixed(&req);
}

fn handle_fixed(req: &HttpRequest) {
    let v = m::clean_cmd(req.query_string());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-request-ref-decoy").value;
    let req = TestRequest::default().uri(&format!("/?{}", v)).to_http_request();
    handle_decoy(&req);
}


fn handle_decoy(req: &HttpRequest) { // @source(pkg-actix-web-request-ref-decoy)
    let t = req.query_string().to_string();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-request-ref-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-request-ref-decoy)
}