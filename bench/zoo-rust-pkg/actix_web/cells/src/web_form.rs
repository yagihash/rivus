// @cell(F/pkg/actix-web/web-form)
// @shape: a handler takes web::Form; what it takes out of it (into_inner) goes to a sink
use zoo_mark as m;
use actix_web::web;

pub fn register() {
    m::case("F/pkg/actix-web/web-form/vuln", vuln);
    m::case("F/pkg/actix-web/web-form/fixed", fixed);
    m::case("F/pkg/actix-web/web-form/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-web-form-vuln").value;
    handle_vuln(web::Form(v));
}

fn handle_vuln(f: web::Form<String>) { // @source(pkg-actix-web-web-form-vuln)
    m::sink_cmd(&f.into_inner()); // @sink(pkg-actix-web-web-form-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-web-form-fixed").value;
    handle_fixed(web::Form(v));
}

fn handle_fixed(f: web::Form<String>) {
    let v = m::clean_cmd(&f.into_inner());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-web-form-decoy").value;
    handle_decoy(web::Form(v));
}


fn handle_decoy(f: web::Form<String>) { // @source(pkg-actix-web-web-form-decoy)
    let t = f.into_inner();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-web-form-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-web-form-decoy)
}