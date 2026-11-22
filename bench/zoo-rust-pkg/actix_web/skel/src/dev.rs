// `actix_web::dev`: `ConnectionInfo` is the framework's; `Path` and `Url` are the crate actix-router's, re-exported.
pub use crate::info::ConnectionInfo;

pub struct Url;

pub struct Path<T> {
    pairs: Vec<(String, String)>,
    _url: std::marker::PhantomData<T>,
}

impl<T> Path<T> {
    pub(crate) fn from_pairs(pairs: Vec<(String, String)>) -> Self {
        Path { pairs, _url: std::marker::PhantomData }
    }

    pub fn get(&self, name: &str) -> Option<&str> {
        self.pairs.iter().find(|(k, _)| k == name).map(|(_, v)| v.as_str())
    }
}
