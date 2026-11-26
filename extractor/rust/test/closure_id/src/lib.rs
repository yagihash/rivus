// The closures whose environment is an alloc (the extractor writes the ID of the function of the closure on it): one that
// captures nothing, one that captures a value, and a macro that writes a closure twice at one place.

pub fn run_with(f: impl Fn(&str)) {
    f("");
}

pub fn sink(_: &str) {}

pub fn nothing_captured() {
    run_with(|s: &str| sink(s));
}

pub fn captured(v: String) {
    run_with(move |s: &str| sink(&v[..s.len()]));
}

macro_rules! twice {
    () => {
        run_with(|s: &str| sink(s))
    };
}

pub fn macro_twice() {
    twice!();
    twice!();
}
