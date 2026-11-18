//! Lowering of a MIR body (rustc_public) to the neutral IR.
//!
//! MIR is a control flow graph of blocks over mutable locals, the form the IR
//! has. What is decided here:
//!  - a local whose address is taken (`&x`) lives in a cell (`alloc`, `load`,
//!    `store`); the other locals are IR locals;
//!  - a place with a projection is read and written through `field_load`,
//!    `field_store`, `index_*`, `load` and `store`; `(*p).f` is one access;
//!  - an enum variant's field is named `Variant.field`, a tuple's `0`, `1`...;
//!  - unwinding (cleanup blocks, `Drop`, `Assert`) is not drawn: a panic is
//!    not a flow inside the function;
//!  - the call of a closure is resolved by its type, and its tuple of
//!    arguments is taken apart.

use crate::irjson::{Block, Func, Local, Pos, Texts};
use rustc_public::mir::mono::{Instance, InstanceKind};
use rustc_public::mir::*;
use rustc_public::ty::*;
use rustc_public::CrateDef;
use serde_json::{json, Map, Value};
use std::collections::{HashMap, HashSet};
use std::rc::Rc;

/// What lowering needs of the whole crate.
pub struct Ctx {
    pub root: String,
    pub files: HashMap<String, Rc<Vec<String>>>,
    pub texts: Texts,
    /// The names of the items that have a body here.
    pub bodies: HashSet<String>,
}

impl Ctx {
    pub fn new(root: String, bodies: HashSet<String>) -> Ctx {
        Ctx { root, files: HashMap::new(), texts: Texts::default(), bodies }
    }

    fn lines(&mut self, path: &str) -> Rc<Vec<String>> {
        if !self.files.contains_key(path) {
            let text = std::fs::read_to_string(path).unwrap_or_default();
            self.files.insert(path.to_string(), Rc::new(text.lines().map(|l| l.to_string()).collect()));
        }
        self.files[path].clone()
    }

    /// The position of a span, and the texts the core will ask for.
    pub fn pos(&mut self, span: &Span) -> Pos {
        let file = span.get_filename().to_string();
        if file.is_empty() {
            return Pos::none();
        }
        let li = span.get_lines();
        let rel = match std::path::Path::new(&file).strip_prefix(&self.root) {
            Ok(r) => r.to_string_lossy().to_string(),
            Err(_) => file.clone(),
        };
        let p = Pos { file: rel, line: li.start_line, col: li.start_col };
        let lines = self.lines(&file);
        self.texts.add("snippet", &p, snippet(&lines, li.start_line, li.end_line));
        self.texts.add("cond", &p, cond_of(&lines, li.start_line));
        p
    }

    pub fn expr_text(&mut self, span: &Span) -> String {
        let file = span.get_filename().to_string();
        let li = span.get_lines();
        let lines = self.lines(&file);
        span_text(&lines, li.start_line, li.start_col, li.end_line, li.end_col)
    }
}

fn snippet(lines: &[String], start: usize, end: usize) -> String {
    if start == 0 || start > lines.len() {
        return String::new();
    }
    let last = end.min(start + 11).min(lines.len());
    let chosen: Vec<&str> = lines[start - 1..last].iter().map(|s| s.as_str()).collect();
    let indent = chosen.iter().filter(|l| !l.trim().is_empty()).map(|l| l.len() - l.trim_start().len()).min().unwrap_or(0);
    chosen.iter().map(|l| if l.len() >= indent { l[indent..].trim_end() } else { l.trim() }).collect::<Vec<_>>().join("\n")
}

/// The condition of an `if` or a `while` that starts on the line, if any.
fn cond_of(lines: &[String], line: usize) -> String {
    if line == 0 || line > lines.len() {
        return String::new();
    }
    let l = lines[line - 1].trim();
    for kw in ["if let ", "if ", "while let ", "while ", "} else if "] {
        if let Some(rest) = l.strip_prefix(kw).or_else(|| l.find(kw).map(|i| &l[i + kw.len()..])) {
            let rest = rest.trim_end_matches('{').trim();
            return rest.to_string();
        }
    }
    String::new()
}

fn span_text(lines: &[String], sl: usize, sc: usize, el: usize, ec: usize) -> String {
    if sl == 0 || sl > lines.len() || el > lines.len() {
        return String::new();
    }
    let mut out = String::new();
    for l in sl..=el {
        let text: Vec<char> = lines[l - 1].chars().collect();
        let from = if l == sl { sc.saturating_sub(1).min(text.len()) } else { 0 };
        let to = if l == el { ec.saturating_sub(1).min(text.len()) } else { text.len() };
        if from < to {
            out.extend(&text[from..to]);
        }
        out.push(' ');
    }
    out.split_whitespace().collect::<Vec<_>>().join(" ")
}

