// Calls the functions of std whose built-in model has a row of calls (the function calls a closure it is given)
// with a closure that records what it was called with, and writes what came out as JSON.
// cmd/rivus TestZooCallbacksRust holds the rows to it (the same check as Go and Dart, zoo_callbacks_test.go): the
// answer is the run, not the model. No dependency: `rustc observe.rs`.
//
// The receiver (the first argument: Rust names it arg0) has the marker RECV; what the closures return is "Ж".
use std::cell::RefCell;
use std::sync::{Arc, Mutex, Once};

#[derive(Default)]
struct Rec {
    calls: usize,
    params: Vec<String>,
}

impl Rec {
    fn see(&mut self, p: &[String]) {
        if self.calls == 0 {
            self.params = p.to_vec();
        }
        self.calls += 1;
    }
}

fn s(x: &str) -> String {
    x.to_string()
}

fn json(x: &str) -> String {
    let mut o = String::from("\"");
    for c in x.chars() {
        match c {
            '"' => o.push_str("\\\""),
            '\\' => o.push_str("\\\\"),
            '\n' => o.push_str("\\n"),
            c => o.push(c),
        }
    }
    o.push('"');
    o
}

fn report(out: &mut Vec<String>, symbol: &str, markers: &[(&str, &str)], r: &Rec, result: String) {
    let ms: Vec<String> = markers.iter().map(|(k, v)| format!("{}:{}", json(k), json(v))).collect();
    let ps: Vec<String> = r.params.iter().map(|p| json(p)).collect();
    out.push(format!(
        "{{\"symbol\":{},\"markers\":{{{}}},\"calls\":{},\"params\":[{}],\"result\":{}}}",
        json(symbol),
        ms.join(","),
        r.calls,
        ps.join(","),
        json(&result)
    ));
}

const RECV: [(&str, &str); 1] = [("arg0", "RECV")];

fn main() {
    let mut out: Vec<String> = Vec::new();
    macro_rules! case {
        ($sym:expr, $markers:expr, |$r:ident| $body:expr) => {{
            let cell = RefCell::new(Rec::default());
            let res = {
                let $r = &cell;
                $body
            };
            report(&mut out, $sym, &$markers, &cell.borrow(), format!("{:?}", res));
        }};
    }
    case!("std::sync::Once::call_once", [], |r| {
        let once = Once::new();
        once.call_once(|| r.borrow_mut().see(&[]));
    });
    // a provided method of a trait calls a method of the same trait on its receiver: `write_all` calls `write`
    // (a row with a method: the receiver is arg0 and what it is called with is arg1)
    case!("std::io::Write::write_all", [("arg1", "DATA")], |r| {
        struct W<'a>(&'a RefCell<Rec>);
        impl std::io::Write for W<'_> {
            fn write(&mut self, p: &[u8]) -> std::io::Result<usize> {
                self.0.borrow_mut().see(&[String::from_utf8_lossy(p).to_string()]);
                Ok(p.len())
            }
            fn flush(&mut self) -> std::io::Result<()> {
                Ok(())
            }
        }
        std::io::Write::write_all(&mut W(r), b"DATA")
    });
    case!("std::option::Option::map", RECV, |r| Some(s("RECV")).map(|x| {
        r.borrow_mut().see(&[x.clone()]);
        s("Ж")
    }));
    case!("std::option::Option::and_then", RECV, |r| Some(s("RECV")).and_then(|x| {
        r.borrow_mut().see(&[x.clone()]);
        Some(s("Ж"))
    }));
    case!("std::option::Option::ok_or_else", RECV, |r| None::<String>.ok_or_else(|| {
        r.borrow_mut().see(&[]);
        s("Ж")
    }));
    case!("std::option::Option::unwrap_or_else", RECV, |r| None::<String>.unwrap_or_else(|| {
        r.borrow_mut().see(&[]);
        s("Ж")
    }));
    case!("std::option::Option::filter", RECV, |r| Some(s("RECV")).filter(|x| {
        r.borrow_mut().see(&[x.clone()]);
        true
    }));
    case!("std::result::Result::map", RECV, |r| Ok::<String, String>(s("RECV")).map(|x| {
        r.borrow_mut().see(&[x.clone()]);
        s("Ж")
    }));
    case!("std::result::Result::map_err", RECV, |r| Err::<String, String>(s("RECV")).map_err(|x| {
        r.borrow_mut().see(&[x.clone()]);
        s("Ж")
    }));
    case!("std::result::Result::and_then", RECV, |r| Ok::<String, String>(s("RECV")).and_then(|x| {
        r.borrow_mut().see(&[x.clone()]);
        Ok::<String, String>(s("Ж"))
    }));
    case!("std::result::Result::unwrap_or_else", RECV, |r| Err::<String, String>(s("RECV")).unwrap_or_else(|x| {
        r.borrow_mut().see(&[x.clone()]);
        s("Ж")
    }));
    case!("std::iter::Iterator::map", RECV, |r| vec![s("RECV")]
        .into_iter()
        .map(|x| {
            r.borrow_mut().see(&[x.clone()]);
            s("Ж")
        })
        .collect::<Vec<String>>());
    case!("std::iter::Iterator::filter", RECV, |r| vec![s("RECV")]
        .into_iter()
        .filter(|x| {
            r.borrow_mut().see(&[x.clone()]);
            true
        })
        .collect::<Vec<String>>());
    case!("std::iter::Iterator::filter_map", RECV, |r| vec![s("RECV")]
        .into_iter()
        .filter_map(|x| {
            r.borrow_mut().see(&[x.clone()]);
            Some(s("Ж"))
        })
        .collect::<Vec<String>>());
    case!("std::iter::Iterator::flat_map", RECV, |r| vec![s("RECV")]
        .into_iter()
        .flat_map(|x| {
            r.borrow_mut().see(&[x.clone()]);
            vec![s("Ж")]
        })
        .collect::<Vec<String>>());
    case!("std::iter::Iterator::for_each", RECV, |r| vec![s("RECV")].into_iter().for_each(|x| {
        r.borrow_mut().see(&[x.clone()]);
    }));
    case!("std::iter::Iterator::any", RECV, |r| vec![s("RECV")].into_iter().any(|x| {
        r.borrow_mut().see(&[x.clone()]);
        false
    }));
    case!("std::iter::Iterator::all", RECV, |r| vec![s("RECV")].into_iter().all(|x| {
        r.borrow_mut().see(&[x.clone()]);
        true
    }));
    case!("std::iter::Iterator::find", RECV, |r| vec![s("RECV")].into_iter().find(|x| {
        r.borrow_mut().see(&[x.clone()]);
        true
    }));
    case!("std::iter::Iterator::position", RECV, |r| vec![s("RECV")].into_iter().position(|x| {
        r.borrow_mut().see(&[x.clone()]);
        true
    }));
    case!("std::iter::Iterator::take_while", RECV, |r| vec![s("RECV")]
        .into_iter()
        .take_while(|x| {
            r.borrow_mut().see(&[x.clone()]);
            true
        })
        .collect::<Vec<String>>());
    {
        let rec = Arc::new(Mutex::new(Rec::default()));
        let r2 = Arc::clone(&rec);
        std::thread::spawn(move || r2.lock().unwrap().see(&[])).join().unwrap();
        let r = rec.lock().unwrap();
        report(&mut out, "std::thread::spawn", &[], &r, String::new());
    }
    println!("[{}]", out.join(","));
}
