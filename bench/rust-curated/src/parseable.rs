//! The shapes of the parseable advisories (bench/parseable): the handlers take their input
//! in the types of actix-web (`web::Query`, `web::Json`, `web::Path`), and the sinks are the
//! Location header of a response, the files of the object storage, and the requests that the
//! alerts and the health checks send. The framework and the clients are small types of the crate;
//! rules.yaml names them the way parseable's rules name the real ones.
use std::collections::HashMap;
use std::sync::{OnceLock, RwLock};

pub mod web {
    // A `param_type` rule makes the parameter of that type a source in every function that takes
    // one: the source of a flow is the first place where the value is used, `into_inner`. So each
    // type has its own flow here, and the mark of the source is on the line of `into_inner`.
    pub struct Query<T>(pub T);
    pub struct Json<T>(pub T);
    pub struct Form<T>(pub T);
    pub struct Path<T>(pub T);
    impl<T> Query<T> {
        pub fn into_inner(self) -> T { // @source(pb_redirect)
            self.0
        }
    }
    impl<T> Json<T> {
        pub fn into_inner(self) -> T { // @source(pb_store)
            self.0
        }
    }
    impl<T> Form<T> {
        pub fn into_inner(self) -> T { // @source(pb_target) @miss(silent)
            self.0
        }
    }
    impl<T> Path<T> {
        pub fn into_inner(self) -> T { // @source(pb_node)
            self.0
        }
    }
}

pub mod http {
    pub struct ResponseBuilder {
        pub headers: Vec<(String, String)>,
    }
    impl ResponseBuilder {
        pub fn found() -> Self {
            ResponseBuilder { headers: Vec::new() }
        }
        // the sink of the open redirects: the header is a (name, value) pair
        pub fn insert_header(&mut self, header: (&str, String)) -> &mut Self {
            self.headers.push((header.0.to_string(), header.1));
            self
        }
        pub fn finish(&mut self) -> Response {
            Response { status: 302 }
        }
    }
    pub struct Response {
        pub status: u16,
    }
    // a client of the HTTP: its get and post are the sinks of the SSRF
    pub struct Client;
    impl Client {
        pub fn new() -> Self {
            Client
        }
        pub fn get(&self, url: &str) -> Result<String, String> {
            Ok(url.to_string())
        }
        pub fn post(&self, url: &str) -> Result<String, String> {
            Ok(url.to_string())
        }
    }
}

pub mod fs {
    // the files of the local storage: the sink of the path traversals
    pub fn write(path: &str, data: &[u8]) -> Result<(), String> {
        let _ = (path, data);
        Ok(())
    }
}

use http::{Client, Response, ResponseBuilder};

// ---- GHSA-mh89: the redirect of the logout (open redirect) ----

pub struct LogoutQuery {
    pub redirect: Option<String>,
}

pub fn redirect_to_client(url: &str, cookies: Vec<String>) -> Response {
    let mut resp = ResponseBuilder::found();
    for c in cookies {
        resp.insert_header(("Set-Cookie", c));
    }
    resp.insert_header(("Location", url.to_string())).finish() // @sink(pb_redirect)
}

pub async fn logout(query: web::Query<LogoutQuery>) -> Response {
    let q = query.into_inner();
    match q.redirect {
        Some(url) => redirect_to_client(&url, Vec::new()),
        None => redirect_to_client("/", Vec::new()),
    }
}

// ---- GHSA-q8xx: the object storage writes the path made of the name of a stream ----

pub trait ObjectStorage {
    fn put_object(&self, path: &str, body: &[u8]) -> Result<(), String>;
}

pub struct LocalFs {
    pub root: String,
}

impl ObjectStorage for LocalFs {
    fn put_object(&self, path: &str, body: &[u8]) -> Result<(), String> {
        let full = format!("{}/{}", self.root, path);
        fs::write(&full, body) // @sink(pb_store)
    }
}

pub struct S3Store {
    pub bucket: String,
}

impl ObjectStorage for S3Store {
    fn put_object(&self, path: &str, body: &[u8]) -> Result<(), String> {
        let _ = (&self.bucket, path, body);
        Ok(())
    }
}

pub struct Parseable {
    pub storage: Box<dyn ObjectStorage>,
}

pub struct Filter {
    pub stream_name: String,
    pub filter_id: String,
}

// the body of the request names the stream; the path of the filter is made of it
pub async fn post_filter(p: &Parseable, body: web::Json<Filter>) -> Result<(), String> {
    let filter = body.into_inner();
    let path = format!("{}/.filters/{}.json", filter.stream_name, filter.filter_id);
    p.storage.put_object(&path, b"{}")
}

// ---- GHSA-64vg: the endpoint of an alert target is stored, and posted to later ----

#[derive(Clone)]
pub struct Target {
    pub name: String,
    pub endpoint: String,
}

fn targets() -> &'static RwLock<HashMap<String, Target>> {
    static TARGETS: OnceLock<RwLock<HashMap<String, Target>>> = OnceLock::new();
    TARGETS.get_or_init(|| RwLock::new(HashMap::new()))
}

// POST /api/v1/targets: the target is kept in the map of the targets (it is a `Form` here, so that the
// flow has a source of its own: in parseable it is a Json, like the body of the filter)
pub async fn post_target(body: web::Form<Target>) -> Result<(), String> {
    let target = body.into_inner();
    targets().write().unwrap().insert(target.name.clone(), target);
    Ok(())
}

impl Target {
    fn call(&self, client: &Client) -> Result<String, String> {
        client.post(&self.endpoint) // @sink(pb_target)
    }
}

// the alert that fires reads the targets, and each of them posts
pub fn fire_alert(client: &Client) {
    let all: Vec<Target> = targets().read().unwrap().values().cloned().collect();
    for t in all {
        let _ = t.call(client);
    }
}

// ---- a peer to drop is named by the path (a client-chosen host; the shape only) ----

fn normalize_addr(s: String) -> String {
    if s.starts_with("http") {
        s
    } else {
        format!("http://{}", s)
    }
}

pub fn ping_peer(addr: &str) -> bool {
    Client::new().get(addr).is_ok() // @sink(pb_node)
}

pub async fn choose_peer(peer: web::Path<String>) -> bool {
    let url = normalize_addr(peer.into_inner());
    ping_peer(&url)
}
