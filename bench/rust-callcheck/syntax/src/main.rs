//! The syntax side of bench/rust-callcheck/callcheck.py: the calls that the source writes, one JSON object per line.
//!
//!   syntax_calls DIR        (the .rs files under DIR; paths are relative to DIR; nothing is built or run)
//!
//! A call is `f(..)` or `x.m(..)` (syn ExprCall, ExprMethodCall), also inside the arguments of a macro whose arguments read
//! as a list of expressions (`format!`, `vec![a, b]`, `write!`, `assert_eq!`, ...). Position (see callcheck.py): line and
//! column of the start of the call expression (1-based, in characters: the same as the span of the MIR of rustc), and the
//! name that it calls (the method, or the last segment of the path). The end of the call and the position of the "(" are
//! written as well (the check does not key on them: the IR of the extractor has only the start).
//!
//! A record has `excl` when the call is not counted, and why: ctor (`Some(x)`, `P(x)`: the name starts with an upper-case
//! letter, which a tuple struct or a variant has and a function does not; syn does not resolve names), cfg_test (under
//! `#[cfg(test)]` or `#[test]`), const_init (the initializer of a const or a static), unreachable (after a `return`,
//! `break`, `continue` or `panic!`-like macro of the same block, or in `if false`). The tool does not look at the body of
//! `macro_rules!` and at the macros whose arguments are not a list of expressions: they are `macro` records with
//! `args_parse: false` (macro_opaque), and the calls in them are not seen.
use proc_macro2::LineColumn;
use quote::ToTokens;
use std::path::Path;
use syn::visit::Visit;

struct V<'a> {
    file: &'a str,
    owner: Vec<(String, usize)>,
    macro_depth: usize,
    cfg_test: usize,
    const_init: usize,
    dead: usize,
    cfg_other: usize,
    locals: Vec<std::collections::HashSet<String>>,
}

fn lc(p: LineColumn) -> (usize, usize) {
    (p.line, p.column + 1)
}

fn first(e: &impl ToTokens) -> LineColumn {
    e.to_token_stream().into_iter().next().map(|t| t.span().start()).unwrap_or(LineColumn { line: 0, column: 0 })
}

/// 1: under #[cfg(test)] or #[test]; 2: under another #[cfg(..)] (the syntax does not evaluate it); 0: neither.
fn cfg_kind(attrs: &[syn::Attribute]) -> u8 {
    let mut k = 0;
    for a in attrs {
        if a.path().is_ident("test") {
            return 1;
        }
        if a.path().is_ident("cfg") {
            let s = a.meta.to_token_stream().to_string();
            if s.contains("test") && !s.contains("not") {
                return 1;
            }
            k = 2;
        }
    }
    k
}

fn is_test_attr(attrs: &[syn::Attribute]) -> bool {
    cfg_kind(attrs) == 1
}

fn is_false(e: &syn::Expr) -> bool {
    matches!(e, syn::Expr::Lit(l) if matches!(&l.lit, syn::Lit::Bool(b) if !b.value))
}

fn diverges(s: &syn::Stmt) -> bool {
    match s {
        syn::Stmt::Expr(e, _) => match e {
            syn::Expr::Return(_) | syn::Expr::Break(_) | syn::Expr::Continue(_) => true,
            syn::Expr::Macro(m) => is_diverging_macro(&m.mac),
            _ => false,
        },
        syn::Stmt::Macro(m) => is_diverging_macro(&m.mac),
        _ => false,
    }
}

fn is_diverging_macro(m: &syn::Macro) -> bool {
    m.path.segments.last().map(|s| matches!(s.ident.to_string().as_str(), "panic" | "unreachable" | "todo" | "unimplemented")).unwrap_or(false)
}