/// How the engine sees a type.
fn class_of(ty: &Ty) -> &'static str {
    match ty.kind() {
        TyKind::RigidTy(r) => match r {
            RigidTy::Bool => "bool",
            RigidTy::Int(_) | RigidTy::Uint(_) | RigidTy::Float(_) | RigidTy::Char => "number",
            RigidTy::Str => "string",
            RigidTy::Ref(_, _, _) | RigidTy::RawPtr(_, _) => "pointer",
            RigidTy::Array(_, _) | RigidTy::Slice(_) => "container",
            RigidTy::Tuple(_) => "struct",
            RigidTy::FnDef(_, _) | RigidTy::FnPtr(_) | RigidTy::Closure(_, _) => "func",
            RigidTy::Adt(def, _) => match def.name().as_str() {
                "std::string::String" | "alloc::string::String" => "string",
                "std::vec::Vec" | "alloc::vec::Vec" | "std::collections::HashMap" | "std::collections::hash::map::HashMap"
                | "std::collections::HashSet" | "std::collections::hash::set::HashSet" | "std::collections::VecDeque"
                | "std::collections::BTreeMap" | "std::collections::BTreeSet" | "alloc::collections::vec_deque::VecDeque" => "container",
                "std::boxed::Box" | "alloc::boxed::Box" | "std::rc::Rc" | "std::sync::Arc" | "alloc::rc::Rc" | "alloc::sync::Arc" => "pointer",
                _ => "struct",
            },
            _ => "other",
        },
        _ => "other",
    }
}

/// The symbol of a function as the rules write it: the path, without the
/// generic parameters of its impl (`Option::<T>::unwrap` is `Option::unwrap`).
fn clean_symbol(name: &str) -> String {
    if name.starts_with('<') {
        return name.to_string();
    }
    // core and alloc are the parts of std that std re-exports: rules say std
    let name = if let Some(r) = name.strip_prefix("core::").or_else(|| name.strip_prefix("alloc::")) { format!("std::{}", r) } else { name.to_string() };
    let name = name.as_str();
    let mut out = String::new();
    let b: Vec<char> = name.chars().collect();
    let mut i = 0;
    while i < b.len() {
        if i + 2 < b.len() && b[i] == ':' && b[i + 1] == ':' && b[i + 2] == '<' {
            let mut depth = 0;
            let mut j = i + 2;
            while j < b.len() {
                match b[j] {
                    '<' => depth += 1,
                    '>' => {
                        depth -= 1;
                        if depth == 0 {
                            break;
                        }
                    }
                    _ => {}
                }
                j += 1;
            }
            // the "::" that follows the generics goes with them
            i = j + 1;
            if i + 1 < b.len() && b[i] == ':' && b[i + 1] == ':' {
                out.push_str("::");
                i += 2;
            }
            continue;
        }
        out.push(b[i]);
        i += 1;
    }
    out
}

/// A type whose fields are the machinery of a pointer, not data.
fn is_smart_pointer(ty: &Ty) -> bool {
    if let TyKind::RigidTy(RigidTy::Adt(def, _)) = ty.kind() {
        let name = def.name();
        let last = name.rsplit("::").next().unwrap_or("");
        return matches!(last, "Box" | "BoxRaw" | "Unique" | "NonNull" | "Rc" | "Arc");
    }
    false
}

/// The identity of a type for the IR: the path of a struct or an enum without
/// its type arguments, `&` and `*` kept: `axum::extract::Query<P>` is
/// `axum::extract::Query`, the type a rule names.
fn type_id(ty: &Ty) -> String {
    match ty.kind() {
        TyKind::RigidTy(RigidTy::Adt(def, _)) => clean_symbol(&def.name()),
        TyKind::RigidTy(RigidTy::Ref(_, t, m)) => format!("&{}{}", if matches!(m, Mutability::Mut) { "mut " } else { "" }, type_id(&t)),
        TyKind::RigidTy(RigidTy::RawPtr(t, _)) => format!("*{}", type_id(&t)),
        _ => format!("{}", ty),
    }
}

fn bin_op(op: &BinOp) -> &'static str {
    match op {
        BinOp::Add | BinOp::AddUnchecked | BinOp::Offset => "+",
        BinOp::Sub | BinOp::SubUnchecked => "-",
        BinOp::Mul | BinOp::MulUnchecked => "*",
        BinOp::Div => "/",
        BinOp::Rem => "%",
        BinOp::BitXor => "^",
        BinOp::BitAnd => "&",
        BinOp::BitOr => "|",
        BinOp::Shl | BinOp::ShlUnchecked => "<<",
        BinOp::Shr | BinOp::ShrUnchecked => ">>",
        BinOp::Eq => "==",
        BinOp::Lt => "<",
        BinOp::Le => "<=",
        BinOp::Ne => "!=",
        BinOp::Ge => ">=",
        BinOp::Gt => ">",
        BinOp::Cmp => "cmp",
    }
}

/// Where a place is while it is walked: at an address (a pointer to it), or
/// as a value.
#[derive(Clone, Copy)]
enum Cur {
    Ptr(usize),
    Val(usize),
}

struct Walk {
    cur: Cur,
    ty: Ty,
    variant: Option<VariantIdx>,
}

pub struct Lower<'a> {
    pub cx: &'a mut Ctx,
    body: &'a Body,
    f: Func,
    cur: usize,
    var: Vec<usize>,
    cells: HashSet<usize>,
    span: Span,
}

