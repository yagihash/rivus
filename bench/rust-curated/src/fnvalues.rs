use crate::helpers::*;
use crate::{sink, source};

// Functions as values: items, pointers, methods, trait objects in a table.

fn keep(s: String) -> String {
    s
}

fn drop_it(_s: String) -> String {
    String::from("fixed")
}

pub fn fn_item_keep() {
    let a = source(); // @source(fn-keep)
    sink(&transform(a, keep)); // @sink(fn-keep)
}

pub fn fn_item_drop() {
    let a = source(); // @source(fn-drop)
    sink(&transform(a.clone(), drop_it)); // @sink(none) @fp
    sink(&a); // @sink(fn-drop)
}

pub fn fn_pointer_a() {
    let a = source(); // @source(fnptr-a)
    sink(&transform_ptr(a, keep)); // @sink(fnptr-a) @nosink(fnptr-b)
}

pub fn fn_pointer_b() {
    let b = source(); // @source(fnptr-b)
    sink(&transform_ptr(b, keep)); // @sink(fnptr-b) @nosink(fnptr-a)
}

pub fn fn_pointer_variable() {
    let a = source(); // @source(fnptr-var)
    let f: fn(String) -> String = keep;
    sink(&f(a)); // @sink(fnptr-var)
}

pub fn fn_pointer_drop() {
    let a = source(); // @source(fnptr-drop)
    let f: fn(String) -> String = drop_it;
    sink(&f(a.clone())); // @sink(none)
    sink(&a); // @sink(fnptr-drop)
}

pub fn non_capturing_closure_as_pointer() {
    let a = source(); // @source(fnptr-closure)
    let f: fn(String) -> String = |x| x;
    sink(&f(a)); // @sink(fnptr-closure)
}

// the sink as a function value
pub fn sink_as_value() {
    let items = vec![source()]; // @source(sink-value)
    items.iter().for_each(|s| sink(s)); // @sink(sink-value)
}

pub fn sink_pointer() {
    let a = source(); // @source(sink-ptr)
    let f: fn(&str) = sink;
    f(&a); // @sink(sink-ptr)
}

// a path to a method as the callback
pub fn method_path() {
    let items = vec![source()]; // @source(method-path)
    let out: Vec<String> = items.iter().map(String::clone).collect();
    sink(&out[0]); // @sink(method-path)
}

pub fn method_path_to_string() {
    let items = vec![source()]; // @source(to-string)
    let out: Vec<String> = items.iter().map(ToString::to_string).collect();
    sink(&out[0]); // @sink(to-string)
}

// a table of boxed functions
pub fn table_lookup() {
    let a = source(); // @source(table)
    let table: Vec<Box<dyn Fn(String) -> String>> = vec![Box::new(|x| x), Box::new(|_x| String::from("fixed"))];
    sink(&table[0](a)); // @sink(table)
}

pub fn table_map() {
    let a = source(); // @source(table-map)
    let mut table: std::collections::HashMap<&str, Box<dyn Fn(String) -> String>> = std::collections::HashMap::new();
    table.insert("keep", Box::new(|x| x));
    sink(&table["keep"](a)); // @sink(table-map)
}

// a function that returns a function pointer
fn pick(flag: bool) -> fn(String) -> String {
    if flag {
        keep
    } else {
        drop_it
    }
}

pub fn returned_pointer() {
    let a = source(); // @source(returned-ptr)
    sink(&pick(true)(a)); // @sink(returned-ptr)
}

// a function value given to a function that returns it
fn identity_fn(f: fn(String) -> String) -> fn(String) -> String {
    f
}

pub fn through_identity_fn() {
    let a = source(); // @source(identity-fn)
    sink(&identity_fn(keep)(a)); // @sink(identity-fn)
}
