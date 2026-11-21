// @cell(F/dyn/fprint-writer-own)
// @shape: a value is written with write! to a writer of the project, and read back from the field that its write stores it in
use std::io::{self, Write};
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/fprint-writer-own/vuln", vuln);
    m::case("F/dyn/fprint-writer-own/fixed", fixed);
    m::case("F/dyn/fprint-writer-own/decoy", decoy);
}

struct SliceWriter {
    data: Vec<u8>,
}

impl Write for SliceWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        self.data.extend_from_slice(p);
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn vuln() { // @vuln
    let mut w = SliceWriter { data: Vec::new() };
    write!(w, "{}", m::src("dyn-fprint-writer-own-vuln")).unwrap(); // @source(dyn-fprint-writer-own-vuln) @ref(IO-WRITE-FMT,PARITY)
    w.flush().unwrap();
    m::sink_cmd(&String::from_utf8_lossy(&w.data)); // @sink(dyn-fprint-writer-own-vuln)
}

fn fixed() { // @fixed
    let mut w = SliceWriter { data: Vec::new() };
    write!(w, "{}", m::src("dyn-fprint-writer-own-fixed")).unwrap();
    w.flush().unwrap();
    m::sink_cmd(&m::clean_cmd(&String::from_utf8_lossy(&w.data))); // @sink(none)
}

struct NamedWriter {
    name: String,
    data: Vec<u8>,
}

impl Write for NamedWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        self.data.extend_from_slice(p);
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn decoy() { // @decoy
    let mut w = NamedWriter { name: String::from("constant"), data: Vec::new() };
    write!(w, "{}", m::src("dyn-fprint-writer-own-decoy")).unwrap(); // @source(dyn-fprint-writer-own-decoy) @ref(IO-WRITE-FMT,PARITY)
    w.flush().unwrap();
    m::sink_cmd(&w.name); // @sink(none) @nosink(dyn-fprint-writer-own-decoy)
    m::sink_sql(&String::from_utf8_lossy(&w.data)); // @sink(dyn-fprint-writer-own-decoy)
}