impl<'a> Lower<'a> {
    pub fn lower(cx: &'a mut Ctx, id: String, body: &'a Body, item_span: Span) -> Func {
        let pos = cx.pos(&item_span);
        let mut l = Lower {
            cx,
            body,
            f: Func { id: id.clone(), symbol: id, exported: true, entry: false, pos, params: vec![], results: vec![], locals: vec![], blocks: vec![] },
            cur: 0,
            var: vec![],
            cells: HashSet::new(),
            span: item_span,
        };
        l.run();
        l.f
    }

    // --- plumbing -----------------------------------------------------------

    fn new_local(&mut self, name: &str, ty: Option<&Ty>) -> usize {
        let (t, c) = match ty {
            Some(t) => (type_id(t), class_of(t)),
            None => (String::new(), ""),
        };
        self.f.locals.push(Local { name: name.to_string(), ty: t, class: c });
        self.f.locals.len() - 1
    }

    fn pointer_local(&mut self, name: &str) -> usize {
        self.f.locals.push(Local { name: name.to_string(), ty: String::new(), class: "pointer" });
        self.f.locals.len() - 1
    }

    fn emit(&mut self, op: &str, dsts: &[usize], args: &[usize], pos: &Pos, extra: Value) {
        let mut m = Map::new();
        m.insert("op".into(), json!(op));
        if !dsts.is_empty() {
            m.insert("dsts".into(), json!(dsts));
        }
        if !args.is_empty() {
            m.insert("args".into(), json!(args));
        }
        if !pos.is_none() {
            m.insert("pos".into(), pos.to_json());
        }
        if let Value::Object(e) = extra {
            for (k, v) in e {
                m.insert(k, v);
            }
        }
        self.f.blocks[self.cur].instrs.push(Value::Object(m));
    }

    fn here(&mut self) -> Pos {
        let s = self.span.clone();
        self.cx.pos(&s)
    }

    fn new_block(&mut self) -> usize {
        self.f.blocks.push(Block::default());
        self.f.blocks.len() - 1
    }

    fn jump(&mut self, to: usize) {
        let b = &mut self.f.blocks[self.cur];
        b.term = Some(json!({"kind": "jump"}));
        b.succs.push(json!({"to": to, "kind": "normal"}));
    }

    fn branch(&mut self, cond: usize, t: usize, f: usize, pos: &Pos) {
        let b = &mut self.f.blocks[self.cur];
        let mut term = json!({"kind": "branch", "cond": cond});
        if !pos.is_none() {
            term["cond_pos"] = pos.to_json();
        }
        b.term = Some(term);
        b.succs.push(json!({"to": t, "kind": "true"}));
        b.succs.push(json!({"to": f, "kind": "false"}));
    }

    fn constant_int(&mut self, v: i64, pos: &Pos) -> usize {
        let d = self.new_local("", None);
        self.emit("const", &[d], &[], pos, json!({"const": {"kind": "int", "value": v.to_string()}}));
        d
    }

    // --- the body -------------------------------------------------------------

    fn run(&mut self) {
        let body = self.body;
        for _ in 0..body.blocks.len() {
            self.f.blocks.push(Block::default());
        }
        // which locals live in cells: those whose address is taken
        for bb in &body.blocks {
            for st in &bb.statements {
                if let StatementKind::Assign(_, rv) = &st.kind {
                    let place = match rv {
                        Rvalue::Ref(_, _, p) | Rvalue::AddressOf(_, p) | Rvalue::Reborrow(_, _, p) => Some(p),
                        _ => None,
                    };
                    if let Some(p) = place {
                        if !matches!(p.projection.first(), Some(ProjectionElem::Deref)) {
                            self.cells.insert(p.local);
                        }
                    }
                }
            }
        }
        let mut names: HashMap<usize, String> = HashMap::new();
        for d in &body.var_debug_info {
            if let VarDebugInfoContents::Place(p) = &d.value {
                if p.projection.is_empty() {
                    names.insert(p.local, d.name.clone());
                }
            }
        }
        for (i, d) in body.locals().iter().enumerate() {
            let name = names.get(&i).cloned().unwrap_or_default();
            let cell = self.cells.contains(&i);
            let l = if cell { self.pointer_local(&name) } else { self.new_local(&name, Some(&d.ty)) };
            self.var.push(l);
        }
        self.f.results = vec![type_id(&body.ret_local().ty)];
        // the prologue: parameters, and the cells
        self.cur = 0;
        // the parameters are where the function is: a source that is a parameter is
        // shown there
        let pos = self.here();
        let nargs = body.arg_locals().len();
        for i in 1..=nargs {
            let d = &body.locals()[i];
            let name = names.get(&i).cloned().unwrap_or_default();
            self.f.params.push(json!({"name": name, "type": type_id(&d.ty), "class": class_of(&d.ty), "display": type_id(&d.ty)}));
            if self.cells.contains(&i) {
                let v = self.new_local(&name, Some(&d.ty));
                self.emit("param", &[v], &[], &pos, json!({"index": i - 1}));
                self.emit("alloc", &[self.var[i]], &[], &pos, json!({"type": type_id(&d.ty), "class": class_of(&d.ty)}));
                self.emit("store", &[], &[self.var[i], v], &pos, json!({}));
            } else {
                self.emit("param", &[self.var[i]], &[], &pos, json!({"index": i - 1}));
            }
        }
        for i in 0..body.locals().len() {
            if self.cells.contains(&i) && !(1..=nargs).contains(&i) {
                let d = &body.locals()[i];
                self.emit("alloc", &[self.var[i]], &[], &pos, json!({"type": type_id(&d.ty), "class": class_of(&d.ty)}));
            }
        }
        for (i, bb) in body.blocks.iter().enumerate() {
            self.cur = i;
            for st in &bb.statements {
                self.span = st.source_info.span.clone();
                self.statement(&st.kind);
            }
            self.span = bb.terminator.source_info.span.clone();
            self.terminator(&bb.terminator.kind);
        }
    }

