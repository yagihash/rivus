//! The neutral IR as the Go side reads it (internal/ir: JSON Lines,
//! "rivus.ir/0"). Only what the extractor writes is here.

use serde_json::{json, Map, Value};
use std::collections::{BTreeMap, BTreeSet};

pub const SCHEMA_VERSION: &str = "rivus.ir/0";

/// A position. Rust spans give lines and columns, not byte offsets, so the
/// offset is a number made of the two (the core only compares offsets of the
/// same file, and uses them as keys).
#[derive(Clone, Default)]
pub struct Pos {
    pub file: String,
    pub line: usize,
    pub col: usize,
}

impl Pos {
    pub fn none() -> Pos {
        Pos::default()
    }
    pub fn is_none(&self) -> bool {
        self.file.is_empty()
    }
    pub fn offset(&self) -> i64 {
        (self.line as i64) * 100_000 + self.col as i64
    }
    pub fn to_json(&self) -> Value {
        if self.is_none() {
            return json!({});
        }
        json!({"file": self.file, "offset": self.offset(), "line": self.line, "col": self.col})
    }
}

pub struct Local {
    pub name: String,
    pub ty: String,
    pub class: &'static str,
}

impl Local {
    pub fn to_json(&self) -> Value {
        let mut m = Map::new();
        if !self.name.is_empty() {
            m.insert("name".into(), json!(self.name));
        }
        if !self.ty.is_empty() {
            m.insert("type".into(), json!(self.ty));
        }
        if !self.class.is_empty() {
            m.insert("class".into(), json!(self.class));
        }
        Value::Object(m)
    }
}

#[derive(Default)]
pub struct Block {
    pub instrs: Vec<Value>,
    pub term: Option<Value>,
    pub succs: Vec<Value>,
}

impl Block {
    pub fn to_json(&self, id: usize) -> Value {
        let mut m = Map::new();
        m.insert("id".into(), json!(id));
        if !self.instrs.is_empty() {
            m.insert("instrs".into(), Value::Array(self.instrs.clone()));
        }
        m.insert("term".into(), self.term.clone().unwrap_or_else(|| json!({"kind": "unreachable"})));
        if !self.succs.is_empty() {
            m.insert("succs".into(), Value::Array(self.succs.clone()));
        }
        Value::Object(m)
    }
}

pub struct Func {
    pub id: String,
    pub symbol: String,
    pub exported: bool,
    pub entry: bool,
    /// Parameter 0 is the receiver of a method (for a closure, the closure itself).
    pub recv: bool,
    pub pos: Pos,
    pub params: Vec<Value>,
    pub results: Vec<String>,
    pub locals: Vec<Local>,
    pub blocks: Vec<Block>,
}

impl Func {
    pub fn to_json(&self) -> Value {
        let mut m = Map::new();
        m.insert("id".into(), json!(self.id));
        if !self.params.is_empty() {
            m.insert("params".into(), Value::Array(self.params.clone()));
        }
        if !self.results.is_empty() {
            m.insert("results".into(), json!(self.results));
        }
        m.insert("locals".into(), Value::Array(self.locals.iter().map(|l| l.to_json()).collect()));
        m.insert("blocks".into(), Value::Array(self.blocks.iter().enumerate().map(|(i, b)| b.to_json(i)).collect()));
        m.insert("roots".into(), json!([0]));
        if !self.pos.is_none() {
            m.insert("pos".into(), self.pos.to_json());
            m.insert("file".into(), json!(self.pos.file));
        }
        if self.recv {
            m.insert("recv".into(), json!(true));
        }
        if self.entry {
            m.insert("entry".into(), json!(true));
        }
        if self.exported {
            m.insert("exported".into(), json!(true));
        }
        if !self.symbol.is_empty() {
            m.insert("symbol".into(), json!(self.symbol));
        }
        let mut v = Value::Object(m);
        if !self.pos.is_none() {
            strip_file(&mut v, &self.pos.file, true);
        }
        v
    }
}

