//! Stands for a dependency of the zoo: a crate of its own, outside the workspace (a path dependency of the cells), so that its
//! functions are those of a dependency: rivus reads their bodies as it reads those of a third-party crate, and does not
//! analyse them as the code under test.
use std::fmt::{self, Write};

/// Wraps a writer: what is written is handed to the writer that it wraps (the shape of the logging and indenting wrappers).
pub struct Wrap<W: Write> {
    inner: W,
}

pub fn wrap<W: Write>(w: W) -> Wrap<W> {
    Wrap { inner: w }
}

impl<W: Write> Write for Wrap<W> {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        self.inner.write_str(s)
    }
}

/// A function of a dependency that the analysis has no model of: it moves the "body" parameter of a statement to its `sql`
/// (what a function that is not read may do to a structure that it is given).
pub fn opaque(s: &mut zoo_mark::Stmt) {
    if let Some(b) = s.params.get("body") {
        s.sql = b.clone();
    }
}

/// Returns a closure that calls `f` (the shape of the middleware of the web frameworks: the closure that the dependency makes holds the
/// function it was given).
pub fn wrap_fn<F: Fn(String) -> String>(f: F) -> impl Fn(String) -> String {
    move |s| f(s)
}

/// The request that a `Router` hands to a handler.
pub struct Request {
    pub path: String,
}

/// A handler of a route: it takes the request and the segment that the route captures.
pub type Handler = fn(&Request, &str);

/// The skeleton of a router for the cells (the shape of the routers of the web frameworks): a handler is registered with a route, and
/// `serve` calls it with the request and the segment that the route captures (`/item/{id}`: the text of `id`).
pub struct Router {
    routes: Vec<(String, Handler)>,
}

impl Router {
    pub fn new() -> Router {
        Router { routes: Vec::new() }
    }

    /// Registers the handler of the route.
    pub fn get(&mut self, route: &str, h: Handler) {
        self.routes.push((route.to_string(), h));
    }

    /// Calls the handler of the route with the request and the segment of the path.
    pub fn serve(&self, route: &str, req: &Request, segment: &str) {
        for (r, h) in &self.routes {
            if r == route {
                h(req, segment);
            }
        }
    }
}