    fn statement(&mut self, k: &StatementKind) {
        if let StatementKind::Assign(place, rv) = k {
            let pos = self.here();
            let dst = if place.projection.is_empty() && !self.cells.contains(&place.local) { Some(self.var[place.local]) } else { None };
            let v = self.rvalue(rv, dst, &pos);
            if dst.is_none() || dst != Some(v) {
                self.write_place(place, v, &pos);
            }
        }
    }

    // --- places ---------------------------------------------------------------

    fn start(&self, p: &Place) -> Walk {
        let ty = self.body.locals()[p.local].ty;
        let cur = if self.cells.contains(&p.local) { Cur::Ptr(self.var[p.local]) } else { Cur::Val(self.var[p.local]) };
        Walk { cur, ty, variant: None }
    }

    fn field_name(&self, w: &Walk, i: usize) -> String {
        if let TyKind::RigidTy(RigidTy::Adt(def, _)) = w.ty.kind() {
            let found = match w.variant {
                Some(vi) => def.variant(vi),
                None => def.variants_iter().next(),
            };
            if let Some(v) = found {
                let fields = v.fields();
                let fname = fields.get(i).map(|f| f.name.clone()).unwrap_or_else(|| i.to_string());
                if matches!(def.kind(), AdtKind::Enum) {
                    return format!("{}.{}", v.name(), fname);
                }
                return fname;
            }
        }
        i.to_string()
    }

    fn pointee_value(&mut self, w: &Walk, pos: &Pos) -> usize {
        match w.cur {
            Cur::Ptr(c) => {
                let d = self.new_local("", Some(&w.ty));
                self.emit("load", &[d], &[c], pos, json!({}));
                d
            }
            Cur::Val(v) => v,
        }
    }

    /// Takes one step into the place. `last` is the last step of a read: a
    /// field read through an address is then one access.
    fn step(&mut self, w: &mut Walk, e: &ProjectionElem, last: bool, pos: &Pos) {
        match e {
            ProjectionElem::Deref => {
                let ptr = self.pointee_value(w, pos);
                w.ty = match w.ty.kind() {
                    TyKind::RigidTy(RigidTy::Ref(_, t, _)) | TyKind::RigidTy(RigidTy::RawPtr(t, _)) => t,
                    TyKind::RigidTy(RigidTy::Adt(_, args)) => match args.0.first() {
                        Some(GenericArgKind::Type(t)) => *t,
                        _ => w.ty,
                    },
                    _ => w.ty,
                };
                w.cur = Cur::Ptr(ptr);
                w.variant = None;
            }
            ProjectionElem::Field(_, fty) if is_smart_pointer(&w.ty) => {
                // The inside of a Box, a Unique, a NonNull: they are the
                // pointer itself, as far as the data is concerned.
                if let Cur::Ptr(_) = w.cur {
                    let v = self.pointee_value(w, pos);
                    w.cur = Cur::Val(v);
                }
                w.ty = *fty;
                w.variant = None;
            }
            ProjectionElem::Field(i, fty) => {
                let name = self.field_name(w, *i);
                match w.cur {
                    Cur::Ptr(p) if !last => {
                        let d = self.pointer_local("");
                        self.emit("addr_of", &[d], &[p], pos, json!({"path": [{"kind": "field", "field": name}]}));
                        w.cur = Cur::Ptr(d);
                    }
                    Cur::Ptr(p) => {
                        let d = self.new_local("", Some(fty));
                        self.emit("field_load", &[d], &[p], pos, json!({"field": name, "folded": true, "addr_pos": pos.to_json()}));
                        w.cur = Cur::Val(d);
                    }
                    Cur::Val(v) => {
                        let d = self.new_local("", Some(fty));
                        self.emit("field_load", &[d], &[v], pos, json!({"field": name}));
                        w.cur = Cur::Val(d);
                    }
                }
                w.ty = *fty;
                w.variant = None;
            }
            ProjectionElem::Index(l) => {
                let idx = self.read_local(*l, pos);
                self.index_step(w, idx, last, pos);
            }
            ProjectionElem::ConstantIndex { offset, .. } => {
                let idx = self.constant_int(*offset as i64, pos);
                self.index_step(w, idx, last, pos);
            }
            ProjectionElem::Subslice { .. } => {}
            ProjectionElem::Downcast(v) => {
                w.variant = Some(*v);
            }
            ProjectionElem::OpaqueCast(t) => {
                w.ty = *t;
            }
        }
    }

