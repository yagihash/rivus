use crate::helpers::*;
use crate::{sink, source};

// Result, `?`, and the error path.

#[derive(Debug)]
pub struct AppError {
    pub detail: String,
}

fn parse(s: String) -> Result<String, AppError> {
    if s.is_empty() {
        Err(AppError { detail: s })
    } else {
        Ok(s)
    }
}

pub fn ok_value() {
    let r = parse(source()); // @source(ok-value)
    if let Ok(v) = r {
        sink(&v); // @sink(ok-value)
    }
}

pub fn err_value() {
    let r = parse(source()); // @source(err-value)
    if let Err(e) = r {
        sink(&e.detail); // @sink(err-value)
    }
}

fn question(s: String) -> Result<String, AppError> {
    let v = parse(s)?;
    Ok(v)
}

pub fn question_mark() {
    let a = source(); // @source(question)
    if let Ok(v) = question(a) {
        sink(&v); // @sink(question)
    }
}

pub fn question_mark_err() {
    let a = source(); // @source(question-err)
    if let Err(e) = question(a) {
        sink(&e.detail); // @sink(question-err)
    }
}

fn convert(s: String) -> Result<String, String> {
    parse(s).map_err(|e| e.detail)
}

pub fn map_err_value() {
    let a = source(); // @source(map-err)
    if let Err(e) = convert(a) {
        sink(&e); // @sink(map-err)
    }
}

pub fn match_result() {
    let a = source(); // @source(match)
    match parse(a) {
        Ok(v) => sink(&v), // @sink(match)
        Err(e) => sink(&e.detail), // @sink(match)
    }
}

// the closure returns a Result, and `?` is used inside it
pub fn closure_with_question() {
    let a = source(); // @source(closure-q)
    let r: Result<String, AppError> = (|| {
        let v = parse(a)?;
        Ok(v)
    })();
    sink(&r.unwrap()); // @sink(closure-q)
}

// a helper that takes a closure returning a Result
fn try_with(s: String, f: impl Fn(String) -> Result<String, AppError>) -> Result<String, AppError> {
    let v = f(s)?;
    Ok(v)
}

pub fn try_with_a() {
    let a = source(); // @source(try-a)
    let r = try_with(a, |x| Ok(x));
    sink(&r.unwrap()); // @sink(try-a) @nosink(try-b)
}

pub fn try_with_b() {
    let b = source(); // @source(try-b)
    let r = try_with(b, |x| Ok(x));
    sink(&r.unwrap()); // @sink(try-b) @nosink(try-a)
}

pub fn try_with_err() {
    let a = source(); // @source(try-err)
    let r = try_with(a, |x| Err(AppError { detail: x }));
    sink(&r.unwrap_err().detail); // @sink(try-err)
}

// panics and the value that they carry
pub fn unwrap_or_default() {
    let r: Result<String, AppError> = Err(AppError { detail: String::new() });
    let a = source(); // @source(unwrap-or)
    let v = r.unwrap_or(a);
    sink(&v); // @sink(unwrap-or)
}

pub fn unwrap_or_else_closure() {
    let r: Result<String, AppError> = Err(AppError { detail: String::new() });
    let a = source(); // @source(unwrap-or-else-err)
    let v = r.unwrap_or_else(|_e| a);
    sink(&v); // @sink(unwrap-or-else-err)
}

pub fn or_else_value() {
    let a = source(); // @source(or-else)
    let r: Result<String, AppError> = Err(AppError { detail: a });
    let v = r.or_else(|e| Ok::<String, AppError>(e.detail)).unwrap();
    sink(&v); // @sink(or-else)
}

pub fn ok_and_then_chain() {
    let a = source(); // @source(and-then-chain)
    let v = parse(a).and_then(|x| parse(x)).unwrap();
    sink(&v); // @sink(and-then-chain)
}

pub fn into_sink_through_result() {
    let a = source(); // @source(into-sink)
    if let Ok(v) = parse(a) {
        into_sink(&v);
    }
}
