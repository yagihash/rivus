// @cell(F/dart/callback-param-closure)
// @shape: a closure written where the route is registered is called by the router with the segment of the path as its second argument: the segment is user input (callback_param of the rules), the request is not what is marked
use zoo_dep::{Request, Router};
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/callback-param-closure/vuln", vuln);
    m::case("F/dart/callback-param-closure/fixed", fixed);
    m::case("F/dart/callback-param-closure/decoy", decoy);
}

fn typed(tag: &str) -> String {
    m::In::new(tag).value
}

fn vuln() { // @vuln
    let mut router = Router::new();
    router.get("/item/{id}", |_req: &Request, id: &str| { // @source(dart-callback-param-closure-vuln)
        m::sink_cmd(id); // @sink(dart-callback-param-closure-vuln)
    });
    let req = Request { path: "/item/x".to_string() };
    router.serve("/item/{id}", &req, &typed("dart-callback-param-closure-vuln"));
}

fn fixed() { // @fixed
    let mut router = Router::new();
    router.get("/item/{id}", |_req: &Request, id: &str| {
        m::sink_cmd(&m::clean_cmd(id)); // @sink(none)
    });
    let req = Request { path: "/item/x".to_string() };
    router.serve("/item/{id}", &req, &typed("dart-callback-param-closure-fixed"));
}

fn decoy() { // @decoy
    let mut router = Router::new();
    router.get("/item/{id}", |req: &Request, id: &str| { // @source(dart-callback-param-closure-decoy)
        m::sink_cmd(&req.path); // @sink(none) @nosink(dart-callback-param-closure-decoy)
        m::sink_sql(id); // @sink(dart-callback-param-closure-decoy)
    });
    let req = Request { path: "/item/x".to_string() };
    router.serve("/item/{id}", &req, &typed("dart-callback-param-closure-decoy"));
}