impl<'a> V<'a> {
    fn excl(&self, ctor: bool) -> &'static str {
        if self.cfg_test > 0 {
            "cfg_test"
        } else if self.cfg_other > 0 {
            "cfg"
        } else if self.const_init > 0 {
            "const_init"
        } else if self.dead > 0 {
            "unreachable"
        } else if ctor {
            "ctor"
        } else {
            ""
        }
    }
    fn rec(&self, kind: &str, start: LineColumn, open: Option<LineColumn>, end: LineColumn, extra: &str) {
        let (sl, sc) = lc(start);
        let (el, ec) = lc(end);
        let (ol, oc) = open.map(lc).unwrap_or((0, 0));
        let (on, ol2) = self.owner.last().cloned().unwrap_or_default();
        println!(
            "{{\"file\":\"{}\",\"kind\":\"{}\",\"line\":{},\"col\":{},\"open_line\":{},\"open_col\":{},\"end_line\":{},\"end_col\":{},\"in_macro\":{},\"owner\":\"{}\",\"owner_line\":{}{}}}",
            self.file, kind, sl, sc, ol, oc, el, ec, self.macro_depth > 0, on, ol2, extra
        );
    }
    fn mac(&mut self, m: &syn::Macro, def: bool) {
        let name = m.path.segments.last().map(|s| s.ident.to_string()).unwrap_or_default();
        let start = first(&m.path);
        let end = match &m.delimiter {
            syn::MacroDelimiter::Paren(p) => p.span.close().end(),
            syn::MacroDelimiter::Brace(p) => p.span.close().end(),
            syn::MacroDelimiter::Bracket(p) => p.span.close().end(),
        };
        let parsed = m.parse_body_with(syn::punctuated::Punctuated::<syn::Expr, syn::Token![,]>::parse_terminated);
        let ok = !def && parsed.is_ok();
        self.rec("macro", start, None, end, &format!(",\"name\":\"{}\",\"args_parse\":{},\"excl\":\"{}\"", name, ok, if def { "macro_rules" } else { "" }));
        if let (true, Ok(p)) = (ok, parsed) {
            self.macro_depth += 1;
            for e in p.iter() {
                self.visit_expr(e);
            }
            self.macro_depth -= 1;
        }
    }
    fn with_cfg<F: FnOnce(&mut Self)>(&mut self, attrs: &[syn::Attribute], f: F) {
        let k = cfg_kind(attrs);
        if k == 1 {
            self.cfg_test += 1;
        } else if k == 2 {
            self.cfg_other += 1;
        }
        f(self);
        if k == 1 {
            self.cfg_test -= 1;
        } else if k == 2 {
            self.cfg_other -= 1;
        }
    }
    fn in_fn<F: FnOnce(&mut Self)>(&mut self, name: String, line: usize, attrs: &[syn::Attribute], f: F) {
        self.owner.push((name, line));
        self.locals.push(Default::default());
        self.with_cfg(attrs, f);
        self.locals.pop();
        self.owner.pop();
    }
    fn is_value_callee(&self, f: &syn::Expr) -> bool {
        match f {
            syn::Expr::Path(p) => p.path.segments.len() == 1 && p.path.get_ident().map(|i| self.locals.iter().any(|l| l.contains(&i.to_string()))).unwrap_or(false),
            syn::Expr::Paren(p) => self.is_value_callee(&p.expr),
            _ => true,
        }
    }
}

impl<'a> V<'a> {
    fn emit_call(&mut self, c: &syn::ExprCall, start: LineColumn) {
        let (name, ctor) = match &*c.func {
            syn::Expr::Path(p) => {
                let n = p.path.segments.last().map(|s| s.ident.to_string()).unwrap_or_default();
                let up = n.chars().next().map(|ch| ch.is_uppercase()).unwrap_or(false);
                (n, up)
            }
            _ => (String::new(), false),
        };
        let value = self.is_value_callee(&c.func);
        self.cfg_wrap(|s| s.rec("call", start, Some(c.paren_token.span.open().start()), c.paren_token.span.close().end(), &format!(",\"name\":\"{}\",\"value_callee\":{},\"excl\":\"{}\"", name, value, s.excl(ctor))));
    }
    fn emit_method(&mut self, c: &syn::ExprMethodCall, start: LineColumn) {
        self.cfg_wrap(|s| s.rec("method", start, Some(c.paren_token.span.open().start()), c.paren_token.span.close().end(), &format!(",\"name\":\"{}\",\"value_callee\":false,\"excl\":\"{}\"", c.method, s.excl(false))));
    }
    fn cfg_wrap<F: FnOnce(&Self)>(&self, f: F) {
        f(self)
    }
}

