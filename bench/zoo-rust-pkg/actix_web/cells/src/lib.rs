//! The cells of the package actix_web of the Rust zoo: one file is one cell (three cases). The modules are listed here; the
//! harness of the test runs the `register()` of every file of this directory.
pub mod request_query_string;
pub mod request_path;
pub mod request_ref;
pub mod request_headers;
pub mod request_match_info;
pub mod request_cookie;
pub mod request_connection_info;
pub mod web_query;
pub mod web_json;
pub mod web_form;
