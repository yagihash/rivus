//! A skeleton for the zoo: the public paths of the types of axum 0.8 that the model rows (internal/model/builtin/rust-web.yaml) name,
//! with bodies that only record (the signatures are the ones of axum 0.8.9; no code of axum is here). A value that stands for what a
//! client wrote is made with `In::new` of zoo-mark (the tag of the case), as the harness of the zoo does for its own source type.

/// `axum::http` (the re-export of the crate http).
pub mod http {
    pub struct HeaderMap {
        pub value: String,
    }
    impl HeaderMap {
        pub fn new(tag: &str) -> HeaderMap {
            HeaderMap { value: zoo_mark::In::new(tag).value }
        }
    }

    pub struct Request {
        pub value: String,
    }
    impl Request {
        pub fn new(tag: &str) -> Request {
            Request { value: zoo_mark::In::new(tag).value }
        }
    }

    pub mod request {
        pub struct Parts {
            pub value: String,
        }
        impl Parts {
            pub fn new(tag: &str) -> Parts {
                Parts { value: zoo_mark::In::new(tag).value }
            }
        }
    }
}

pub mod extract {
    pub struct OriginalUri {
        pub value: String,
    }
    impl OriginalUri {
        pub fn new(tag: &str) -> OriginalUri {
            OriginalUri { value: zoo_mark::In::new(tag).value }
        }
    }

    pub struct RawPathParams {
        pub value: String,
    }
    impl RawPathParams {
        pub fn new(tag: &str) -> RawPathParams {
            RawPathParams { value: zoo_mark::In::new(tag).value }
        }
    }
}

pub mod body {
    pub struct Body {
        pub value: String,
    }
    impl Body {
        pub fn new(tag: &str) -> Body {
            Body { value: zoo_mark::In::new(tag).value }
        }
    }
}

pub mod response {
    pub struct Redirect {
        uri: String,
    }
    impl Redirect {
        /// The record is the call in the cell (`track_caller`).
        #[track_caller]
        pub fn to(uri: &str) -> Redirect {
            zoo_mark::sink_cmd(uri);
            Redirect { uri: uri.to_string() }
        }
        #[track_caller]
        pub fn temporary(uri: &str) -> Redirect {
            zoo_mark::sink_cmd(uri);
            Redirect { uri: uri.to_string() }
        }
        #[track_caller]
        pub fn permanent(uri: &str) -> Redirect {
            zoo_mark::sink_cmd(uri);
            Redirect { uri: uri.to_string() }
        }
        pub fn location(&self) -> &str {
            &self.uri
        }
    }
}
