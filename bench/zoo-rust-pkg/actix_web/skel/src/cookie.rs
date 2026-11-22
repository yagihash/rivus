// `actix_web::cookie` is the crate `cookie`.
use std::borrow::Cow;

#[derive(Clone)]
pub struct Cookie<'c> {
    name: Cow<'c, str>,
    value: Cow<'c, str>,
}

impl<'c> Cookie<'c> {
    pub fn new<N, V>(name: N, value: V) -> Self
    where
        N: Into<Cow<'c, str>>,
        V: Into<Cow<'c, str>>,
    {
        Cookie { name: name.into(), value: value.into() }
    }

    pub fn name(&self) -> &str {
        &self.name
    }

    pub fn value(&self) -> &str {
        &self.value
    }
}
