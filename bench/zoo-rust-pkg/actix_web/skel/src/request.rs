// `request::HttpRequest` is public as `actix_web::HttpRequest`.
use crate::dev::{ConnectionInfo, Path, Url};
use crate::http::header::HeaderMap;
use crate::http::Uri;
use std::cell::{Ref, RefCell};
use std::net::SocketAddr;

pub struct HttpRequest {
    pub(crate) uri: Uri,
    pub(crate) headers: HeaderMap,
    pub(crate) params: Path<Url>,
    pub(crate) cookies: RefCell<Vec<crate::cookie::Cookie<'static>>>,
    pub(crate) info: RefCell<ConnectionInfo>,
    pub(crate) peer: Option<SocketAddr>,
}

impl HttpRequest {
    pub fn uri(&self) -> &Uri {
        &self.uri
    }

    pub fn headers(&self) -> &HeaderMap {
        &self.headers
    }

    pub fn path(&self) -> &str {
        match self.uri.0.split_once('?') {
            Some((p, _)) => p,
            None => &self.uri.0,
        }
    }

    pub fn query_string(&self) -> &str {
        match self.uri.0.split_once('?') {
            Some((_, q)) => q,
            None => "",
        }
    }

    pub fn match_info(&self) -> &Path<Url> {
        &self.params
    }

    pub fn peer_addr(&self) -> Option<SocketAddr> {
        self.peer
    }

    pub fn connection_info(&self) -> Ref<'_, ConnectionInfo> {
        self.info.borrow()
    }

    pub fn cookies(&self) -> Result<Ref<'_, Vec<crate::cookie::Cookie<'static>>>, ()> {
        Ok(self.cookies.borrow())
    }

    pub fn cookie(&self, name: &str) -> Option<crate::cookie::Cookie<'static>> {
        self.cookies.borrow().iter().find(|c| c.name() == name).cloned()
    }
}
