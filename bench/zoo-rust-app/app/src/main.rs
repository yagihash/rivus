// The app of the Rust zoo, for the entry points that a framework calls (docs/DESIGN-zoo.md, "Reachability"). The shape is
// rustypaste's: main builds an HttpServer with a closure, the closure makes an App and configures it with a function, and the
// function registers the handlers with the config. The comment of each sink says what its finding must have
// (TestZooRustReach holds it): `from_entry` is "not not_from_entry" (from_entry or from_framework_callback), whichever way the
// handlers of a framework come to be found.
mod server;

use zoo_mark::{sink_cmd, src};
use zoo_web::{App, HttpServer};

fn main() -> std::io::Result<()> {
    sink_cmd(&src("app-main")); // from_entry
    helper();
    server::other::run_hook(&server::other::hooked);
    // the shape of the real target: the factory is a closure, the app is configured with a function item
    let http_server = HttpServer::new(move || App::new().configure(server::configure_routes)).bind("127.0.0.1:8000")?;
    http_server.run()
}

// reached from main through a call
fn helper() {
    sink_cmd(&src("app-helper")); // from_entry
}
