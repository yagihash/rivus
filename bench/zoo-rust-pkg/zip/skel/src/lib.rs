//! Stands for the crate zip 9 in the zoo (docs/DESIGN-package-models.md, 6.8): the public signatures that the built-in rows
//! (internal/model/builtin/rust-archive.yaml) name. The names are held as they are written in the archive.
use std::borrow::Cow;
use std::path::PathBuf;

pub mod read {
    use super::*;

    pub struct ZipFile {
        name: String,
        size: u64,
    }

    impl ZipFile {
        pub fn zoo(name: String) -> ZipFile {
            ZipFile { size: name.len() as u64, name }
        }
        pub fn name(&self) -> Result<Cow<'_, str>, ()> {
            Ok(Cow::Borrowed(&self.name))
        }
        pub fn name_raw(&self) -> &[u8] {
            self.name.as_bytes()
        }
        pub fn mangled_name(&self) -> PathBuf {
            PathBuf::from(&self.name)
        }
        pub fn enclosed_name(&self) -> Option<PathBuf> {
            if self.name.contains("..") {
                None
            } else {
                Some(PathBuf::from(&self.name))
            }
        }
        pub fn size(&self) -> u64 {
            self.size
        }
    }
}

pub struct ZipArchive {
    names: Vec<String>,
}

impl ZipArchive {
    pub fn zoo(name: String) -> ZipArchive {
        ZipArchive { names: vec![name] }
    }
    pub fn file_names(&self) -> impl Iterator<Item = Result<Cow<'_, str>, ()>> {
        self.names.iter().map(|n| Ok(Cow::Borrowed(n.as_str())))
    }
}
