// @cell(F/conv/exec-output-echo)
// @shape: a program prints a value that it is given (printenv, of an environment variable: `Command::arg` is itself a sink of rivus), and what the program wrote to its output is the sink: the value reaches the output of the process; the model of the output call would say it may, as an approximation, with an uncertainty
use std::process::Command;
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/exec-output-echo/vuln", vuln);
    m::case("F/conv/exec-output-echo/fixed", fixed);
    m::case("F/conv/exec-output-echo/decoy", decoy);
}

fn vuln() { // @vuln
    let out = Command::new("printenv").arg("V").env("V", m::src("conv-exec-output-echo-vuln")).output().unwrap(); // @source(conv-exec-output-echo-vuln) @ref(PARITY)
    m::sink_html(&String::from_utf8_lossy(&out.stdout)); // @sink(conv-exec-output-echo-vuln)
}

fn fixed() { // @fixed
    let out = Command::new("printenv").arg("V").env("V", m::clean_html(&m::src("conv-exec-output-echo-fixed"))).output().unwrap();
    m::sink_html(&String::from_utf8_lossy(&out.stdout)); // @sink(none)
}

fn decoy() { // @decoy
    let a = Command::new("printenv").arg("V").env("V", m::src("conv-exec-output-echo-decoy")).output().unwrap(); // @source(conv-exec-output-echo-decoy)
    let b = Command::new("printenv").arg("V").env("V", "constant").output().unwrap();
    m::sink_html(&String::from_utf8_lossy(&b.stdout)); // @sink(none) @nosink(conv-exec-output-echo-decoy)
    m::sink_sql(&String::from_utf8_lossy(&a.stdout)); // @sink(conv-exec-output-echo-decoy)
}
