// The recording implementation of zoo-mark (docs/DESIGN-zoo.md, 3.2 and 3.5). The analysis reads decl.rs, which has the
// same declarations with empty bodies; this one is built with the feature `exec` (by the harness, bench/zoo-rust/exec)
// and is no part of what rivus analyses.
//
// The marker and the judgement of "reached" are in bench/zoo/spec.json; the constants below are its copy, and
// exec/tests/spec.rs holds the two to be the same.
use crate::types::{CreateTask, Stmt, TaskMessage};
use std::fmt::Debug;
use std::panic::Location;
use std::sync::Mutex;

pub const CLASS_ANY: &str = "Any";
pub const CLASS_CMD: &str = "Cmd";
pub const CLASS_SQL: &str = "SQL";
pub const CLASS_HTML: &str = "HTML";
pub const CLASS_PATH: &str = "Path";

pub const ORDER: [&str; 4] = [CLASS_CMD, CLASS_SQL, CLASS_HTML, CLASS_PATH];

pub struct ClassSpec {
    pub segment: &'static str,
    pub danger: [&'static str; 2],
    pub set: &'static str,
}

pub fn spec(class: &str) -> ClassSpec {
    match class {
        CLASS_CMD => ClassSpec { segment: ".SH", danger: [";|$", "|$;"], set: ";|$\n" },
        CLASS_SQL => ClassSpec { segment: ".SQL", danger: ["'\"", "'`"], set: "'\"\\`" },
        CLASS_HTML => ClassSpec { segment: ".HTML", danger: ["<>&", "<&>"], set: "<>&" },
        _ => ClassSpec { segment: ".PATH", danger: ["//../..", "/../../"], set: "./" },
    }
}

/// The variant that this process runs (0 or 1): ZOO_VARIANT.
pub fn variant() -> usize {
    if std::env::var("ZOO_VARIANT").as_deref() == Ok("1") {
        1
    } else {
        0
    }
}

/// The danger of the class in the variant that runs.
pub fn danger(class: &str) -> &'static str {
    spec(class).danger[variant()]
}

/// One call of a sink.
pub struct Hit {
    pub case: String,
    pub file: String,
    pub line: u32,
    pub class: &'static str,
    pub seen: Vec<String>,
    pub reached: Vec<String>,
    pub srcs: Vec<String>,
    pub recv: String,
    pub masked: String,
    pub ambiguous: bool,
}

struct State {
    cases: Vec<(String, fn())>,
    current: String,
    case_srcs: Vec<(String, Vec<String>)>,
    all_tags: Vec<String>,
    hits: Vec<Hit>,
}

static STATE: Mutex<State> = Mutex::new(State {
    cases: Vec::new(),
    current: String::new(),
    case_srcs: Vec::new(),
    all_tags: Vec::new(),
    hits: Vec::new(),
});

fn state() -> std::sync::MutexGuard<'static, State> {
    STATE.lock().unwrap_or_else(|e| e.into_inner())
}

/// Registers a case.
pub fn case(id: &'static str, f: fn()) {
    state().cases.push((id.to_string(), f));
}

/// The ids of the registered cases, in the order of the registration.
pub fn case_ids() -> Vec<String> {
    state().cases.iter().map(|c| c.0.clone()).collect()
}

/// Runs the case of the id.
pub fn run_case(id: &str) {
    let f = {
        let mut st = state();
        let (cid, f) = st.cases.iter().find(|c| c.0 == id).cloned().expect("no such case");
        st.current = cid;
        f
    };
    f();
}

/// The number of hits so far.
pub fn hit_count() -> usize {
    state().hits.len()
}

fn marker(tag: &str) -> String {
    let mut st = state();
    if !st.all_tags.iter().any(|t| t == tag) {
        st.all_tags.push(tag.to_string());
    }
    let cur = st.current.clone();
    match st.case_srcs.iter_mut().find(|c| c.0 == cur) {
        Some(c) => {
            if !c.1.iter().any(|t| t == tag) {
                c.1.push(tag.to_string());
            }
        }
        None => st.case_srcs.push((cur, vec![tag.to_string()])),
    }
    let mut b = format!("Z{tag}");
    for c in ORDER {
        let s = spec(c);
        b.push_str(s.segment);
        b.push_str(s.danger[variant()]);
    }
    b
}

/// The source that is a parameter type.
pub struct In {
    pub value: String,
}

impl In {
    pub fn new(tag: &str) -> In {
        // (the text is made with the private marker, not with src: the analysis must not find a second source in
        // the recording implementation, which is what TestZooMarkIndependentRust holds)
        In { value: marker(tag) }
    }
}

pub fn src(tag: &str) -> String {
    marker(tag)
}

/// `src_b` does not record the tag: the cells that use it say that what it returns is not used.
pub fn src_b(_s: String) -> bool {
    false
}

pub fn src_bytes(tag: &str) -> Vec<u8> {
    marker(tag).into_bytes()
}

