use crate::{sink, source};

// A trait method that a model describes (`curated::modelled::Render::render`, in rules.yaml: the receiver
// goes to the result), called through a generic parameter, so that it is dynamic. The model is applied,
// and the code of the project's own implementations is read as well (ADR-075): what the model gives and
// what the bodies do are joined, so that the model does not hide what the implementations do.

pub trait Render {
    fn render(&self) -> String;
}

// the implementation takes a source of its own: the model has nothing about it
pub struct Leaky;

impl Render for Leaky {
    fn render(&self) -> String {
        source() // @source(model-body)
    }
}

fn show<T: Render>(t: &T) -> String {
    t.render()
}

pub fn model_body() {
    let r = show(&Leaky);
    sink(&r); // @sink(model-body)
}

// a second trait with the same model, and one implementation, so that the two examples do not meet
// in the candidates of the call. The implementation drops what the receiver holds: the model says
// it is passed on.
pub trait Mask {
    fn mask(&self) -> String;
}

pub struct Dropper(pub String);

impl Mask for Dropper {
    fn mask(&self) -> String {
        String::from("fixed")
    }
}

fn masked<T: Mask>(t: &T) -> String {
    t.mask()
}

pub fn model_drop() {
    let d = Dropper(source()); // @source(model-drop)
    let r = masked(&d);
    sink(&r); // @sink(none) @fp
    sink(&d.0); // @sink(model-drop)
}

// `write!(s, ..)` on a `String` is `fmt::Write::write_fmt`: what is formatted goes into the string (the row of the
// standard library; the string has no body of its own to read)
pub fn write_fmt_string() {
    use std::fmt::Write;
    let mut s = String::new();
    write!(s, "{}", source()).unwrap(); // @source(write-fmt-string)
    sink(&s); // @sink(write-fmt-string)
}
