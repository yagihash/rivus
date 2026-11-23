//! Stands for the crate tar 0.4 in the zoo (docs/DESIGN-package-models.md, 6.8): the public signatures that the built-in rows
//! (internal/model/builtin/rust-archive.yaml) name. The names are held as they are written in the archive.
use std::borrow::Cow;
use std::io;
use std::path::Path;

pub struct Header {
    name: String,
    link: String,
}

impl Header {
    pub fn zoo(name: String, link: String) -> Header {
        Header { name, link }
    }
    pub fn path(&self) -> io::Result<Cow<'_, Path>> {
        Ok(Cow::Borrowed(Path::new(&self.name)))
    }
    pub fn path_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Borrowed(self.name.as_bytes())
    }
    pub fn link_name(&self) -> io::Result<Option<Cow<'_, Path>>> {
        Ok(Some(Cow::Borrowed(Path::new(&self.link))))
    }
    pub fn link_name_bytes(&self) -> Option<Cow<'_, [u8]>> {
        Some(Cow::Borrowed(self.link.as_bytes()))
    }
    pub fn size(&self) -> io::Result<u64> {
        Ok(self.name.len() as u64)
    }
}

pub struct Entry {
    header: Header,
}

impl Entry {
    pub fn zoo(name: String, link: String) -> Entry {
        Entry { header: Header::zoo(name, link) }
    }
    pub fn header(&self) -> &Header {
        &self.header
    }
    pub fn path(&self) -> io::Result<Cow<'_, Path>> {
        self.header.path()
    }
    pub fn path_bytes(&self) -> Cow<'_, [u8]> {
        self.header.path_bytes()
    }
    pub fn link_name(&self) -> io::Result<Option<Cow<'_, Path>>> {
        self.header.link_name()
    }
    pub fn link_name_bytes(&self) -> Option<Cow<'_, [u8]>> {
        self.header.link_name_bytes()
    }
}