    fn index_step(&mut self, w: &mut Walk, idx: usize, last: bool, pos: &Pos) {
        let elem = match w.ty.kind() {
            TyKind::RigidTy(RigidTy::Array(t, _)) | TyKind::RigidTy(RigidTy::Slice(t)) => t,
            _ => w.ty,
        };
        match w.cur {
            Cur::Ptr(p) if !last => {
                let d = self.pointer_local("");
                self.emit("addr_of", &[d], &[p], pos, json!({"path": [{"kind": "index"}]}));
                w.cur = Cur::Ptr(d);
            }
            Cur::Ptr(p) => {
                let d = self.new_local("", Some(&elem));
                self.emit("index_load", &[d], &[p, idx], pos, json!({"folded": true, "addr_pos": pos.to_json()}));
                w.cur = Cur::Val(d);
            }
            Cur::Val(v) => {
                let d = self.new_local("", Some(&elem));
                self.emit("index_load", &[d], &[v, idx], pos, json!({}));
                w.cur = Cur::Val(d);
            }
        }
        w.ty = elem;
        w.variant = None;
    }

    fn read_local(&mut self, l: usize, pos: &Pos) -> usize {
        if self.cells.contains(&l) {
            let ty = self.body.locals()[l].ty;
            let d = self.new_local("", Some(&ty));
            self.emit("load", &[d], &[self.var[l]], pos, json!({}));
            d
        } else {
            self.var[l]
        }
    }

    fn read_place(&mut self, p: &Place, pos: &Pos) -> usize {
        let mut w = self.start(p);
        let n = p.projection.len();
        for (i, e) in p.projection.iter().enumerate() {
            let last = i + 1 == n && !matches!(e, ProjectionElem::Deref);
            self.step(&mut w, e, last, pos);
        }
        self.pointee_value(&w, pos)
    }

    /// A pointer to the place.
    fn ref_place(&mut self, p: &Place, pos: &Pos) -> usize {
        let mut w = self.start(p);
        for e in &p.projection {
            self.step(&mut w, e, false, pos);
        }
        match w.cur {
            Cur::Ptr(c) => c,
            Cur::Val(v) => v,
        }
    }

    fn write_place(&mut self, p: &Place, value: usize, pos: &Pos) {
        let n = p.projection.len();
        if n == 0 {
            if self.cells.contains(&p.local) {
                self.emit("store", &[], &[self.var[p.local], value], pos, json!({}));
            } else {
                self.emit("copy", &[self.var[p.local]], &[value], pos, json!({}));
            }
            return;
        }
        let mut w = self.start(p);
        for e in &p.projection[..n - 1] {
            self.step(&mut w, e, false, pos);
        }
        match &p.projection[n - 1] {
            ProjectionElem::Deref => {
                let ptr = self.pointee_value(&w, pos);
                self.emit("store", &[], &[ptr, value], pos, json!({}));
            }
            ProjectionElem::Field(i, _) => {
                let name = self.field_name(&w, *i);
                let (base, folded) = match w.cur {
                    Cur::Ptr(c) => (c, true),
                    Cur::Val(v) => (v, false),
                };
                let mut extra = json!({"field": name});
                if folded {
                    extra["folded"] = json!(true);
                    extra["addr_pos"] = pos.to_json();
                }
                self.emit("field_store", &[], &[base, value], pos, extra);
            }
            ProjectionElem::Index(l) => {
                let idx = self.read_local(*l, pos);
                self.index_store(&w, idx, value, pos);
            }
            ProjectionElem::ConstantIndex { offset, .. } => {
                let idx = self.constant_int(*offset as i64, pos);
                self.index_store(&w, idx, value, pos);
            }
            _ => {}
        }
    }

    fn index_store(&mut self, w: &Walk, idx: usize, value: usize, pos: &Pos) {
        let (base, folded) = match w.cur {
            Cur::Ptr(c) => (c, true),
            Cur::Val(v) => (v, false),
        };
        let mut extra = json!({});
        if folded {
            extra["folded"] = json!(true);
            extra["addr_pos"] = pos.to_json();
        }
        self.emit("index_store", &[], &[base, idx, value], pos, extra);
    }

    // --- operands and values ------------------------------------------------

    fn operand(&mut self, o: &Operand, pos: &Pos) -> usize {
        match o {
            Operand::Copy(p) | Operand::Move(p) => self.read_place(p, pos),
            Operand::Constant(c) => self.constant(&c.const_, pos),
            Operand::RuntimeChecks(_) => self.constant_int(0, pos),
        }
    }

    fn constant(&mut self, c: &MirConst, pos: &Pos) -> usize {
        let ty = c.ty();
        let d = self.new_local("", Some(&ty));
        // a function used as a value
        if let TyKind::RigidTy(RigidTy::FnDef(def, args)) = ty.kind() {
            let name = resolve(def, &args).map(|i| i.name()).unwrap_or_else(|| def.name());
            self.emit("make_closure", &[d], &[], pos, json!({"func": name}));
            return d;
        }
        let k = match c.kind() {
            ConstantKind::Allocated(alloc) => const_of(&ty, alloc),
            _ => json!({"kind": "nil"}),
        };
        self.emit("const", &[d], &[], pos, json!({"const": k}));
        d
    }

