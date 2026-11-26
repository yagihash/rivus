// @cell(F/dart/dio-multipartfile-path)
// @shape: a value goes into the path of the file that is read to be a part of a multipart body (File::open, then reqwest::multipart::Form): the path traversal sink of the built-in rows. The file is opened for reading, and the same value is also given to m::sink_path on the line, as the run sees the sink there
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/dio-multipartfile-path/vuln", vuln);
    m::case("F/dart/dio-multipartfile-path/fixed", fixed);
    m::case("F/dart/dio-multipartfile-path/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("dart-dio-multipartfile-path-vuln"); // @source(dart-dio-multipartfile-path-vuln)
    let _form = reqwest::multipart::Form::new().text("file", "upload.bin");
    { m::sink_path("/srv/base", &v); let _ = std::fs::File::open(&v); } // @sink(dart-dio-multipartfile-path-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("dart-dio-multipartfile-path-fixed")))));
    let _form = reqwest::multipart::Form::new().text("file", "upload.bin");
    { m::sink_path("/srv/base", &v); let _ = std::fs::File::open(&v); } // @sink(none)
}

fn decoy() { // @decoy
    let v = m::src("dart-dio-multipartfile-path-decoy"); // @source(dart-dio-multipartfile-path-decoy)
    let _form = reqwest::multipart::Form::new().text("file", &v);
    { m::sink_path("/srv/base", "out.bin"); let _ = std::fs::File::open("out.bin"); } // @sink(none) @nosink(dart-dio-multipartfile-path-decoy)
    m::sink_sql(&v); // @sink(dart-dio-multipartfile-path-decoy)
}