impl<'a, 'ast> Visit<'ast> for V<'a> {
    fn visit_item_fn(&mut self, i: &'ast syn::ItemFn) {
        self.in_fn(i.sig.ident.to_string(), i.sig.ident.span().start().line, &i.attrs, |s| syn::visit::visit_item_fn(s, i));
    }
    fn visit_impl_item_fn(&mut self, i: &'ast syn::ImplItemFn) {
        self.in_fn(i.sig.ident.to_string(), i.sig.ident.span().start().line, &i.attrs, |s| syn::visit::visit_impl_item_fn(s, i));
    }
    fn visit_trait_item_fn(&mut self, i: &'ast syn::TraitItemFn) {
        self.in_fn(i.sig.ident.to_string(), i.sig.ident.span().start().line, &i.attrs, |s| syn::visit::visit_trait_item_fn(s, i));
    }
    fn visit_item_mod(&mut self, i: &'ast syn::ItemMod) {
        self.with_cfg(&i.attrs, |s| syn::visit::visit_item_mod(s, i));
    }
    fn visit_item_impl(&mut self, i: &'ast syn::ItemImpl) {
        self.with_cfg(&i.attrs, |s| syn::visit::visit_item_impl(s, i));
    }
    fn visit_pat_ident(&mut self, p: &'ast syn::PatIdent) {
        if let Some(l) = self.locals.last_mut() {
            l.insert(p.ident.to_string());
        }
        syn::visit::visit_pat_ident(self, p);
    }
    fn visit_stmt(&mut self, st: &'ast syn::Stmt) {
        let attrs: &[syn::Attribute] = match st {
            syn::Stmt::Local(l) => &l.attrs,
            syn::Stmt::Macro(m) => &m.attrs,
            syn::Stmt::Expr(syn::Expr::Block(b), _) => &b.attrs,
            syn::Stmt::Expr(syn::Expr::Unsafe(b), _) => &b.attrs,
            syn::Stmt::Expr(syn::Expr::If(b), _) => &b.attrs,
            syn::Stmt::Expr(syn::Expr::Call(b), _) => &b.attrs,
            syn::Stmt::Expr(syn::Expr::MethodCall(b), _) => &b.attrs,
            syn::Stmt::Expr(syn::Expr::Macro(b), _) => &b.attrs,
            _ => &[],
        };
        self.with_cfg(attrs, |s| syn::visit::visit_stmt(s, st));
    }
    fn visit_expr_paren(&mut self, p: &'ast syn::ExprParen) {
        // rustc gives the expression in parentheses the span of the parentheses: `(a.f())` starts at the "(" in MIR
        let mut inner = &*p.expr;
        while let syn::Expr::Paren(q) = inner {
            inner = &q.expr;
        }
        let start = first(p);
        match inner {
            syn::Expr::Call(c) => {
                self.emit_call(c, start);
                syn::visit::visit_expr_call(self, c);
            }
            syn::Expr::MethodCall(c) => {
                self.emit_method(c, start);
                syn::visit::visit_expr_method_call(self, c);
            }
            _ => syn::visit::visit_expr_paren(self, p),
        }
    }
    fn visit_item_const(&mut self, i: &'ast syn::ItemConst) {
        self.const_init += 1;
        syn::visit::visit_item_const(self, i);
        self.const_init -= 1;
    }
    fn visit_item_static(&mut self, i: &'ast syn::ItemStatic) {
        self.const_init += 1;
        syn::visit::visit_item_static(self, i);
        self.const_init -= 1;
    }
    fn visit_impl_item_const(&mut self, i: &'ast syn::ImplItemConst) {
        self.const_init += 1;
        syn::visit::visit_impl_item_const(self, i);
        self.const_init -= 1;
    }
    fn visit_trait_item_const(&mut self, i: &'ast syn::TraitItemConst) {
        self.const_init += 1;
        syn::visit::visit_trait_item_const(self, i);
        self.const_init -= 1;
    }
    fn visit_block(&mut self, b: &'ast syn::Block) {
        let mut dead = false;
        for s in &b.stmts {
            if dead {
                self.dead += 1;
            }
            self.visit_stmt(s);
            if dead {
                self.dead -= 1;
            }
            if diverges(s) {
                dead = true;
            }
        }
    }
    fn visit_expr_if(&mut self, e: &'ast syn::ExprIf) {
        self.visit_expr(&e.cond);
        let f = is_false(&e.cond);
        if f {
            self.dead += 1;
        }
        self.visit_block(&e.then_branch);
        if f {
            self.dead -= 1;
        }
        if let Some((_, el)) = &e.else_branch {
            self.visit_expr(el);
        }
    }
    fn visit_expr_call(&mut self, c: &'ast syn::ExprCall) {
        self.emit_call(c, first(c));
        syn::visit::visit_expr_call(self, c);
    }
    fn visit_expr_method_call(&mut self, c: &'ast syn::ExprMethodCall) {
        self.emit_method(c, first(c));
        syn::visit::visit_expr_method_call(self, c);
    }
    fn visit_expr_macro(&mut self, m: &'ast syn::ExprMacro) {
        self.mac(&m.mac, false);
    }
    fn visit_stmt_macro(&mut self, m: &'ast syn::StmtMacro) {
        self.mac(&m.mac, false);
    }
    fn visit_item_macro(&mut self, m: &'ast syn::ItemMacro) {
        self.mac(&m.mac, m.mac.path.is_ident("macro_rules"));
    }
}

