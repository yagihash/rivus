// @cell(F/dyn/iface-box-passed)
// @shape: a Box of the project's Write is passed to a function as a &mut dyn Write, and the function calls a method of it directly (not write!); the project's write is reached through the Box
use std::io::{self, Write};
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/iface-box-passed/vuln", vuln);
    m::case("F/dyn/iface-box-passed/fixed", fixed);
    m::case("F/dyn/iface-box-passed/decoy", decoy);
}

fn put(w: &mut dyn Write, p: &[u8]) {
    w.write_all(p).unwrap();
    w.flush().unwrap();
}

struct ShellWriter;

impl Write for ShellWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_cmd_bytes(p); // @sink(dyn-iface-box-passed-vuln)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn vuln() { // @vuln
    let mut b = Box::new(ShellWriter);
    put(&mut b, &m::src_bytes("dyn-iface-box-passed-vuln")); // @source(dyn-iface-box-passed-vuln)
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
    let mut b = Box::new(FixedWriter);
    put(&mut b, &m::src_bytes("dyn-iface-box-passed-fixed"));
}

struct QuietWriter;

impl Write for QuietWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_cmd_bytes(p); // @sink(none) @nosink(dyn-iface-box-passed-decoy)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

struct LoudWriter;

impl Write for LoudWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_sql(&String::from_utf8_lossy(p)); // @sink(dyn-iface-box-passed-decoy)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn decoy() { // @decoy
    let mut q = Box::new(QuietWriter);
    let mut l = Box::new(LoudWriter);
    put(&mut q, b"constant");
    put(&mut l, &m::src_bytes("dyn-iface-box-passed-decoy")); // @source(dyn-iface-box-passed-decoy)
}
