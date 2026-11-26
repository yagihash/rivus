// @cell(F/dart/dio-download-savepath)
// @shape: a value goes into the path of the file that the body of a response is written to (OpenOptions::open for writing after Client::get): the path traversal sink of the built-in rows. The file is opened for writing and not created, so the run makes none; the same value is also given to m::sink_path on the line, as the run sees the sink there
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/dio-download-savepath/vuln", vuln);
    m::case("F/dart/dio-download-savepath/fixed", fixed);
    m::case("F/dart/dio-download-savepath/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("dart-dio-download-savepath-vuln"); // @source(dart-dio-download-savepath-vuln)
    let c = reqwest::blocking::Client::new();
    let _ = c.get("http://example.invalid/file").send();
    { m::sink_path("/srv/base", &v); let _ = std::fs::OpenOptions::new().write(true).open(&v); } // @sink(dart-dio-download-savepath-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("dart-dio-download-savepath-fixed")))));
    let c = reqwest::blocking::Client::new();
    let _ = c.get("http://example.invalid/file").send();
    { m::sink_path("/srv/base", &v); let _ = std::fs::OpenOptions::new().write(true).open(&v); } // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dart-dio-download-savepath-decoy"); // @source(dart-dio-download-savepath-decoy)
    let c = reqwest::blocking::Client::new();
    let _ = c.post("http://example.invalid/file").body(&v).send();
    { m::sink_path("/srv/base", "out.bin"); let _ = std::fs::OpenOptions::new().write(true).open("out.bin"); } // @sink(none) @nosink(dart-dio-download-savepath-decoy)
    m::sink_sql(&v); // @sink(dart-dio-download-savepath-decoy)
}
