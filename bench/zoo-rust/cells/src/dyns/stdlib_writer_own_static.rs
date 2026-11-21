// @cell(F/dyn/stdlib-writer-own-static)
// @shape: the project implements std::io::Write, and a value written to its type (not through a trait object) reaches a sink in its write
use std::io::{self, Write};
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/stdlib-writer-own-static/vuln", vuln);
    m::case("F/dyn/stdlib-writer-own-static/fixed", fixed);
    m::case("F/dyn/stdlib-writer-own-static/decoy", decoy);
}

struct ShellWriter;

impl Write for ShellWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_cmd_bytes(p); // @sink(dyn-stdlib-writer-own-static-vuln)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn vuln() { // @vuln
    let mut w = ShellWriter;
    w.write(&m::src_bytes("dyn-stdlib-writer-own-static-vuln")).unwrap(); // @source(dyn-stdlib-writer-own-static-vuln)
    w.flush().unwrap();
}

struct FixedWriter;

impl Write for FixedWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_cmd(&m::clean_cmd(&String::from_utf8_lossy(p))); // @sink(none)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn fixed() { // @fixed
    let mut w = FixedWriter;
    w.write(&m::src_bytes("dyn-stdlib-writer-own-static-fixed")).unwrap();
    w.flush().unwrap();
}

struct QuietWriter;

impl Write for QuietWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_cmd_bytes(p); // @sink(none) @nosink(dyn-stdlib-writer-own-static-decoy)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

struct LoudWriter;

impl Write for LoudWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_sql(&String::from_utf8_lossy(p)); // @sink(dyn-stdlib-writer-own-static-decoy)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn decoy() { // @decoy
    let mut q = QuietWriter;
    let mut l = LoudWriter;
    q.write(b"constant").unwrap();
    q.flush().unwrap();
    l.write(&m::src_bytes("dyn-stdlib-writer-own-static-decoy")).unwrap(); // @source(dyn-stdlib-writer-own-static-decoy)
    l.flush().unwrap();
}