#[track_caller]
pub fn sink_cmd(s: &str) {
    record(CLASS_CMD, "", &[s.to_string()], Location::caller());
}
#[track_caller]
pub fn sink_cmd_own(s: String) {
    record(CLASS_CMD, "", &[s], Location::caller());
}
#[track_caller]
pub fn sink_cmd_key(s: &mut String) -> i32 {
    record(CLASS_CMD, "", &[s.clone()], Location::caller());
    0
}
#[track_caller]
pub fn sink_cmd_res(s: String) -> Result<(), String> {
    record(CLASS_CMD, "", &[s], Location::caller());
    Ok(())
}
#[track_caller]
pub fn sink_cmd_bytes(b: &[u8]) {
    record(CLASS_CMD, "", &[String::from_utf8_lossy(b).into_owned()], Location::caller());
}
#[track_caller]
pub fn sink_sql(s: &str) {
    record(CLASS_SQL, "", &[s.to_string()], Location::caller());
}
#[track_caller]
pub fn sink_html(s: &str) {
    record(CLASS_HTML, "", &[s.to_string()], Location::caller());
}
#[track_caller]
pub fn sink_path(base: &str, s: &str) {
    record(CLASS_PATH, base, &[s.to_string()], Location::caller());
}
/// Records the `Debug` text of the value (the marker has no `"` or `\` of its own that Debug would change in a way
/// that matters: the class `Any` judges by all the classes, and the tags are lower-cased letters, digits and `-`).
#[track_caller]
pub fn sink_any<T: Debug + ?Sized>(v: &T) {
    record(CLASS_ANY, "", &[format!("{v:?}")], Location::caller());
}

/// Runs the statement: the `sql` of it is the sink, the `params` are not.
#[track_caller]
pub fn sink_stmt(s: &Stmt) {
    record(CLASS_SQL, "", &[s.sql.clone()], Location::caller());
}
#[track_caller]
pub fn sink_stmt_any(v: &dyn std::any::Any) {
    let s = v.downcast_ref::<Stmt>().expect("a Stmt");
    record(CLASS_SQL, "", &[s.sql.clone()], Location::caller());
}
/// Sends the request of the task: the URL of the HTTP request is the sink.
#[track_caller]
pub fn sink_task(r: &CreateTask) {
    let mut u = String::new();
    if let Some(t) = &r.task {
        if let Some(TaskMessage::HttpRequest(h)) = &t.message_type {
            u = h.url.clone();
        }
    }
    record(CLASS_CMD, "", &[u], Location::caller());
}

fn clean(class: &str, s: &str) -> String {
    let set = spec(class).set;
    s.chars().map(|c| if set.contains(c) { '_' } else { c }).collect()
}

pub fn clean_cmd(s: &str) -> String {
    clean(CLASS_CMD, s)
}
pub fn clean_sql(s: &str) -> String {
    clean(CLASS_SQL, s)
}
pub fn clean_html(s: &str) -> String {
    clean(CLASS_HTML, s)
}
pub fn clean_path(s: &str) -> String {
    clean(CLASS_PATH, s)
}
// the kinds without a judgement of their own: they do what clean_cmd does when they run (see decl.rs)
pub fn clean_url(s: &str) -> String {
    clean(CLASS_CMD, s)
}
pub fn clean_resource(s: &str) -> String {
    clean(CLASS_CMD, s)
}

/// The place of the call of the sink, as a path from the root of the zoo: `cells/src/...`. A sink that is named as a
/// value and given to a function of the library is called by the library: the place is then the first frame of the
/// stack that is in the cells (the call that gave it).
fn place(loc: &Location) -> (String, u32) {
    let f = loc.file().replace('\\', "/");
    match f.rfind("cells/src/") {
        Some(i) => (f[i..].to_string(), loc.line()),
        None => in_cells().unwrap_or((f, loc.line())),
    }
}

fn in_cells() -> Option<(String, u32)> {
    let bt = std::backtrace::Backtrace::force_capture().to_string();
    for l in bt.lines() {
        let Some(rest) = l.trim().strip_prefix("at ") else { continue };
        let rest = rest.replace('\\', "/");
        let Some(i) = rest.rfind("cells/src/") else { continue };
        let mut parts = rest[i..].split(':');
        let file = parts.next()?.to_string();
        let line = parts.next()?.parse().ok()?;
        return Some((file, line));
    }
    None
}

