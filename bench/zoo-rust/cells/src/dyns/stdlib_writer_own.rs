// @cell(F/dyn/stdlib-writer-own)
// @shape: the project implements std::io::Write, and a value written through the trait object reaches a sink in its write
use std::io::{self, Write};
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/stdlib-writer-own/vuln", vuln);
    m::case("F/dyn/stdlib-writer-own/fixed", fixed);
    m::case("F/dyn/stdlib-writer-own/decoy", decoy);
}

struct ShellWriter;

impl Write for ShellWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_cmd_bytes(p); // @sink(dyn-stdlib-writer-own-vuln)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn vuln() { // @vuln
    let mut w = ShellWriter;
    let out: &mut dyn Write = &mut w;
    out.write_all(&m::src_bytes("dyn-stdlib-writer-own-vuln")).unwrap(); // @source(dyn-stdlib-writer-own-vuln)
    out.flush().unwrap();
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
    let out: &mut dyn Write = &mut w;
    out.write_all(&m::src_bytes("dyn-stdlib-writer-own-fixed")).unwrap();
    out.flush().unwrap();
}

struct QuietWriter;

impl Write for QuietWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_cmd_bytes(p); // @sink(none) @nosink(dyn-stdlib-writer-own-decoy)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

struct LoudWriter;

impl Write for LoudWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_sql(&String::from_utf8_lossy(p)); // @sink(dyn-stdlib-writer-own-decoy)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn decoy() { // @decoy
    let mut q = QuietWriter;
    let mut l = LoudWriter;
    let quiet: &mut dyn Write = &mut q;
    let loud: &mut dyn Write = &mut l;
    quiet.write_all(b"constant").unwrap();
    quiet.flush().unwrap();
    loud.write_all(&m::src_bytes("dyn-stdlib-writer-own-decoy")).unwrap(); // @source(dyn-stdlib-writer-own-decoy)
    loud.flush().unwrap();
}
