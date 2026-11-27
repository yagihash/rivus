// @cell(F/dyn/stdlib-writer-boxed)
// @shape: the project implements std::io::Write, and a Box of it (a std type that the project's impl is inside) is passed as a &mut dyn Write; a value written through it reaches a sink in the project's write
use std::io::{self, Write};
use zoo_mark as m;

pub fn register() {
    m::case("F/dyn/stdlib-writer-boxed/vuln", vuln);
    m::case("F/dyn/stdlib-writer-boxed/fixed", fixed);
    m::case("F/dyn/stdlib-writer-boxed/decoy", decoy);
}

struct ShellWriter;

impl Write for ShellWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_cmd_bytes(p); // @sink(dyn-stdlib-writer-boxed-vuln)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn vuln() { // @vuln
    let mut b = Box::new(ShellWriter);
    let out: &mut dyn Write = &mut b;
    out.write_all(&m::src_bytes("dyn-stdlib-writer-boxed-vuln")).unwrap(); // @source(dyn-stdlib-writer-boxed-vuln) @miss(dynamic_dispatch,unmodeled_library_call) @ref(#242)
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
    let mut b = Box::new(FixedWriter);
    let out: &mut dyn Write = &mut b;
    out.write_all(&m::src_bytes("dyn-stdlib-writer-boxed-fixed")).unwrap();
    out.flush().unwrap();
}

struct QuietWriter;

impl Write for QuietWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_cmd_bytes(p); // @sink(none) @nosink(dyn-stdlib-writer-boxed-decoy)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

struct LoudWriter;

impl Write for LoudWriter {
    fn write(&mut self, p: &[u8]) -> io::Result<usize> {
        m::sink_sql(&String::from_utf8_lossy(p)); // @sink(dyn-stdlib-writer-boxed-decoy)
        Ok(p.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn decoy() { // @decoy
    let mut q = Box::new(QuietWriter);
    let mut l = Box::new(LoudWriter);
    let quiet: &mut dyn Write = &mut q;
    let loud: &mut dyn Write = &mut l;
    quiet.write_all(b"constant").unwrap();
    quiet.flush().unwrap();
    loud.write_all(&m::src_bytes("dyn-stdlib-writer-boxed-decoy")).unwrap(); // @source(dyn-stdlib-writer-boxed-decoy) @miss(dynamic_dispatch,unmodeled_library_call) @ref(#242)
    loud.flush().unwrap();
}