fn record(class: &'static str, base: &str, strs: &[String], loc: &Location) {
    let (file, line) = place(loc);
    let text = strs.join("\0");
    let lower = text.to_lowercase();
    let mut st = state();
    let mut seen: Vec<String> =
        st.all_tags.iter().filter(|t| lower.contains(&format!("z{}.", t.to_lowercase()))).cloned().collect();
    seen.sort();
    let mut reached = false;
    for s in strs {
        if class == CLASS_ANY {
            for c in ORDER {
                reached = reached || judge(c, "/zoo/base", s);
            }
        } else {
            reached = reached || judge(class, base, s);
        }
    }
    // for the carry: the text without the dangers of the other classes (they differ between the variants and are
    // no part of what this sink is about); an Any sink has no carry
    let mut masked = text.clone();
    for c in ORDER {
        if c != class {
            let sp = spec(c);
            masked = masked.replace(sp.danger[0], "#").replace(sp.danger[1], "#");
        }
    }
    if class == CLASS_ANY {
        masked = String::new();
    }
    let cur = st.current.clone();
    let srcs: Vec<String> = st.case_srcs.iter().find(|c| c.0 == cur).map(|c| c.1.clone()).unwrap_or_default();
    let mut tags_reached = Vec::new();
    let mut ambiguous = false;
    if reached {
        if !seen.is_empty() {
            tags_reached = seen.clone();
        } else if srcs.len() == 1 {
            tags_reached = srcs.clone();
        } else {
            ambiguous = true;
        }
    }
    st.hits.push(Hit { case: cur, file, line, class, seen, reached: tags_reached, srcs, recv: text, masked, ambiguous });
}

/// Says whether the danger of the class, in the variant that runs, has reached a sink of the class in the text (the
/// judgement of bench/zoo/spec.json).
pub fn judge(class: &str, base: &str, s: &str) -> bool {
    let d = danger(class);
    match class {
        CLASS_CMD => judge_cmd(s, d),
        CLASS_SQL => {
            let b = s.as_bytes();
            let mut i = 0;
            while let Some(k) = s.get(i..).and_then(|r| r.find(d)) {
                let j = i + k;
                let prev = if j > 0 { b[j - 1] } else { 0 };
                let next = if j + 1 < b.len() { b[j + 1] } else { 0 };
                if prev != b'\\' && prev != b'\'' && next != b'\'' {
                    return true;
                }
                i = j + 1;
            }
            false
        }
        CLASS_HTML => s.contains(d),
        CLASS_PATH => {
            if !s.contains("..") {
                return false;
            }
            let b = clean_lexical(base);
            let j = clean_lexical(&format!("{base}/{s}"));
            !(j == b || j.starts_with(&format!("{b}/")))
        }
        _ => false,
    }
}

/// The lexical clean of an absolute path (Go's filepath.Clean).
fn clean_lexical(p: &str) -> String {
    let mut out: Vec<&str> = Vec::new();
    for part in p.split('/') {
        if part.is_empty() || part == "." {
            continue;
        }
        if part == ".." {
            out.pop();
            continue;
        }
        out.push(part);
    }
    format!("/{}", out.join("/"))
}

fn judge_cmd(s: &str, d: &str) -> bool {
    let b = s.as_bytes();
    let db = d.as_bytes();
    let mut quote = 0u8; // 0, b'\'' or b'"'
    let mut i = 0;
    while i < b.len() {
        let c = b[i];
        if quote == 0 && b[i..].starts_with(db) {
            return true;
        } else if c == b'\\' && quote != b'\'' {
            i += 1;
        } else if quote == 0 && (c == b'\'' || c == b'"') {
            quote = c;
        } else if quote != 0 && c == quote {
            quote = 0;
        }
        i += 1;
    }
    false
}

fn json_str(s: &str, out: &mut String) {
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
}

fn json_list(l: &[String], out: &mut String) {
    out.push('[');
    for (i, s) in l.iter().enumerate() {
        if i > 0 {
            out.push(',');
        }
        json_str(s, out);
    }
    out.push(']');
}

/// The observation as JSON (the shape that every language writes, bench/zoo/spec.json "observed").
pub fn observation_json(reverse: bool) -> String {
    let ids = case_ids();
    let st = state();
    let mut o = String::new();
    o.push_str(&format!("{{\"variant\":{},\"reverse\":{},\"cases\":", variant(), reverse));
    json_list(&ids, &mut o);
    o.push_str(",\"hits\":[");
    for (i, h) in st.hits.iter().enumerate() {
        if i > 0 {
            o.push(',');
        }
        o.push_str("{\"case\":");
        json_str(&h.case, &mut o);
        o.push_str(",\"file\":");
        json_str(&h.file, &mut o);
        o.push_str(&format!(",\"line\":{},\"class\":", h.line));
        json_str(h.class, &mut o);
        o.push_str(",\"seen\":");
        json_list(&h.seen, &mut o);
        o.push_str(",\"reached\":");
        json_list(&h.reached, &mut o);
        o.push_str(",\"srcs\":");
        json_list(&h.srcs, &mut o);
        o.push_str(",\"recv\":");
        json_str(&h.recv, &mut o);
        o.push_str(",\"masked\":");
        json_str(&h.masked, &mut o);
        if h.ambiguous {
            o.push_str(",\"ambiguous\":true");
        }
        o.push('}');
    }
    o.push_str("]}");
    o
}
