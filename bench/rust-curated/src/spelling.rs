// Calls whose names the extractor writes in a way that is not the one of the type in the source: a Vec gets sort, reverse,
// iter and first from the slice, a String gets trim from str, the atomic types are one generic type. The rows of the
// built-in model are spelled the way the extractor writes them, and TestBuiltinRowsAreHitByTheZoo reads the IR of this
// file to hold them to it. Nothing here has a source or a sink.
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};

pub fn slices(mut v: Vec<String>) -> usize {
    v.sort();
    v.reverse();
    let n = v.iter().count() + v.first().map(|s| s.len()).unwrap_or(0);
    n + v.last().map(|s| s.len()).unwrap_or(0)
}

pub fn strings(s: &str) -> String {
    let owned = s.to_string();
    owned.trim().to_lowercase()
}

pub fn atomics(a: &AtomicBool, b: &AtomicUsize) -> bool {
    a.store(true, Ordering::SeqCst);
    b.store(1, Ordering::SeqCst);
    let _ = a.swap(false, Ordering::SeqCst);
    a.load(Ordering::SeqCst) && b.load(Ordering::SeqCst) > 0
}
