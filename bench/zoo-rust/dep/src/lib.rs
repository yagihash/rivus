//! Stands for a dependency of the zoo: a crate of its own, outside the workspace (a path dependency of the cells), so that its
//! functions are those of a dependency: rivus reads their bodies as it reads those of a third-party crate, and does not
//! analyse them as the code under test.
use std::fmt::{self, Write};

/// Wraps a writer: what is written is handed to the writer that it wraps (the shape of the logging and indenting wrappers).
pub struct Wrap<W: Write> {
    inner: W,
}

pub fn wrap<W: Write>(w: W) -> Wrap<W> {
    Wrap { inner: w }
}

impl<W: Write> Write for Wrap<W> {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        self.inner.write_str(s)
    }
}

/// A function of a dependency that the analysis has no model of: it moves the "body" parameter of a statement to its `sql`
/// (what a function that is not read may do to a structure that it is given).
pub fn opaque(s: &mut zoo_mark::Stmt) {
    if let Some(b) = s.params.get("body") {
        s.sql = b.clone();
    }
}