    fn rvalue(&mut self, rv: &Rvalue, dst: Option<usize>, pos: &Pos) -> usize {
        let ty = rv.ty(self.body.locals()).ok();
        let out = |me: &mut Self| match dst {
            Some(d) => d,
            None => me.new_local("", ty.as_ref()),
        };
        match rv {
            Rvalue::Use(o, _) => {
                let v = self.operand(o, pos);
                match dst {
                    Some(d) if d != v => {
                        self.emit("copy", &[d], &[v], pos, json!({}));
                        d
                    }
                    _ => v,
                }
            }
            Rvalue::CopyForDeref(p) => {
                let v = self.read_place(p, pos);
                match dst {
                    Some(d) if d != v => {
                        self.emit("copy", &[d], &[v], pos, json!({}));
                        d
                    }
                    _ => v,
                }
            }
            Rvalue::Ref(_, _, p) | Rvalue::AddressOf(_, p) | Rvalue::Reborrow(_, _, p) => {
                let v = self.ref_place(p, pos);
                match dst {
                    Some(d) if d != v => {
                        self.emit("copy", &[d], &[v], pos, json!({}));
                        d
                    }
                    _ => v,
                }
            }
            Rvalue::BinaryOp(op, a, b) | Rvalue::CheckedBinaryOp(op, a, b) => {
                let x = self.operand(a, pos);
                let y = self.operand(b, pos);
                let d = out(self);
                self.emit("binop", &[d], &[x, y], pos, json!({"operator": bin_op(op)}));
                d
            }
            Rvalue::UnaryOp(op, a) => {
                let x = self.operand(a, pos);
                let d = out(self);
                let o = match op {
                    UnOp::Not => "!",
                    UnOp::Neg => "-",
                    UnOp::PtrMetadata => "len",
                };
                self.emit("unop", &[d], &[x], pos, json!({"operator": o}));
                d
            }
            Rvalue::Cast(_, a, t) => {
                let x = self.operand(a, pos);
                let d = out(self);
                self.emit("convert", &[d], &[x], pos, json!({"operator": "convert", "type": type_id(t)}));
                d
            }
            Rvalue::Aggregate(kind, ops) => {
                let vals: Vec<usize> = ops.iter().map(|o| self.operand(o, pos)).collect();
                let d = out(self);
                let cls = ty.as_ref().map(class_of).unwrap_or("struct");
                self.emit("alloc", &[d], &[], pos, json!({"type": ty.as_ref().map(type_id).unwrap_or_default(), "class": cls}));
                for (i, v) in vals.iter().enumerate() {
                    let name = match kind {
                        AggregateKind::Adt(def, vi, _, _, _) => {
                            let w = Walk { cur: Cur::Val(d), ty: ty.unwrap(), variant: Some(*vi) };
                            let _ = def;
                            self.field_name(&w, i)
                        }
                        _ => i.to_string(),
                    };
                    if matches!(kind, AggregateKind::Array(_)) {
                        let idx = self.constant_int(i as i64, pos);
                        self.emit("index_store", &[], &[d, idx, *v], pos, json!({}));
                    } else {
                        self.emit("field_store", &[], &[d, *v], pos, json!({"field": name}));
                    }
                }
                // An `async fn` makes a coroutine that holds its arguments, and its
                // body runs when something polls it: a framework's executor, which
                // is not in the code that is read. The body is run here, as if the
                // coroutine were polled at once: whatever the executor does with it,
                // the body does with the arguments it was given.
                if let AggregateKind::Coroutine(def, _) = kind {
                    let body = def.name();
                    if self.cx.bodies.contains(&body) {
                        // the body takes the coroutine as `Pin<&mut Self>`: a box with the
                        // pointer in its field, as the body reads it
                        // (`&mut coroutine` is the cell the coroutine is in)
                        let cell = self.new_local("", None);
                        self.f.locals[cell].class = "pointer";
                        self.emit("alloc", &[cell], &[], pos, json!({"type": ty.as_ref().map(type_id).unwrap_or_default(), "class": "pointer"}));
                        self.emit("store", &[], &[cell, d], pos, json!({}));
                        let pin = self.new_local("", None);
                        self.emit("alloc", &[pin], &[], pos, json!({"type": "std::pin::Pin", "class": "struct"}));
                        self.emit("field_store", &[], &[pin, cell], pos, json!({"field": "pointer"}));
                        let resume = self.constant_int(0, pos);
                        let r = self.new_local("", None);
                        self.emit("call", &[r], &[pin, resume], pos, json!({"callee": {"kind": "static", "func": body, "symbol": clean_symbol(&body)}}));
                    }
                }
                d
            }
            Rvalue::Repeat(o, _) => {
                let v = self.operand(o, pos);
                let d = out(self);
                self.emit("alloc", &[d], &[], pos, json!({"type": ty.as_ref().map(type_id).unwrap_or_default(), "class": "container"}));
                let idx = self.constant_int(0, pos);
                self.emit("index_store", &[], &[d, idx, v], pos, json!({}));
                d
            }
            _ => {
                // lengths, discriminants, thread locals, ...: values that carry
                // no data of the program
                let d = out(self);
                self.emit("const", &[d], &[], pos, json!({"const": {"kind": "int", "value": "0"}}));
                d
            }
        }
    }

