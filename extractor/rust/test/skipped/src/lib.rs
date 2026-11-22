//! A crate for the extractor that holds functions it cannot read: rustc_public does not lower async closures (a
//! panic of the conversion), so `run` and the closures in it are skipped, and the other functions are read.
pub fn source() -> String {
    String::from("x")
}

pub fn sink(_s: &str) {}

pub fn before(x: u32) -> u32 {
    x + 1
}

pub async fn run(v: String) -> usize {
    let f = async |s: &str| s.len();
    f(&v).await
}

pub fn after(s: &str) -> String {
    s.to_string()
}

/// A flow that does not touch what is skipped: it is still found.
pub fn flows() {
    let v = source();
    sink(&after(&v));
}

/// A function that holds an async closure in a local: the type of the local cannot be converted, so the function itself
/// is skipped (it is not an async function: the panic is in its own body).
pub fn skipped_named(s: &str) -> String {
    let f = async |x: &str| x.len();
    let _ = f(s);
    s.to_string()
}

/// Calls the function whose body could not be read, with a value of the source, and gives the result to the sink.
pub fn calls_the_skipped() {
    let v = source();
    sink(&skipped_named(&v));
}