fn skipped_dir(p: &Path) -> bool {
    let n = p.file_name().and_then(|n| n.to_str()).unwrap_or("");
    if n == "target" || n.starts_with('.') {
        return true;
    }
    // tests, examples and benchmarks of a crate (the directory that has a Cargo.toml) are not shipped
    matches!(n, "tests" | "examples" | "benches") && p.parent().map(|d| d.join("Cargo.toml").exists()).unwrap_or(false)
}

fn walk(dir: &Path, out: &mut Vec<std::path::PathBuf>) {
    if let Ok(rd) = std::fs::read_dir(dir) {
        for e in rd.flatten() {
            let p = e.path();
            if p.is_dir() {
                if !skipped_dir(&p) {
                    walk(&p, out);
                }
            } else if p.extension().map(|x| x == "rs").unwrap_or(false) {
                out.push(p);
            }
        }
    }
}

fn main() {
    let root = std::env::args().nth(1).expect("usage: syntax_calls DIR");
    let root = Path::new(&root);
    let mut files = vec![];
    walk(root, &mut files);
    files.sort();
    for f in files {
        let rel = f.strip_prefix(root).unwrap().to_string_lossy().to_string();
        let text = std::fs::read_to_string(&f).unwrap_or_default();
        match syn::parse_file(&text) {
            Ok(ast) => {
                let mut v = V { file: &rel, owner: vec![], macro_depth: 0, cfg_test: 0, const_init: 0, dead: 0, cfg_other: 0, locals: vec![] };
                v.visit_file(&ast);
            }
            Err(e) => {
                eprintln!("parse error {}: {}", rel, e);
                println!("{{\"file\":\"{}\",\"kind\":\"parse_error\",\"line\":{},\"col\":{}}}", rel, e.span().start().line, e.span().start().column + 1);
            }
        }
    }
}