    // --- terminators --------------------------------------------------------

    fn terminator(&mut self, k: &TerminatorKind) {
        let pos = self.here();
        match k {
            TerminatorKind::Goto { target } => self.jump(*target),
            TerminatorKind::SwitchInt { discr, targets } => {
                let d = self.operand(discr, &pos);
                let branches: Vec<(u128, usize)> = targets.branches().collect();
                for (v, t) in branches {
                    let c = self.constant_int(v as i64, &pos);
                    let eq = self.new_local("", None);
                    self.f.locals[eq].ty = "bool".into();
                    self.f.locals[eq].class = "bool";
                    self.emit("binop", &[eq], &[d, c], &pos, json!({"operator": "=="}));
                    let next = self.new_block();
                    self.branch(eq, t, next, &pos);
                    self.cur = next;
                }
                self.jump(targets.otherwise());
            }
            TerminatorKind::Return => {
                let r = self.read_local(0, &pos);
                let b = &mut self.f.blocks[self.cur];
                b.term = Some(json!({"kind": "return", "values": [r]}));
            }
            TerminatorKind::Drop { target, .. } | TerminatorKind::Assert { target, .. } => self.jump(*target),
            TerminatorKind::Call { func, args, destination, target, .. } => {
                self.call(func, args, destination, *target, &pos);
            }
            _ => {
                // resume, abort, unreachable, inline assembly
                self.f.blocks[self.cur].term = Some(json!({"kind": "unreachable"}));
            }
        }
    }

    fn call(&mut self, func: &Operand, args: &[Operand], dest: &Place, target: Option<usize>, pos: &Pos) {
        let span = self.span.clone();
        let mut vals: Vec<usize> = args.iter().map(|a| self.operand(a, pos)).collect();
        let fty = func.ty(self.body.locals()).ok();
        let ret_ty = dest.ty(self.body.locals()).ok();
        let mut callee = json!({"kind": "value"});
        match fty.as_ref().map(|t| t.kind()) {
            Some(TyKind::RigidTy(RigidTy::FnDef(def, gargs))) => {
                let def_name = clean_symbol(&def.name());
                let resolved = resolve(def, &gargs);
                let virtual_call = resolved.as_ref().map(|i| matches!(i.kind, InstanceKind::Virtual { .. })).unwrap_or(false);
                let inst = resolved.map(|i| i.name());
                let is_trait = inst.as_ref().map(|n| n.starts_with('<')).unwrap_or(false) && !def_name.starts_with('<');
                // a closure called through Fn::call: its arguments come in a tuple
                if matches!(def_name.as_str(), "std::ops::Fn::call" | "std::ops::FnMut::call_mut" | "std::ops::FnOnce::call_once") && args.len() == 2 {
                    if let Ok(tt) = args[1].ty(self.body.locals()) {
                        if let TyKind::RigidTy(RigidTy::Tuple(tys)) = tt.kind() {
                            let tup = vals[1];
                            let mut spread = vec![vals[0]];
                            for (i, t) in tys.iter().enumerate() {
                                let d = self.new_local("", Some(t));
                                self.emit("field_load", &[d], &[tup], pos, json!({"field": i.to_string()}));
                                spread.push(d);
                            }
                            vals = spread;
                        }
                    }
                }
                let raw_def = def.name();
                match inst {
                    // a call through a trait object: whichever impl the object has
                    _ if virtual_call => {
                        let cands = self.candidates(&def_name);
                        callee = json!({"kind": "dynamic", "symbol": def_name, "candidates": cands});
                    }
                    Some(name) if self.cx.bodies.contains(&name) || self.cx.bodies.contains(&raw_def) => {
                        let func = if self.cx.bodies.contains(&name) { name.clone() } else { raw_def.clone() };
                        let sym = if is_trait { def_name.clone() } else { clean_symbol(&func) };
                        callee = json!({"kind": "static", "func": func, "symbol": sym});
                    }
                    Some(name) => {
                        // a function that has no body here: the core finds it
                        // missing and the models say what it does
                        callee = json!({"kind": "static", "func": name, "symbol": def_name});
                    }
                    None if vals.is_empty() => {
                        // a call that cannot be resolved here and has no receiver
                        // (T::default()): there is no value to dispatch on, so it
                        // is a call of a function the models have to know
                        callee = json!({"kind": "static", "func": def_name, "symbol": def_name});
                    }
                    None => {
                        // a call that cannot be resolved here: through a type parameter
                        let cands = self.candidates(&def_name);
                        callee = json!({"kind": "dynamic", "symbol": def_name, "candidates": cands});
                    }
                }
            }
            _ => {
                // a function pointer or a trait object: the callee is the value
                if let Some(f) = vals.first().copied() {
                    let _ = f;
                }
                let fv = self.operand(func, pos);
                vals.insert(0, fv);
            }
        }
        // the expression, and the call as written
        let expr = self.cx.expr_text(&span);
        self.cx.texts.add("expr", pos, expr.clone());
        self.cx.texts.call(pos, expr, vec![]);

        let plain = dest.projection.is_empty() && !self.cells.contains(&dest.local);
        let d = if plain { self.var[dest.local] } else { self.new_local("", ret_ty.as_ref()) };
        self.emit("call", &[d], &vals, pos, json!({"callee": callee}));
        if !plain {
            self.write_place(dest, d, pos);
        }
        match target {
            Some(t) => self.jump(t),
            None => self.f.blocks[self.cur].term = Some(json!({"kind": "unreachable"})),
        }
    }

