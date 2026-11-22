// @cell(F/pkg/actix-web/request-path)
// @shape: a handler takes the HttpRequest; the path that it reads goes to a sink
use zoo_mark as m;
use actix_web::test::TestRequest;
use actix_web::HttpRequest;

pub fn register() {
    m::case("F/pkg/actix-web/request-path/vuln", vuln);
    m::case("F/pkg/actix-web/request-path/fixed", fixed);
    m::case("F/pkg/actix-web/request-path/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-request-path-vuln").value;
    handle_vuln(TestRequest::default().uri(&format!("/{}", v)).to_http_request());
}

fn handle_vuln(req: HttpRequest) { // @source(pkg-actix-web-request-path-vuln)
    m::sink_cmd(req.path()); // @sink(pkg-actix-web-request-path-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-request-path-fixed").value;
    handle_fixed(TestRequest::default().uri(&format!("/{}", v)).to_http_request());
}

fn handle_fixed(req: HttpRequest) {
    let v = m::clean_cmd(req.path());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-request-path-decoy").value;
    handle_decoy(TestRequest::default().uri(&format!("/{}", v)).to_http_request());
}


fn handle_decoy(req: HttpRequest) { // @source(pkg-actix-web-request-path-decoy)
    let t = req.path().to_string();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-request-path-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-request-path-decoy)
}