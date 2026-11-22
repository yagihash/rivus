// @cell(F/pkg/actix-web/web-json)
// @shape: a handler takes web::Json; what it takes out of it (into_inner) goes to a sink
use zoo_mark as m;
use actix_web::web;

pub fn register() {
    m::case("F/pkg/actix-web/web-json/vuln", vuln);
    m::case("F/pkg/actix-web/web-json/fixed", fixed);
    m::case("F/pkg/actix-web/web-json/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-web-json-vuln").value;
    handle_vuln(web::Json(v));
}

fn handle_vuln(j: web::Json<String>) { // @source(pkg-actix-web-web-json-vuln)
    m::sink_cmd(&j.into_inner()); // @sink(pkg-actix-web-web-json-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-web-json-fixed").value;
    handle_fixed(web::Json(v));
}

fn handle_fixed(j: web::Json<String>) {
    let v = m::clean_cmd(&j.into_inner());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-web-json-decoy").value;
    handle_decoy(web::Json(v));
}


fn handle_decoy(j: web::Json<String>) { // @source(pkg-actix-web-web-json-decoy)
    let t = j.into_inner();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-web-json-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-web-json-decoy)
}