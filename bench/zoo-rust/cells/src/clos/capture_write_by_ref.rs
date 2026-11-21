// @cell(F/clos/capture-write-by-ref)
// @shape: a closure captures a variable: written after the closure is made (a RefCell: the borrow rules allow no other way) or written by the closure and read outside (a `&mut` capture); also a variable that a loop moves into each closure, and the closure given to a helper of the project or to Once::call_once
use std::cell::RefCell;
use std::sync::Once;
use zoo_mark as m;

pub fn register() {
    m::case("F/clos/capture-write-by-ref/vuln", vuln);
    m::case("F/clos/capture-write-by-ref/vuln2", vuln2);
    m::case("F/clos/capture-write-by-ref/vuln3", vuln3);
    m::case("F/clos/capture-write-by-ref/vuln4", vuln4);
    m::case("F/clos/capture-write-by-ref/vuln5", vuln5);
    m::case("F/clos/capture-write-by-ref/vuln6", vuln6);
    m::case("F/clos/capture-write-by-ref/vuln7", vuln7);
    m::case("F/clos/capture-write-by-ref/fixed", fixed);
    m::case("F/clos/capture-write-by-ref/decoy", decoy);
}

fn run(f: impl FnOnce()) {
    f()
}

// vuln: written after the closure is made, then the closure is called.
fn vuln() { // @vuln
    let x = RefCell::new(String::new());
    let f = || {
        m::sink_cmd(&x.borrow()); // @sink(clos-capture-write-by-ref-vuln) @noisy(4) @ref(ENV-PTR)
    };
    *x.borrow_mut() = m::src("clos-capture-write-by-ref-vuln"); // @source(clos-capture-write-by-ref-vuln)
    f();
}

// vuln2: the closure writes the variable through a `&mut` capture; the outside reads it.
fn vuln2() { // @vuln
    let mut x = String::new();
    let mut f = || {
        x = m::src("clos-capture-write-by-ref-vuln2"); // @source(clos-capture-write-by-ref-vuln2) @miss(unmodeled_library_call) @ref(ENV-PTR,PARITY)
    };
    f();
    m::sink_cmd(&x); // @sink(clos-capture-write-by-ref-vuln2)
}

// vuln3: the variable of a loop is moved into each closure; the closures are called later.
fn vuln3() { // @vuln
    let mut fs: Vec<Box<dyn Fn()>> = Vec::new();
    for s in vec![m::src("clos-capture-write-by-ref-vuln3")] { // @source(clos-capture-write-by-ref-vuln3) @miss(unmodeled_library_call) @ref(FUNC-VALUE-MEMORY,PARITY)
        fs.push(Box::new(move || {
            m::sink_cmd(&s); // @sink(clos-capture-write-by-ref-vuln3) @noisy(4) @ref(ENV-PTR)
        }));
    }
    for f in &fs {
        f();
    }
}

// vuln4: written after the closure is made; a helper of the project calls it.
fn vuln4() { // @vuln
    let x = RefCell::new(String::new());
    let f = || {
        m::sink_cmd(&x.borrow()); // @sink(clos-capture-write-by-ref-vuln4) @noisy(4) @ref(ENV-PTR)
    };
    *x.borrow_mut() = m::src("clos-capture-write-by-ref-vuln4"); // @source(clos-capture-write-by-ref-vuln4)
    run(f);
}

// vuln5: written after the closure is made; Once::call_once calls it.
fn vuln5() { // @vuln
    let x = RefCell::new(String::new());
    let f = || {
        m::sink_cmd(&x.borrow()); // @sink(clos-capture-write-by-ref-vuln5) @noisy(4) @ref(ENV-PTR)
    };
    *x.borrow_mut() = m::src("clos-capture-write-by-ref-vuln5"); // @source(clos-capture-write-by-ref-vuln5) @miss(unmodeled_library_call) @ref(CALLBACK-MODEL,PARITY)
    let once = Once::new();
    once.call_once(f);
}

// vuln6: the closure writes, a helper of the project calls it; the outside reads.
fn vuln6() { // @vuln
    let mut x = String::new();
    run(|| {
        x = m::src("clos-capture-write-by-ref-vuln6"); // @source(clos-capture-write-by-ref-vuln6) @miss(unmodeled_library_call) @ref(ENV-PTR,PARITY)
    });
    m::sink_cmd(&x); // @sink(clos-capture-write-by-ref-vuln6)
}

// vuln7: the closure writes, Once::call_once calls it; the outside reads.
fn vuln7() { // @vuln
    let mut x = String::new();
    let once = Once::new();
    once.call_once(|| {
        x = m::src("clos-capture-write-by-ref-vuln7"); // @source(clos-capture-write-by-ref-vuln7) @miss(unmodeled_library_call) @ref(CALLBACK-MODEL,PARITY)
    });
    m::sink_cmd(&x); // @sink(clos-capture-write-by-ref-vuln7)
}

fn fixed() { // @fixed
    let x = RefCell::new(String::new());
    let f = || {
        m::sink_cmd(&x.borrow()); // @sink(none) @fp @ref(ENV-PTR)
    };
    *x.borrow_mut() = m::clean_cmd(&m::src("clos-capture-write-by-ref-fixed"));
    f();
    let mut y = String::new();
    run(|| {
        y = m::clean_cmd(&m::src("clos-capture-write-by-ref-fixed"));
    });
    m::sink_cmd(&y); // @sink(none)
}

fn decoy() { // @decoy
    let x = RefCell::new(String::new());
    let y = "constant";
    let set = || {
        *x.borrow_mut() = m::src("clos-capture-write-by-ref-decoy"); // @source(clos-capture-write-by-ref-decoy)
    };
    let read = || {
        m::sink_sql(&x.borrow()); // @sink(clos-capture-write-by-ref-decoy) @noisy(3) @ref(ENV-PTR)
    };
    let quiet = || {
        m::sink_cmd(y); // @sink(none) @fp @nosink(clos-capture-write-by-ref-decoy) @fp @ref(ENV-PTR,PARITY)
    };
    run(set);
    run(read);
    run(quiet);
}
