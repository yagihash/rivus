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

// A trait that the dependency defines and calls through, and a type with a destructor: the impls are in
// the crates that use them, and the destructor of the type is here (yagihash/rivus #41).
pub trait DepHandler {
    fn handle(&self, s: &str);
}

pub fn run(h: &dyn DepHandler, s: String) {
    h.handle(&s)
}

pub trait DepHandler2 {
    fn handle2(&self, s: &str);
}

pub fn run_generic<H: DepHandler2>(h: &H, s: String) {
    h.handle2(&s)
}

static SEEN: std::sync::Mutex<Vec<String>> = std::sync::Mutex::new(Vec::new());

pub struct Guard {
    pub buf: String,
}

impl Drop for Guard {
    fn drop(&mut self) {
        SEEN.lock().unwrap().push(self.buf.clone());
    }
}

pub fn seen() -> Vec<String> {
    SEEN.lock().unwrap().clone()
}
