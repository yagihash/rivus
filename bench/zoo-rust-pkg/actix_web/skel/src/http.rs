// `actix_web::http`: the types of the crates http and actix-http, as the framework re-exports them.
pub mod header {
    use std::collections::HashMap;

    #[derive(Debug)]
    pub struct ToStrError;

    pub struct HeaderValue(String);

    impl HeaderValue {
        pub fn to_str(&self) -> Result<&str, ToStrError> {
            Ok(&self.0)
        }

        pub fn as_bytes(&self) -> &[u8] {
            self.0.as_bytes()
        }
    }

    #[derive(Default)]
    pub struct HeaderMap {
        map: HashMap<String, HeaderValue>,
    }

    impl HeaderMap {
        pub fn get(&self, key: &str) -> Option<&HeaderValue> {
            self.map.get(key)
        }

        pub(crate) fn insert_text(&mut self, name: String, value: String) {
            self.map.insert(name, HeaderValue(value));
        }
    }
}

pub struct Uri(pub(crate) String);
