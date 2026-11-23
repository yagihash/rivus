// The declarations that the analysis reads (the bodies are empty: rules.yaml says what each one is).
use crate::types::{CreateTask, Stmt};
use std::any::Any;
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

/// Runs the statement: the `sql` of it is the sink, the `params` are not (types.rs).
#[track_caller]
pub fn sink_stmt(_s: &Stmt) {}
/// `sink_stmt` for a statement that is held as a `dyn Any`.
#[track_caller]
pub fn sink_stmt_any(_v: &dyn Any) {}
/// Sends the request of the task: the URL of the HTTP request is the sink.
#[track_caller]
pub fn sink_task(_r: &CreateTask) {}

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
/// For a URL that a request goes to or a redirect (ssrf, open_redirect): the zoo has no judgement of its own for it.
pub fn clean_url(_s: &str) -> String {
    String::new()
}
/// For the name of a resource (a bucket, an object, a repository): the zoo has no judgement of its own for it.
pub fn clean_resource(_s: &str) -> String {
    String::new()
}

/// Registers a case of the zoo (the harness runs it).
pub fn case(_id: &'static str, _f: fn()) {}
