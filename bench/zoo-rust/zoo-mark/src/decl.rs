// The declarations that the analysis reads (the bodies are empty: rules.yaml says what each one is).
use std::fmt::Debug;

/// The source that is the type of a parameter: the handler under test takes an `In`.
pub struct In {
    pub value: String,
}

impl In {
    pub fn new(_tag: &str) -> In {
        In { value: String::new() }
    }
}

/// Source: the marker of the tag.
pub fn src(_tag: &str) -> String {
    String::new()
}

/// Source: the marker as bytes.
pub fn src_bytes(_tag: &str) -> Vec<u8> {
    Vec::new()
}

// The sinks of each class: they record what they got and execute nothing.
#[track_caller]
pub fn sink_cmd(_s: &str) {}
/// A sink that takes the string (ownership moves in).
#[track_caller]
pub fn sink_cmd_own(_s: String) {}
#[track_caller]
pub fn sink_cmd_bytes(_b: &[u8]) {}
#[track_caller]
pub fn sink_sql(_s: &str) {}
#[track_caller]
pub fn sink_html(_s: &str) {}
#[track_caller]
pub fn sink_path(_base: &str, _s: &str) {}
/// For structs and containers (the text is the `Debug` output).
#[track_caller]
pub fn sink_any<T: Debug + ?Sized>(_v: &T) {}

// The sanitizers: each replaces the dangerous characters of its class by '_'.
pub fn clean_cmd(_s: &str) -> String {
    String::new()
}
pub fn clean_sql(_s: &str) -> String {
    String::new()
}
pub fn clean_html(_s: &str) -> String {
    String::new()
}
pub fn clean_path(_s: &str) -> String {
    String::new()
}

/// Registers a case of the zoo (the harness runs it).
pub fn case(_id: &'static str, _f: fn()) {}
