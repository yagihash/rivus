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

// Generic code of a dependency: the instance that a caller names has type arguments, the function
// the dependency writes out does not (yagihash/rivus #40).
pub fn pass<T>(x: T) -> T {
    x
}

pub struct Holder<T>(pub T);

impl<T> Holder<T> {
    pub fn into_inner(self) -> T {
        self.0
    }
}
