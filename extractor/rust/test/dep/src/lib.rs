// A crate outside the workspace of the sample: a dependency. Its functions have
// bodies that the analysis reads, so no model is needed for them.
pub fn wrap(s: &str) -> String {
    format!("<{}>", s)
}

pub fn constant(_s: &str) -> String {
    String::from("fixed")
}

pub fn never_called(s: &str) -> String {
    s.to_string()
}
