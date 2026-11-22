// @cell(F/pkg/actix-web/request-match-info)
// @shape: a handler takes the HttpRequest; a segment of the path that it reads (match_info) goes to a sink
use zoo_mark as m;
use actix_web::test::TestRequest;
use actix_web::HttpRequest;

pub fn register() {
    m::case("F/pkg/actix-web/request-match-info/vuln", vuln);
    m::case("F/pkg/actix-web/request-match-info/fixed", fixed);
    m::case("F/pkg/actix-web/request-match-info/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-request-match-info-vuln").value;
    handle_vuln(TestRequest::default().param("id", v).to_http_request());
}

fn handle_vuln(req: HttpRequest) { // @source(pkg-actix-web-request-match-info-vuln)
    m::sink_cmd(req.match_info().get("id").unwrap()); // @sink(pkg-actix-web-request-match-info-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-request-match-info-fixed").value;
    handle_fixed(TestRequest::default().param("id", v).to_http_request());
}

fn handle_fixed(req: HttpRequest) {
    let v = m::clean_cmd(req.match_info().get("id").unwrap());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-request-match-info-decoy").value;
    handle_decoy(TestRequest::default().param("id", v).to_http_request());
}


fn handle_decoy(req: HttpRequest) { // @source(pkg-actix-web-request-match-info-decoy)
    let t = req.match_info().get("id").unwrap().to_string();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-request-match-info-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-request-match-info-decoy)
}