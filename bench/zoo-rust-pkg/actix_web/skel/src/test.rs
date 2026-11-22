// `actix_web::test::TestRequest`: builds an `HttpRequest` (the cells of the zoo use it to make one).
use crate::cookie::Cookie;
use crate::dev::{ConnectionInfo, Path};
use crate::http::header::HeaderMap;
use crate::http::Uri;
use crate::HttpRequest;
use std::borrow::Cow;
use std::cell::RefCell;

#[derive(Default)]
pub struct TestRequest {
    uri: String,
    headers: HeaderMap,
    params: Vec<(String, String)>,
    cookies: Vec<Cookie<'static>>,
}

impl TestRequest {
    pub fn uri(mut self, path: &str) -> Self {
        self.uri = path.to_string();
        self
    }

    pub fn insert_header(mut self, header: (&str, &str)) -> Self {
        self.headers.insert_text(header.0.to_string(), header.1.to_string());
        self
    }

    pub fn param(mut self, name: impl Into<Cow<'static, str>>, value: impl Into<Cow<'static, str>>) -> Self {
        self.params.push((name.into().into_owned(), value.into().into_owned()));
        self
    }

    pub fn cookie(mut self, cookie: Cookie<'_>) -> Self {
        self.cookies.push(Cookie::new(cookie.name().to_string(), cookie.value().to_string()));
        self
    }

    pub fn to_http_request(self) -> HttpRequest {
        let host = match self.headers.get("Host") {
            Some(v) => v.to_str().unwrap_or("").to_string(),
            None => String::new(),
        };
        HttpRequest {
            uri: Uri(self.uri),
            headers: self.headers,
            params: Path::from_pairs(self.params),
            cookies: RefCell::new(self.cookies),
            info: RefCell::new(ConnectionInfo { host }),
            peer: None,
        }
    }
}