    /// The implementations in this crate of a trait method named by its path.
    fn candidates(&self, def_name: &str) -> Vec<String> {
        let (trait_path, method) = match def_name.rsplit_once("::") {
            Some(x) => x,
            None => return vec![],
        };
        let mut out: Vec<String> = self
            .cx
            .bodies
            .iter()
            .filter(|n| n.starts_with('<') && n.ends_with(&format!(">::{}", method)) && n.contains(&format!(" as {}", trait_path)))
            .cloned()
            .collect();
        out.sort();
        out
    }
}

/// A constant of a type, from its bytes.
fn const_of(ty: &Ty, alloc: &Allocation) -> Value {
    let bytes: Vec<u8> = alloc.bytes.iter().filter_map(|b| *b).collect();
    match ty.kind() {
        TyKind::RigidTy(RigidTy::Bool) => json!({"kind": "bool", "value": (bytes.first().copied().unwrap_or(0) != 0).to_string()}),
        TyKind::RigidTy(RigidTy::Uint(_)) | TyKind::RigidTy(RigidTy::Char) => {
            let mut v: u128 = 0;
            for (i, b) in bytes.iter().take(16).enumerate() {
                v |= (*b as u128) << (8 * i);
            }
            json!({"kind": "int", "value": v.to_string()})
        }
        TyKind::RigidTy(RigidTy::Int(_)) => {
            let n = bytes.len().min(16);
            let mut v: u128 = 0;
            for (i, b) in bytes.iter().take(16).enumerate() {
                v |= (*b as u128) << (8 * i);
            }
            let shift = 128 - 8 * n as u32;
            let s = if n == 0 { 0 } else { ((v << shift) as i128) >> shift };
            json!({"kind": "int", "value": s.to_string()})
        }
        TyKind::RigidTy(RigidTy::Ref(_, t, _)) if matches!(t.kind(), TyKind::RigidTy(RigidTy::Str)) => {
            // &str: the bytes are in the allocation the pointer points to
            if let Some((_, prov)) = alloc.provenance.ptrs.first() {
                if let rustc_public::mir::alloc::GlobalAlloc::Memory(a) = rustc_public::mir::alloc::GlobalAlloc::from(prov.0) {
                    if let Ok(b) = a.raw_bytes() {
                        return json!({"kind": "string", "value": String::from_utf8_lossy(&b).to_string()});
                    }
                }
            }
            json!({"kind": "string", "value": ""})
        }
        _ => json!({"kind": "nil"}),
    }
}

/// `Instance::resolve`, which rustc aborts on (an internal error) for the calls
/// of a generic function that is analysed as written, when a generic argument
/// is a const parameter or a bound const (`[T; N]`, `Foo<N>`): rustc cannot
/// find it in the parameter environment of the function. Such a call is as
/// unresolved as one through a type parameter: `None`. (A panic is not caught: it leaves the
/// queries of rustc active, and the compiler then aborts at the end.)
pub fn resolve(def: FnDef, args: &GenericArgs) -> Option<Instance> {
    if args.0.iter().any(has_open_const) {
        return None;
    }
    Instance::resolve(def, args).ok()
}

/// Whether a generic argument holds a const that is not a value yet.
fn has_open_const(a: &GenericArgKind) -> bool {
    match a {
        GenericArgKind::Const(c) => !matches!(c.kind(), TyConstKind::Value(..) | TyConstKind::ZSTValue(..)),
        GenericArgKind::Type(t) => ty_has_open_const(t),
        _ => false,
    }
}

/// A type that has a const argument that is not a value: an array `[T; N]`, or a generic
/// type applied to one.
fn ty_has_open_const(t: &Ty) -> bool {
    match t.kind() {
        TyKind::RigidTy(RigidTy::Array(inner, len)) => !matches!(len.kind(), TyConstKind::Value(..) | TyConstKind::ZSTValue(..)) || ty_has_open_const(&inner),
        TyKind::RigidTy(RigidTy::Adt(_, args)) => args.0.iter().any(has_open_const),
        TyKind::RigidTy(RigidTy::Ref(_, inner, _)) | TyKind::RigidTy(RigidTy::Slice(inner)) => ty_has_open_const(&inner),
        _ => false,
    }
}
