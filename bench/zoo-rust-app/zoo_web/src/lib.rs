//! A skeleton of the shape of actix-web for the reachability cells (bench/zoo-rust-app): the items that the app calls, in the
//! modules where the real crate has them, with bodies that only keep what they are given. It is read as a dependency. The trait
//! `HttpServiceFactory` is declared in `service` and re-exported in `dev`: the handlers implement it as `dev::HttpServiceFactory`,
//! and the call in `ServiceConfig::service` names `service::HttpServiceFactory` (ADR-058).
mod service;
pub mod web;

pub mod dev {
    pub use crate::service::{AppService, HttpServiceFactory};
}

use crate::web::ServiceConfig;

pub struct App {
    configured: usize,
}

impl App {
    pub fn new() -> Self {
        App { configured: 0 }
    }

    /// `App::configure`: the function that is given is called with the config of the app.
    pub fn configure<F>(mut self, f: F) -> Self
    where
        F: FnOnce(&mut ServiceConfig),
    {
        let mut cfg = ServiceConfig::new();
        f(&mut cfg);
        self.configured += 1;
        self
    }
}

pub struct HttpServer<F> {
    factory: F,
}

impl<F> HttpServer<F>
where
    F: Fn() -> App + Send + Clone + 'static,
{
    pub fn new(factory: F) -> Self {
        HttpServer { factory }
    }

    pub fn bind(self, _addr: &str) -> std::io::Result<Self> {
        Ok(self)
    }

    pub fn run(self) -> std::io::Result<()> {
        let _app = (self.factory)();
        Ok(())
    }
}
