//! Stands for the `reqwest` crate (the shape of the API that the cells use). Nothing is sent: the place where the real crate sends
//! (`blocking::get`, `RequestBuilder::send`, `Client::execute`) hands the URL to the recording
//! declaration of zoo-mark, which does nothing in the analysis and records it when the harness runs the cells.
use std::fmt;

/// The error of the crate.
#[derive(Debug)]
pub struct Error;

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("reqwest stub")
    }
}

/// The URL object.
#[derive(Debug, Clone)]
pub struct Url(String);

impl Url {
    pub fn parse(s: &str) -> Result<Url, Error> {
        Ok(Url(s.to_string()))
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }
}

/// What a method of the crate takes as a URL.
pub trait IntoUrl {
    fn into_url(self) -> Result<Url, Error>;
}

impl IntoUrl for &str {
    fn into_url(self) -> Result<Url, Error> {
        Url::parse(self)
    }
}

impl IntoUrl for String {
    fn into_url(self) -> Result<Url, Error> {
        Url::parse(&self)
    }
}

impl IntoUrl for Url {
    fn into_url(self) -> Result<Url, Error> {
        Ok(self)
    }
}

#[derive(Debug, Clone, Copy)]
pub enum Method {
    GET,
    POST,
}

/// A request made by hand: the method and the URL.
pub struct Request {
    method: Method,
    url: Url,
}

impl Request {
    pub fn new(method: Method, url: Url) -> Request {
        Request { method, url }
    }

    /// A header of the request (what the sink is not about).
    pub fn header(self, _name: &str, _value: &str) -> Request {
        self
    }

    pub fn method(&self) -> Method {
        self.method
    }

    pub fn url(&self) -> &Url {
        &self.url
    }
}

pub mod multipart {
    /// The multipart body.
    pub struct Form {
        parts: Vec<String>,
    }

    impl Form {
        pub fn new() -> Form {
            Form { parts: Vec::new() }
        }

        /// A text part (its value is no path and no URL).
        pub fn text(mut self, name: &str, value: &str) -> Form {
            self.parts.push(format!("{name}={value}"));
            self
        }
    }
}

pub mod blocking {
    use super::{Error, IntoUrl, Method, Request, Url};

    /// What the server answers (nothing: the stub sends nothing).
    pub struct Response;

    /// Gets the URL.
    pub fn get<U: IntoUrl>(url: U) -> Result<Response, Error> {
        let u = url.into_url()?;
        zoo_mark::sink_any(u.as_str());
        Ok(Response)
    }

    pub struct Client;

    impl Client {
        pub fn new() -> Client {
            Client
        }

        pub fn get<U: IntoUrl>(&self, url: U) -> RequestBuilder {
            RequestBuilder { method: Method::GET, url: url.into_url(), body: String::new(), headers: Vec::new() }
        }

        pub fn post<U: IntoUrl>(&self, url: U) -> RequestBuilder {
            RequestBuilder { method: Method::POST, url: url.into_url(), body: String::new(), headers: Vec::new() }
        }

        /// Sends the request that was made by hand.
        pub fn execute(&self, request: Request) -> Result<Response, Error> {
            zoo_mark::sink_any(request.url().as_str());
            Ok(Response)
        }
    }

    pub struct RequestBuilder {
        method: Method,
        url: Result<Url, Error>,
        body: String,
        headers: Vec<(String, String)>,
    }

    impl RequestBuilder {
        pub fn body(mut self, body: &str) -> RequestBuilder {
            self.body = body.to_string();
            self
        }

        pub fn header(mut self, name: &str, value: &str) -> RequestBuilder {
            self.headers.push((name.to_string(), value.to_string()));
            self
        }

        /// Makes the request without sending it.
        pub fn build(self) -> Result<Request, Error> {
            Ok(Request::new(self.method, self.url?))
        }

        /// Sends the request: the URL is the sink.
        pub fn send(self) -> Result<Response, Error> {
            let u = self.url?;
            zoo_mark::sink_any(u.as_str());
            Ok(Response)
        }
    }
}