/// Leaves the file out of the positions that are in the file of the function
/// (the function says it once). `top` keeps the file of the function itself.
fn strip_file(v: &mut Value, file: &str, top: bool) {
    match v {
        Value::Object(m) => {
            if !top && m.contains_key("offset") && m.get("file").and_then(|f| f.as_str()) == Some(file) {
                m.remove("file");
            }
            for (k, x) in m.iter_mut() {
                if top && k == "file" {
                    continue;
                }
                strip_file(x, file, false);
            }
        }
        Value::Array(a) => a.iter_mut().for_each(|x| strip_file(x, file, false)),
        _ => {}
    }
}

/// The texts, grouped as the stream has them: one text for every position it
/// belongs to.
#[derive(Default)]
pub struct Texts {
    by_kind: BTreeMap<(String, String, String), BTreeSet<i64>>, // (kind, file, text) -> offsets
    calls: BTreeMap<(String, i64), (String, Vec<String>)>,
    funcs: Vec<(String, String, String)>,
}

impl Texts {
    pub fn add(&mut self, kind: &str, p: &Pos, text: String) {
        if text.is_empty() || p.is_none() {
            return;
        }
        self.by_kind.entry((kind.into(), p.file.clone(), text)).or_default().insert(p.offset());
    }
    pub fn call(&mut self, p: &Pos, call: String, args: Vec<String>) {
        if p.is_none() {
            return;
        }
        self.calls.insert((p.file.clone(), p.offset()), (call, args));
    }
    pub fn func(&mut self, id: String, body: String, source: String) {
        self.funcs.push((id, body, source));
    }

    pub fn write(&self, out: &mut Vec<Value>) {
        for kind in ["cond", "expr", "snippet"] {
            for ((k, file, text), offsets) in &self.by_kind {
                if k == kind {
                    out.push(json!({"kind": "text", "text": {"kind": k, "file": file, "text": text, "offsets": offsets.iter().collect::<Vec<_>>()}}));
                }
            }
        }
        for ((file, offset), (call, args)) in &self.calls {
            let mut c = Map::new();
            c.insert("file".into(), json!(file));
            c.insert("offset".into(), json!(offset));
            if !call.is_empty() {
                c.insert("call".into(), json!(call));
            }
            if !args.is_empty() {
                c.insert("args".into(), json!(args));
            }
            out.push(json!({"kind": "call_text", "call_text": c}));
        }
        for (id, body, source) in &self.funcs {
            let mut c = Map::new();
            c.insert("func".into(), json!(id));
            if !body.is_empty() {
                c.insert("body".into(), json!(body));
            }
            if !source.is_empty() {
                c.insert("source".into(), json!(source));
            }
            out.push(json!({"kind": "func_text", "func_text": c}));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn at(file: &str, line: usize, col: usize) -> Pos {
        Pos { file: file.to_string(), line, col }
    }

    // a text is written once for all the positions it belongs to, in the order the stream has: cond, expr, snippet
    #[test]
    fn texts_are_grouped_by_text_and_written_in_the_order_of_kinds() {
        let mut t = Texts::default();
        t.add("snippet", &at("a.rs", 1, 10), "x();".into());
        t.add("expr", &at("a.rs", 1, 10), "x()".into());
        t.add("expr", &at("a.rs", 3, 30), "x()".into());
        t.add("cond", &at("a.rs", 1, 5), "a".into());
        t.add("expr", &Pos::none(), "nowhere".into());
        t.add("expr", &at("a.rs", 4, 40), String::new());
        let mut out = Vec::new();
        t.write(&mut out);
        let kinds: Vec<&str> = out.iter().map(|v| v["text"]["kind"].as_str().unwrap_or("")).collect();
        assert_eq!(kinds, vec!["cond", "expr", "snippet"]);
        assert_eq!(out[1]["text"]["offsets"], json!([100_010, 300_030]));
    }
}
