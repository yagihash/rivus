//! A skeleton of the crate google-cloud-storage (1.22.0) for the zoo: the public paths and signatures of the part that the cells
//! use, with bodies that only record (zoo-mark; `track_caller` hands the place of the call in the cell to it). It is not the crate.
//! The re-exports are the ones of the crate (`client`, `builder::storage`, `builder::storage_control`): the extractor writes the
//! path that the code of the user writes, and the rows of internal/model/builtin/rust-google-cloud-storage.yaml are on those.

mod control;
mod signed_url;
mod storage;

pub mod client {
    pub use crate::control::StorageControl;
    pub use crate::storage::Storage;
}

pub mod builder {
    pub mod storage {
        pub use crate::signed_url::SignedUrlBuilder;
        pub use crate::storage::{OpenObject, ReadObject, WriteObject};
    }
    pub mod storage_control {
        pub use crate::control::{DeleteObject, GetObject, ListObjects, MoveObject, RewriteObject};
    }
}

#[track_caller]
fn record(s: &str) {
    zoo_mark::sink_any(s);
}
