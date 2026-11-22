// @cell(F/pkg/actix-web/request-connection-info)
// @shape: a handler takes the HttpRequest; the host that it reads from the connection info goes to a sink
use zoo_mark as m;
use actix_web::test::TestRequest;
use actix_web::HttpRequest;

pub fn register() {
    m::case("F/pkg/actix-web/request-connection-info/vuln", vuln);
    m::case("F/pkg/actix-web/request-connection-info/fixed", fixed);
    m::case("F/pkg/actix-web/request-connection-info/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-request-connection-info-vuln").value;
    handle_vuln(TestRequest::default().insert_header(("Host", v.as_str())).to_http_request());
}

fn handle_vuln(req: HttpRequest) { // @source(pkg-actix-web-request-connection-info-vuln)
    m::sink_cmd(req.connection_info().host()); // @sink(pkg-actix-web-request-connection-info-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-request-connection-info-fixed").value;
    handle_fixed(TestRequest::default().insert_header(("Host", v.as_str())).to_http_request());
}

fn handle_fixed(req: HttpRequest) {
    let v = m::clean_cmd(req.connection_info().host());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-request-connection-info-decoy").value;
    handle_decoy(TestRequest::default().insert_header(("Host", v.as_str())).to_http_request());
}


fn handle_decoy(req: HttpRequest) { // @source(pkg-actix-web-request-connection-info-decoy)
    let t = req.connection_info().host().to_string();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-request-connection-info-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-request-connection-info-decoy)
}