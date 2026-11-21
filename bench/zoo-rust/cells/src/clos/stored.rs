// @cell(F/clos/stored)
// @shape: a closure that captures a value is stored (a field, a static, a Vec) and another function calls it
use std::sync::Mutex;
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/stored/vuln", vuln);
    m::case("F/clos/stored/vuln2", vuln2);
    m::case("F/clos/stored/vuln3", vuln3);
    m::case("F/clos/stored/fixed", fixed);
    m::case("F/clos/stored/decoy", decoy);
}

struct Holder {
    a: Option<Box<dyn Fn()>>,
    b: Option<Box<dyn Fn()>>,
}

fn call_a(h: &Holder) {
    (h.a.as_ref().unwrap())()
}

// a field
fn vuln() { // @vuln
    let v = m::src("clos-stored-vuln"); // @source(clos-stored-vuln) @miss(unmodeled_library_call) @ref(CLOSURE-IN-MEMORY,PARITY)
    let h = Holder {
        a: Some(Box::new(move || {
            m::sink_cmd(&v); // @sink(clos-stored-vuln)
        })),
        b: None,
    };
    call_a(&h);
}

// a static
static G_FN: Mutex<Option<Box<dyn Fn() + Send>>> = Mutex::new(None);

fn call_g() {
    (G_FN.lock().unwrap().as_ref().unwrap())()
}

fn vuln2() { // @vuln
    let v = m::src("clos-stored-vuln2"); // @source(clos-stored-vuln2) @miss(unmodeled_library_call) @ref(CLOSURE-IN-MEMORY,PARITY)
    *G_FN.lock().unwrap() = Some(Box::new(move || {
        m::sink_cmd(&v); // @sink(clos-stored-vuln2)
    }));
    call_g();
}

// a Vec
fn run_all(fs: &[Box<dyn Fn()>]) {
    for f in fs {
        f();
    }
}

fn vuln3() { // @vuln
    let v = m::src("clos-stored-vuln3"); // @source(clos-stored-vuln3) @miss(unmodeled_library_call) @ref(CLOSURE-IN-MEMORY,PARITY)
    let fs: Vec<Box<dyn Fn()>> = vec![Box::new(move || {
        m::sink_cmd(&v); // @sink(clos-stored-vuln3)
    })];
    run_all(&fs);
}

fn fixed() { // @fixed
    let v = m::clean_cmd(&m::src("clos-stored-fixed"));
    let h = Holder {
        a: Some(Box::new(move || {
            m::sink_cmd(&v); // @sink(none)
        })),
        b: None,
    };
    call_a(&h);
}

fn call_both(h: &Holder) {
    (h.a.as_ref().unwrap())();
    (h.b.as_ref().unwrap())();
}

fn decoy() { // @decoy
    let v = m::src("clos-stored-decoy"); // @source(clos-stored-decoy) @miss(unmodeled_library_call) @ref(CLOSURE-IN-MEMORY,PARITY)
    let k = "constant";
    let h = Holder {
        a: Some(Box::new(move || {
            m::sink_cmd(k); // @sink(none) @nosink(clos-stored-decoy)
        })),
        b: Some(Box::new(move || {
            m::sink_sql(&v); // @sink(clos-stored-decoy)
        })),
    };
    call_both(&h);
}
