// @cell(F/dyn/func-field)
// @shape: a function value kept in a field of a struct is called; the struct has another function field with another sink
use zoo_mark as m;

// Each case has its own argument type (a newtype of the marker), so that the cases of the cell do not meet at a call of
// Fn::call, whose candidates are the closures of the same argument types (the same type in one case is the point of the decoy).
struct Tv(String);
struct Tf(String);
struct Td(String);

pub fn register() {
    m::case("F/dyn/func-field/vuln", vuln);
    m::case("F/dyn/func-field/fixed", fixed);
    m::case("F/dyn/func-field/decoy", decoy);
}

struct HandlersV {
    cmd: Option<Box<dyn Fn(&Tv)>>,
}

struct HandlersF {
    cmd: Option<Box<dyn Fn(&Tf)>>,
}

struct HandlersD {
    cmd: Option<Box<dyn Fn(&Td)>>,
    sql: Option<Box<dyn Fn(&Td)>>,
}

fn vuln() { // @vuln
    let v = Tv(m::src("dyn-func-field-vuln")); // @source(dyn-func-field-vuln) @miss(unmodeled_library_call) @ref(FUNC-VALUE-MEMORY,PARITY)
    let h = HandlersV {
        cmd: Some(Box::new(|x| {
            m::sink_cmd(&x.0); // @sink(dyn-func-field-vuln)
        })),
    };
    (h.cmd.as_ref().unwrap())(&v);
}

fn fixed() { // @fixed
    let v = Tf(m::clean_cmd(&m::src("dyn-func-field-fixed")));
    let h = HandlersF {
        cmd: Some(Box::new(|x| {
            m::sink_cmd(&x.0); // @sink(none)
        })),
    };
    (h.cmd.as_ref().unwrap())(&v);
}

fn decoy() { // @decoy
    let v = Td(m::src("dyn-func-field-decoy")); // @source(dyn-func-field-decoy) @miss(unmodeled_library_call) @ref(FUNC-VALUE-MEMORY,PARITY)
    let h = HandlersD {
        cmd: Some(Box::new(|x| {
            m::sink_cmd(&x.0); // @sink(none) @nosink(dyn-func-field-decoy)
        })),
        sql: Some(Box::new(|x| {
            m::sink_sql(&x.0); // @sink(dyn-func-field-decoy)
        })),
    };
    (h.cmd.as_ref().unwrap())(&Td("constant".to_string()));
    (h.sql.as_ref().unwrap())(&v);
}
