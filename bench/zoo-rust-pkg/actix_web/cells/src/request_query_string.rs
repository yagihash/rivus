// @cell(F/pkg/actix-web/request-query-string)
// @shape: a handler takes the HttpRequest; the query string that it reads goes to a sink
use zoo_mark as m;
use actix_web::test::TestRequest;
use actix_web::HttpRequest;

pub fn register() {
    m::case("F/pkg/actix-web/request-query-string/vuln", vuln);
    m::case("F/pkg/actix-web/request-query-string/fixed", fixed);
    m::case("F/pkg/actix-web/request-query-string/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-request-query-string-vuln").value;
    handle_vuln(TestRequest::default().uri(&format!("/?{}", v)).to_http_request());
}

fn handle_vuln(req: HttpRequest) { // @source(pkg-actix-web-request-query-string-vuln)
    m::sink_cmd(req.query_string()); // @sink(pkg-actix-web-request-query-string-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-request-query-string-fixed").value;
    handle_fixed(TestRequest::default().uri(&format!("/?{}", v)).to_http_request());
}

fn handle_fixed(req: HttpRequest) {
    let v = m::clean_cmd(req.query_string());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-request-query-string-decoy").value;
    handle_decoy(TestRequest::default().uri(&format!("/?{}", v)).to_http_request());
}


fn handle_decoy(req: HttpRequest) { // @source(pkg-actix-web-request-query-string-decoy)
    let t = req.query_string().to_string();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-request-query-string-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-request-query-string-decoy)
}