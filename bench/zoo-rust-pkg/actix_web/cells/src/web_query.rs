// @cell(F/pkg/actix-web/web-query)
// @shape: a handler takes web::Query; what it takes out of it (into_inner) goes to a sink
use zoo_mark as m;
use actix_web::web;

pub fn register() {
    m::case("F/pkg/actix-web/web-query/vuln", vuln);
    m::case("F/pkg/actix-web/web-query/fixed", fixed);
    m::case("F/pkg/actix-web/web-query/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::In::new("pkg-actix-web-web-query-vuln").value;
    handle_vuln(web::Query(v));
}

fn handle_vuln(q: web::Query<String>) { // @source(pkg-actix-web-web-query-vuln)
    m::sink_cmd(&q.into_inner()); // @sink(pkg-actix-web-web-query-vuln)
}

fn fixed() { // @fixed
    let v = m::In::new("pkg-actix-web-web-query-fixed").value;
    handle_fixed(web::Query(v));
}

fn handle_fixed(q: web::Query<String>) {
    let v = m::clean_cmd(&q.into_inner());
    m::sink_cmd(&v); // @sink(none)
}

fn decoy() { // @decoy
    let v = m::In::new("pkg-actix-web-web-query-decoy").value;
    handle_decoy(web::Query(v));
}


fn handle_decoy(q: web::Query<String>) { // @source(pkg-actix-web-web-query-decoy)
    let t = q.into_inner();
    m::sink_cmd("constant"); // @sink(none) @nosink(pkg-actix-web-web-query-decoy)
    m::sink_cmd(&t); // @sink(pkg-actix-web-web-query-decoy)
}