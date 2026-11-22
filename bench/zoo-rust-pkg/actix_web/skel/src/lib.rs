//! A skeleton of actix-web 4.x for the zoo (docs/DESIGN-package-models.md, 6.8): the public signatures that the cells call,
//! in the modules where the real crate has them (so that the paths of the items are the real ones: `actix_web::HttpRequest`,
//! `actix_web::web::Query`, `actix_web::dev::ConnectionInfo`, `actix_web::http::header::HeaderMap`), and bodies that only
//! keep what they are given and hand it back. It is not actix-web and runs no server; it is read as a dependency.
mod info;
mod request;
mod types;

pub mod cookie;
pub mod dev;
pub mod http;
pub mod test;
pub mod web;

pub use crate::request::HttpRequest;
