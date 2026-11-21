// Holds the constants of zoo-mark to bench/zoo/spec.json and runs its probes (in the variant of ZOO_VARIANT).
// std only: the file is read with a small JSON reader of this file.
use zoo_mark as m;

#[derive(Debug, Clone)]
enum J {
    Null,
    Bool(bool),
    Num(f64),
    Str(String),
    Arr(Vec<J>),
    Obj(Vec<(String, J)>),
}

impl J {
    fn get(&self, k: &str) -> &J {
        match self {
            J::Obj(o) => &o.iter().find(|e| e.0 == k).unwrap_or_else(|| panic!("no key {k}")).1,
            _ => panic!("not an object"),
        }
    }
    fn has(&self, k: &str) -> bool {
        matches!(self, J::Obj(o) if o.iter().any(|e| e.0 == k))
    }
    fn str(&self) -> &str {
        match self {
            J::Str(s) => s,
            _ => panic!("not a string: {self:?}"),
        }
    }
    fn arr(&self) -> &[J] {
        match self {
            J::Arr(a) => a,
            _ => panic!("not an array"),
        }
    }
    fn bool(&self) -> bool {
        match self {
            J::Bool(b) => *b,
            _ => panic!("not a bool"),
        }
    }
}

struct P<'a> {
    b: &'a [u8],
    i: usize,
}

impl P<'_> {
    fn ws(&mut self) {
        while self.i < self.b.len() && (self.b[self.i] as char).is_whitespace() {
            self.i += 1;
        }
    }
    fn value(&mut self) -> J {
        self.ws();
        match self.b[self.i] {
            b'{' => {
                self.i += 1;
                let mut o = Vec::new();
                loop {
                    self.ws();
                    if self.b[self.i] == b'}' {
                        self.i += 1;
                        return J::Obj(o);
                    }
                    if self.b[self.i] == b',' {
                        self.i += 1;
                        continue;
                    }
                    let k = self.string();
                    self.ws();
                    assert_eq!(self.b[self.i], b':');
                    self.i += 1;
                    o.push((k, self.value()));
                }
            }
            b'[' => {
                self.i += 1;
                let mut a = Vec::new();
                loop {
                    self.ws();
                    if self.b[self.i] == b']' {
                        self.i += 1;
                        return J::Arr(a);
                    }
                    if self.b[self.i] == b',' {
                        self.i += 1;
                        continue;
                    }
                    a.push(self.value());
                }
            }
            b'"' => J::Str(self.string()),
            b't' => {
                self.i += 4;
                J::Bool(true)
            }
            b'f' => {
                self.i += 5;
                J::Bool(false)
            }
            b'n' => {
                self.i += 4;
                J::Null
            }
            _ => {
                let st = self.i;
                while self.i < self.b.len() && matches!(self.b[self.i], b'-' | b'+' | b'.' | b'e' | b'E' | b'0'..=b'9') {
                    self.i += 1;
                }
                J::Num(std::str::from_utf8(&self.b[st..self.i]).unwrap().parse().unwrap())
            }
        }
    }
    fn string(&mut self) -> String {
        assert_eq!(self.b[self.i], b'"');
        self.i += 1;
        let mut out: Vec<u8> = Vec::new();
        loop {
            let c = self.b[self.i];
            self.i += 1;
            match c {
                b'"' => return String::from_utf8(out).unwrap(),
                b'\\' => {
                    let e = self.b[self.i];
                    self.i += 1;
                    match e {
                        b'n' => out.push(b'\n'),
                        b't' => out.push(b'\t'),
                        b'r' => out.push(b'\r'),
                        b'u' => {
                            let h = std::str::from_utf8(&self.b[self.i..self.i + 4]).unwrap();
                            self.i += 4;
                            let ch = char::from_u32(u32::from_str_radix(h, 16).unwrap()).unwrap();
                            out.extend(ch.to_string().as_bytes());
                        }
                        e => out.push(e),
                    }
                }
                c => out.push(c),
            }
        }
    }
}

fn spec() -> J {
    let text = std::fs::read_to_string("../../zoo/spec.json").expect("bench/zoo/spec.json");
    P { b: text.as_bytes(), i: 0 }.value()
}

#[test]
fn the_constants_are_the_spec() {
    let sp = spec();
    assert_eq!(sp.get("prefix").str(), "Z");
    let order: Vec<&str> = sp.get("order").arr().iter().map(|c| c.str()).collect();
    assert_eq!(order, m::ORDER);
    for c in m::ORDER {
        let s = sp.get("classes").get(c);
        let mine = m::spec(c);
        assert_eq!(mine.segment, s.get("segment").str(), "{c}");
        let d = s.get("danger").arr();
        assert_eq!(mine.danger[0], d[0].str(), "{c}");
        assert_eq!(mine.danger[1], d[1].str(), "{c}");
        assert_eq!(mine.set, s.get("set").str(), "{c}");
        assert_eq!(d[0].str().len(), d[1].str().len(), "{c}: the dangers differ in length");
        for danger in d {
            for r in danger.str().chars() {
                assert!(mine.set.contains(r), "{c}: {r} of the danger is not in the set");
            }
        }
    }
}

#[test]
fn the_set_of_a_class_holds_no_character_of_the_danger_of_another_class() {
    for c in m::ORDER {
        for o in m::ORDER {
            if o == c {
                continue;
            }
            let theirs = m::spec(o).danger.join("");
            for r in m::spec(c).set.chars() {
                assert!(!theirs.contains(r), "the set of {c} has {r}, which is in the danger of {o}");
            }
        }
    }
}

#[test]
fn the_probes() {
    let sp = spec();
    for p in sp.get("probes").arr() {
        let class = p.get("class").str();
        let d = m::danger(class);
        let dq = format!("{}{}", &d[..1], d);
        let text = p.get("text").str().replace("{DQ}", &dq).replace("{D}", d);
        let base = if p.has("base") { p.get("base").str() } else { "" };
        let got = m::judge(class, base, &text);
        assert_eq!(got, p.get("reached").bool(), "probe {class} {text:?} (variant {})", m::variant());
    }
}

#[test]
fn a_sanitizer_gives_the_same_text_for_both_variants() {
    let clean: [(&str, fn(&str) -> String); 4] =
        [(m::CLASS_CMD, m::clean_cmd), (m::CLASS_SQL, m::clean_sql), (m::CLASS_HTML, m::clean_html), (m::CLASS_PATH, m::clean_path)];
    for (c, f) in clean {
        let d = m::spec(c).danger;
        assert_eq!(f(d[0]), f(d[1]), "{c}");
        assert!(f(d[0]).replace('_', "").is_empty(), "{c}: a danger is left");
    }
}
