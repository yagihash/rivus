pub mod async_flows;
pub mod std_flows;
pub mod traits;

pub fn source() -> String {
    String::from("x")
}

pub fn sink(_s: &str) {}

pub struct Boxed {
    pub value: String,
}

pub fn direct() {
    let s = source(); // @source(direct)
    sink(&s); // @sink(direct)
}

pub fn field() {
    let b = Boxed { value: source() }; // @source(field)
    sink(&b.value); // @sink(field)
}

pub fn closure() {
    let s = source(); // @source(closure)
    let f = || sink(&s); // @sink(closure)
    f();
}

pub fn branch(c: bool) {
    let mut s = String::from("safe");
    if c {
        s = source(); // @source(branch)
    }
    sink(&s); // @sink(branch)
}

pub fn constant() {
    sink("constant"); // @sink(none)
}

pub fn sanitized() {
    let s = source();
    let t = s.len().to_string();
    sink(&t); // @sink(none)
}

pub fn through_function(x: String) -> String {
    x
}

pub fn via_call() {
    let s = source(); // @source(call)
    let t = through_function(s);
    sink(&t); // @sink(call)
}

pub fn via_reference() {
    let mut s = String::new();
    fill(&mut s);
    sink(&s); // @sink(reference)
}

fn fill(out: &mut String) {
    out.push_str(&source()); // @source(reference)
}

pub fn via_vec() {
    let mut v = Vec::new();
    v.push(source()); // @source(vec)
    sink(&v[0]); // @sink(vec)
}

pub enum Choice {
    Text(String),
    Number(i64),
}

pub fn via_enum() {
    let c = Choice::Text(source()); // @source(enum)
    match c {
        Choice::Text(t) => sink(&t), // @sink(enum)
        Choice::Number(_) => {}
    }
}
