use crate::{sink, source};

// The callbacks of the standard library.

pub fn map_collect() {
    let items = vec![source()]; // @source(map-collect)
    let out: Vec<String> = items.into_iter().map(|s| s.trim().to_string()).collect();
    sink(&out[0]); // @sink(map-collect)
}

pub fn map_other_list() {
    let items = vec![source()]; // @source(map-other)
    let other = vec![String::from("const")];
    let out: Vec<String> = other.into_iter().map(|s| s.trim().to_string()).collect();
    sink(&out[0]); // @sink(none)
    sink(&items[0]); // @sink(map-other)
}

pub fn for_each_sink() {
    let items = vec![source()]; // @source(for-each)
    items.iter().for_each(|s| sink(s)); // @sink(for-each) @noisy(3)
}

pub fn for_each_capture() {
    let mut last = String::new();
    let items = vec![source()]; // @source(for-each-cap) @miss(silent)
    items.into_iter().for_each(|s| last = s);
    sink(&last); // @sink(for-each-cap)
}

pub fn filter_keeps() {
    let items = vec![source()]; // @source(filter)
    let out: Vec<String> = items.into_iter().filter(|s| !s.is_empty()).collect();
    sink(&out[0]); // @sink(filter)
}

pub fn fold_value() {
    let items = vec![source()]; // @source(fold)
    let out = items.into_iter().fold(String::new(), |acc, s| acc + &s);
    sink(&out); // @sink(fold)
}

pub fn flat_map_value() {
    let items = vec![source()]; // @source(flat-map)
    let out: Vec<String> = items.into_iter().flat_map(|s| vec![s.clone(), s]).collect();
    sink(&out[1]); // @sink(flat-map)
}

pub fn map_const_result() {
    let items = vec![source()]; // @source(map-const)
    let out: Vec<String> = items.iter().map(|_s| String::from("fixed")).collect();
    sink(&out[0]); // @sink(none) @fp
    sink(&items[0]); // @sink(map-const)
}

pub fn option_map() {
    let o = Some(source()); // @source(opt-map)
    let r = o.map(|s| s + "!");
    sink(&r.unwrap()); // @sink(opt-map)
}

pub fn option_and_then() {
    let o = Some(source()); // @source(opt-and-then)
    let r = o.and_then(|s| if s.is_empty() { None } else { Some(s) });
    sink(&r.unwrap()); // @sink(opt-and-then)
}

pub fn option_and_then_const() {
    let o = Some(source()); // @source(opt-then-const)
    let r = o.clone().and_then(|_s| Some(String::from("fixed")));
    sink(&r.unwrap()); // @sink(none) @fp
    sink(&o.unwrap()); // @sink(opt-then-const)
}

pub fn option_unwrap_or_else() {
    let o: Option<String> = None;
    let a = source(); // @source(unwrap-or-else)
    let v = o.unwrap_or_else(|| a);
    sink(&v); // @sink(unwrap-or-else)
}

pub fn option_map_or() {
    let o = Some(String::from("c"));
    let a = source(); // @source(map-or)
    let v = o.map_or(a, |s| s);
    sink(&v); // @sink(map-or)
}

pub fn result_map() {
    let r: Result<String, String> = Ok(source()); // @source(res-map)
    let v = r.map(|s| s + "!").unwrap();
    sink(&v); // @sink(res-map)
}

pub fn result_map_err() {
    let r: Result<String, String> = Err(source()); // @source(res-map-err)
    let e = r.map_err(|e| format!("bad {}", e)).unwrap_err();
    sink(&e); // @sink(res-map-err)
}

pub fn any_callback() {
    let items = vec![source()]; // @source(any)
    items.iter().any(|s| {
        sink(s); // @sink(any) @noisy(3)
        true
    });
}

pub fn sort_by_key() {
    let mut items = vec![source(), String::from("b")]; // @source(sort)
    items.sort_by(|a, b| a.cmp(b));
    sink(&items[0]); // @sink(sort)
}

pub fn map_entry_or_insert_with() {
    let mut m: std::collections::HashMap<String, String> = std::collections::HashMap::new();
    let t = source(); // @source(or-insert-with)
    let v = m.entry(String::from("k")).or_insert_with(|| t);
    sink(v); // @sink(or-insert-with)
}

pub fn zip_map() {
    let a = vec![source()]; // @source(zip)
    let b = vec![String::from("c")];
    let out: Vec<String> = a.into_iter().zip(b.into_iter()).map(|(x, y)| x + &y).collect();
    sink(&out[0]); // @sink(zip)
}

pub fn closure_in_loop() {
    let a = source(); // @source(loop)
    let f = |x: &String| x.clone();
    let mut v = String::from("const");
    for _ in 0..3 {
        v = f(&a);
    }
    sink(&v); // @sink(loop)
}
