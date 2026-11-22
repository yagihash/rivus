// `info::ConnectionInfo` is public as `actix_web::dev::ConnectionInfo`.
pub struct ConnectionInfo {
    pub(crate) host: String,
}

impl ConnectionInfo {
    pub fn realip_remote_addr(&self) -> Option<&str> {
        Some(&self.host)
    }

    pub fn peer_addr(&self) -> Option<&str> {
        Some(&self.host)
    }

    pub fn host(&self) -> &str {
        &self.host
    }

    pub fn scheme(&self) -> &str {
        "http"
    }

    pub fn remote_addr(&self) -> Option<&str> {
        Some(&self.host)
    }
}
